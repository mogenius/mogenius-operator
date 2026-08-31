package sshgateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

const (
	// cmdTimeout bounds short metadata operations (stat, mkdir, rm, …).
	// Content transfers use the request context instead, so they end with
	// the client rather than on a fixed clock.
	cmdTimeout = 30 * time.Second

	// maxTransferBytes caps a single file transfer in either direction.
	// Transfers are staged in a temporary file on the operator, which lives
	// on the node's filesystem — without a ceiling, reading an endless file
	// such as /dev/zero fills the node until the kubelet starts evicting
	// pods that have nothing to do with this session.
	maxTransferBytes = 2 << 30 // 2 GiB

	// statBatchSize is how many entries are stat'ed per exec while listing.
	// Names average well under 256 bytes, so a batch stays far below the
	// kernel's argument limit — which is what makes a single `stat -- *`
	// fail outright on large directories.
	statBatchSize = 500

	// renameDestExistsCode is the exit status the rename script uses to
	// report that the destination is taken; SFTP requires a failure there
	// rather than a silent overwrite.
	renameDestExistsCode       = 17 // EEXIST
	renameDestExistsCodeString = "17"
)

// errTransferTooLarge is returned once a transfer hits maxTransferBytes.
var errTransferTooLarge = fmt.Errorf("transfer exceeds the %d byte limit of the ssh gateway", maxTransferBytes)

// serveSFTP runs an SFTP server on the session channel, translating every
// file operation into an exec call inside the container (cat/tee/stat/…).
// This is how scp and sftp work against pods that have no sshd: the protocol
// is terminated here and only standard POSIX tools run in the container.
// Requires sh + coreutils/busybox basics in the image — the same class of
// requirement as the shell for interactive sessions.
func serveSFTP(logger *slog.Logger, clients *execClients, channel ssh.Channel, namespace, podName, container string) int {
	handlers := &sftpHandlers{
		logger:    logger.With("scope", "sftp"),
		clients:   clients,
		namespace: namespace,
		pod:       podName,
		container: container,
	}

	server := sftp.NewRequestServer(channel, sftp.Handlers{
		FileGet:  handlers,
		FilePut:  handlers,
		FileCmd:  handlers,
		FileList: handlers,
	}, sftp.WithStartDirectory(handlers.homeDir()))

	if err := server.Serve(); err != nil && err != io.EOF {
		logger.Info("sftp server ended with error", "error", err)
		return 1
	}
	return 0
}

type sftpHandlers struct {
	logger    *slog.Logger
	clients   *execClients
	namespace string
	pod       string
	container string
}

// homeDir resolves the container's home so relative scp/sftp paths behave
// like they would against a real sshd. Falls back to "/".
func (h *sftpHandlers) homeDir() string {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	var out bytes.Buffer
	if err := h.execArgv(ctx, []string{"sh", "-c", "cd ~ 2>/dev/null && pwd"}, nil, &out); err != nil {
		return "/"
	}
	home := strings.TrimSpace(out.String())
	if !strings.HasPrefix(home, "/") {
		return "/"
	}
	return home
}

// execArgv runs argv (no shell unless argv is one) in the target container.
// stderr is captured and folded into the returned error; "not found" and
// "permission denied" map to fs errors so sftp reports proper status codes.
func (h *sftpHandlers) execArgv(ctx context.Context, argv []string, stdin io.Reader, stdout io.Writer) error {
	_, err := h.execArgvStatus(ctx, argv, stdin, stdout)
	return err
}

// execArgvStatus is execArgv plus the command's exit status, for callers
// that encode a specific outcome in it (see Rename).
func (h *sftpHandlers) execArgvStatus(ctx context.Context, argv []string, stdin io.Reader, stdout io.Writer) (int, error) {
	executor, err := newExecutor(h.clients, execParams{
		namespace: h.namespace,
		pod:       h.pod,
		container: h.container,
		command:   argv,
		stdin:     stdin != nil,
		stderr:    true,
	})
	if err != nil {
		return 0, err
	}

	if stdout == nil {
		stdout = io.Discard
	}
	var stderr bytes.Buffer
	opts := remotecommand.StreamOptions{Stdout: stdout, Stderr: &stderr}
	if stdin != nil {
		opts.Stdin = stdin
	}

	err = executor.StreamWithContext(ctx, opts)
	if err == nil {
		return 0, nil
	}

	exitCode := 0
	var codeErr utilexec.CodeExitError
	if errors.As(err, &codeErr) {
		exitCode = codeErr.Code
	}

	msg := strings.TrimSpace(stderr.String())
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "no such file"), strings.Contains(lower, "not a directory"):
		return exitCode, os.ErrNotExist
	case strings.Contains(lower, "permission denied"), strings.Contains(lower, "read-only file system"):
		return exitCode, os.ErrPermission
	case msg != "":
		return exitCode, fmt.Errorf("%s: %w", msg, err)
	}
	return exitCode, err
}

// containerPath joins dir and name into a path safe to pass as an argument:
// a leading "./" keeps a name that starts with a dash from being read as an
// option by the tool it is handed to.
func containerPath(dir, name string) string {
	joined := path.Join(dir, name)
	if !strings.HasPrefix(joined, "/") {
		joined = "./" + joined
	}
	return joined
}

// --- transfer staging ---------------------------------------------------

// spoolFile stages a transfer in a temporary file on the operator. SFTP is a
// random-access protocol while the container is reachable only as a stream,
// so reads are pulled down once and writes are collected here and pushed on
// close.
type spoolFile struct {
	handlers *sftpHandlers
	file     *os.File
	path     string

	// writable transfers upload on close; read-only handles do not.
	writable bool

	mu       sync.Mutex
	aborted  bool
	uploaded bool
}

func (h *sftpHandlers) newSpool(prefix string) (*os.File, error) {
	return os.CreateTemp("", "sshgw-"+prefix+"-*")
}

// download streams the remote file into w, refusing to stage more than
// maxTransferBytes. A missing file is reported as os.ErrNotExist.
func (h *sftpHandlers) download(ctx context.Context, remotePath string, w io.Writer) error {
	limited := &limitedWriter{w: w, remaining: maxTransferBytes}
	err := h.execArgv(ctx, []string{"cat", containerPath("", remotePath)}, nil, limited)
	if limited.exceeded {
		return errTransferTooLarge
	}
	return err
}

// openSpool stages a writable handle for req, honouring the open flags.
func (h *sftpHandlers) openSpool(req *sftp.Request) (*spoolFile, error) {
	flags := req.Pflags()

	if flags.Excl {
		if _, err := h.stat(req.Context(), req.Filepath); err == nil {
			return nil, os.ErrExist
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	file, err := h.newSpool("write")
	if err != nil {
		return nil, err
	}

	// Without O_TRUNC the client may write at arbitrary offsets and expects
	// everything it does not touch to survive — an editor saving four bytes
	// in the middle of a file, or a resumed upload continuing at the end.
	// The upload replaces the whole file, so the current content has to be
	// in the spool first.
	if !flags.Trunc {
		if err := h.download(req.Context(), req.Filepath, file); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = file.Close()
			_ = os.Remove(file.Name())
			return nil, err
		}
	}

	return &spoolFile{handlers: h, file: file, path: req.Filepath, writable: true}, nil
}

func (s *spoolFile) ReadAt(p []byte, off int64) (int, error) { return s.file.ReadAt(p, off) }

func (s *spoolFile) WriteAt(p []byte, off int64) (int, error) {
	if off+int64(len(p)) > maxTransferBytes {
		return 0, errTransferTooLarge
	}
	return s.file.WriteAt(p, off)
}

// TransferError marks the transfer as failed. Without it a connection that
// drops mid-upload would still push the partial spool over the destination
// on close — turning an interrupted transfer into data loss.
func (s *spoolFile) TransferError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aborted = true
	s.handlers.logger.Info("sftp transfer aborted; leaving the remote file untouched", "path", s.path, "error", err)
}

// Close uploads the staged content unless the transfer was aborted, then
// removes the spool.
func (s *spoolFile) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.uploaded {
		return nil
	}
	s.uploaded = true

	defer func() {
		name := s.file.Name()
		_ = s.file.Close()
		_ = os.Remove(name)
	}()

	if !s.writable || s.aborted {
		return nil
	}

	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// tee writes stdin to the target path without needing a shell for
	// redirection; its stdout copy is discarded.
	return s.handlers.execArgv(ctx, []string{"tee", containerPath("", s.path)}, s.file, io.Discard)
}

// limitedWriter fails the copy once more than remaining bytes are written,
// so a file that never ends cannot fill the operator's disk.
type limitedWriter struct {
	w         io.Writer
	remaining int64
	exceeded  bool
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > l.remaining {
		l.exceeded = true
		return 0, errTransferTooLarge
	}
	l.remaining -= int64(len(p))
	return l.w.Write(p)
}

// --- read / write handlers ----------------------------------------------

func (h *sftpHandlers) Fileread(req *sftp.Request) (io.ReaderAt, error) {
	file, err := h.newSpool("read")
	if err != nil {
		return nil, err
	}

	// req.Context() is cancelled when the client goes away, so an abandoned
	// download does not keep streaming into the spool.
	if err := h.download(req.Context(), req.Filepath, file); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, err
	}
	return &spoolFile{handlers: h, file: file, path: req.Filepath}, nil
}

func (h *sftpHandlers) Filewrite(req *sftp.Request) (io.WriterAt, error) {
	return h.openSpool(req)
}

// OpenFile serves handles opened for reading and writing at once; without it
// pkg/sftp hands back a write-only handle and every read fails.
func (h *sftpHandlers) OpenFile(req *sftp.Request) (sftp.WriterAtReaderAt, error) {
	return h.openSpool(req)
}

// --- metadata commands --------------------------------------------------

func (h *sftpHandlers) Filecmd(req *sftp.Request) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	switch req.Method {
	case "Mkdir":
		return h.execArgv(ctx, []string{"mkdir", containerPath("", req.Filepath)}, nil, nil)
	case "Remove":
		return h.execArgv(ctx, []string{"rm", containerPath("", req.Filepath)}, nil, nil)
	case "Rmdir":
		return h.execArgv(ctx, []string{"rmdir", containerPath("", req.Filepath)}, nil, nil)
	case "Rename":
		return h.rename(ctx, req.Filepath, req.Target, false)
	case "PosixRename":
		// The POSIX variant is defined to replace the destination; that is
		// the whole reason clients ask for it by name.
		return h.rename(ctx, req.Filepath, req.Target, true)
	case "Symlink":
		return h.execArgv(ctx, []string{"ln", "-s", containerPath("", req.Filepath), containerPath("", req.Target)}, nil, nil)
	case "Link":
		return h.execArgv(ctx, []string{"ln", containerPath("", req.Filepath), containerPath("", req.Target)}, nil, nil)
	case "Setstat":
		// scp sets permissions (and times) after upload. Apply chmod when
		// requested; ignore the rest — best effort is enough here.
		if req.AttrFlags().Permissions {
			// Attributes() parses the raw attribute blob and returns nil when
			// it is truncated (pkg/sftp discards that error), so a malformed
			// packet would otherwise nil-panic and take the operator down.
			attrs := req.Attributes()
			if attrs == nil {
				return fmt.Errorf("sftp: malformed attributes in %s request", req.Method)
			}
			mode := fmt.Sprintf("%o", attrs.FileMode().Perm())
			return h.execArgv(ctx, []string{"chmod", mode, containerPath("", req.Filepath)}, nil, nil)
		}
		return nil
	default:
		return fmt.Errorf("sftp: unsupported command %q", req.Method)
	}
}

// rename moves from to to. SFTP requires the plain rename to fail when the
// destination exists, so the check and the move happen in one shell command
// rather than as two calls a competing client could interleave.
func (h *sftpHandlers) rename(ctx context.Context, from, to string, replace bool) error {
	if replace {
		return h.execArgv(ctx, []string{"mv", "-f", containerPath("", from), containerPath("", to)}, nil, nil)
	}

	code, err := h.execArgvStatus(ctx, []string{"sh", "-c", renameScript, "sh", from, to}, nil, nil)
	if code == renameDestExistsCode {
		return os.ErrExist
	}
	return err
}

// --- listing ------------------------------------------------------------

// statFormat deliberately omits the name (%n): the entry names are already
// known from the listing pass, and echoing them back would let a file whose
// name contains a newline fabricate extra entries in the output.
const statFormat = "%f|%s|%Y"

// listScript emits the directory's entries NUL-separated. Both properties
// matter: NUL is the one byte a filename cannot contain, and the loop is a
// shell builtin, so unlike `stat -- *` it does not hand the whole expansion
// to execve and fail on a large directory.
const listScript = `cd -- "$1" || exit 1
for entry in * .*; do
  case "$entry" in .|..) continue ;; esac
  [ -e "$entry" ] || [ -L "$entry" ] || continue
  printf '%s\0' "$entry"
done`

// renameScript refuses an existing destination and otherwise moves. The
// check and the move share one invocation so a competing client cannot slip
// in between them.
const renameScript = `if [ -e "$2" ] || [ -L "$2" ]; then exit ` + renameDestExistsCodeString + `; fi
exec mv -- "$1" "$2"`

func (h *sftpHandlers) Filelist(req *sftp.Request) (sftp.ListerAt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	switch req.Method {
	case "List":
		infos, err := h.listDir(ctx, req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerat(infos), nil

	case "Stat":
		infos, err := h.statEntries(ctx, req.Filepath, []string{""})
		if err != nil {
			return nil, err
		}
		if len(infos) != 1 {
			return nil, os.ErrNotExist
		}
		infos[0].name = path.Base(req.Filepath)
		return listerat{infos[0]}, nil

	case "Readlink":
		var out bytes.Buffer
		if err := h.execArgv(ctx, []string{"readlink", containerPath("", req.Filepath)}, nil, &out); err != nil {
			return nil, err
		}
		target := strings.TrimSpace(out.String())
		return listerat{&sftpFileInfo{name: target, mode: 0o777 | os.ModeSymlink}}, nil

	default:
		return nil, fmt.Errorf("sftp: unsupported list method %q", req.Method)
	}
}

// listDir reads a directory in two passes. The names come back NUL-separated
// from a shell loop — NUL cannot occur in a filename, and the loop is a shell
// builtin, so neither a crafted name nor a directory with tens of thousands
// of entries can distort the result. The attributes are then fetched in
// batches whose size we control, instead of one `stat -- *` that the kernel
// rejects outright once the argument list grows too long.
func (h *sftpHandlers) listDir(ctx context.Context, dir string) ([]os.FileInfo, error) {
	var out bytes.Buffer
	if err := h.execArgv(ctx, []string{"sh", "-c", listScript, "sh", dir}, nil, &out); err != nil {
		return nil, err
	}

	var names []string
	for _, name := range strings.Split(out.String(), "\x00") {
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, nil
	}

	infos := make([]os.FileInfo, 0, len(names))
	for start := 0; start < len(names); start += statBatchSize {
		end := min(start+statBatchSize, len(names))

		batch := names[start:end]
		stats, err := h.statEntries(ctx, dir, batch)
		if err != nil {
			return nil, err
		}
		for i := range stats {
			stats[i].name = batch[i]
			infos = append(infos, stats[i])
		}
	}
	return infos, nil
}

// statEntries stats dir/name for every name, in one exec. Results come back
// in argument order, which is how they are paired with their names — the
// output itself carries no name to parse. An empty name means dir itself.
//
// A short result means at least one entry could not be stat'ed (a file
// removed underneath us is the common case), so the batch is retried one
// entry at a time to keep the pairing honest rather than silently shifted.
func (h *sftpHandlers) statEntries(ctx context.Context, dir string, names []string) ([]*sftpFileInfo, error) {
	argv := make([]string, 0, len(names)+3)
	argv = append(argv, "stat", "-c", statFormat)
	for _, name := range names {
		argv = append(argv, containerPath(dir, name))
	}

	var out bytes.Buffer
	err := h.execArgv(ctx, argv, nil, &out)
	lines := nonEmptyLines(out.String())

	if err == nil && len(lines) == len(names) {
		infos := make([]*sftpFileInfo, 0, len(names))
		for _, line := range lines {
			info, parseErr := parseStatLine(line)
			if parseErr != nil {
				return nil, parseErr
			}
			infos = append(infos, info)
		}
		return infos, nil
	}

	if len(names) == 1 {
		if err != nil {
			return nil, err
		}
		return nil, os.ErrNotExist
	}

	infos := make([]*sftpFileInfo, 0, len(names))
	for _, name := range names {
		single, singleErr := h.statEntries(ctx, dir, []string{name})
		if singleErr != nil {
			// The entry vanished between listing and stat; report it with
			// what is still known rather than failing the whole listing.
			infos = append(infos, &sftpFileInfo{name: name})
			continue
		}
		infos = append(infos, single[0])
	}
	return infos, nil
}

// stat returns the attributes of a single path.
func (h *sftpHandlers) stat(ctx context.Context, remotePath string) (*sftpFileInfo, error) {
	infos, err := h.statEntries(ctx, remotePath, []string{""})
	if err != nil {
		return nil, err
	}
	return infos[0], nil
}

func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// parseStatLine parses "<hex mode>|<size>|<mtime epoch>". The name is not
// part of the format on purpose (see statFormat).
func parseStatLine(line string) (*sftpFileInfo, error) {
	parts := strings.Split(line, "|")
	if len(parts) != 3 {
		return nil, fmt.Errorf("unparsable stat line %q", line)
	}
	rawMode, err := strconv.ParseUint(parts[0], 16, 32)
	if err != nil {
		return nil, fmt.Errorf("bad mode in stat line %q: %w", line, err)
	}
	size, _ := strconv.ParseInt(parts[1], 10, 64)
	mtime, _ := strconv.ParseInt(parts[2], 10, 64)
	return &sftpFileInfo{
		size:  size,
		mode:  fileModeFromUnix(uint32(rawMode)),
		mtime: time.Unix(mtime, 0),
	}, nil
}

// fileModeFromUnix converts raw st_mode bits (as printed by `stat %f`) into
// an os.FileMode including the type bits.
func fileModeFromUnix(m uint32) os.FileMode {
	mode := os.FileMode(m & 0o777)
	switch m & 0xF000 {
	case 0x4000:
		mode |= os.ModeDir
	case 0xA000:
		mode |= os.ModeSymlink
	case 0x1000:
		mode |= os.ModeNamedPipe
	case 0x2000:
		mode |= os.ModeDevice | os.ModeCharDevice
	case 0x6000:
		mode |= os.ModeDevice
	case 0xC000:
		mode |= os.ModeSocket
	}
	return mode
}

type sftpFileInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	mtime time.Time
}

func (f *sftpFileInfo) Name() string       { return f.name }
func (f *sftpFileInfo) Size() int64        { return f.size }
func (f *sftpFileInfo) Mode() os.FileMode  { return f.mode }
func (f *sftpFileInfo) ModTime() time.Time { return f.mtime }
func (f *sftpFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f *sftpFileInfo) Sys() any           { return nil }

// listerat serves a fixed []os.FileInfo through the sftp.ListerAt interface.
type listerat []os.FileInfo

func (l listerat) ListAt(dst []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(dst, l[offset:])
	if n < len(dst) {
		return n, io.EOF
	}
	return n, nil
}
