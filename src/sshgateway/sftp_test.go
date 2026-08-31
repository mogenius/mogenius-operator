package sshgateway

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runListScript executes the real listing script against a local directory.
// The script is plain POSIX sh, so running it here exercises exactly what
// the container would run.
func runListScript(t *testing.T, dir string) ([]string, error) {
	t.Helper()

	cmd := exec.Command("sh", "-c", listScript, "sh", dir)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// A filename containing a newline must not be able to fabricate entries.
// The previous implementation asked stat to echo the name back and split the
// output on "\n", so a file called "note.txt\n41ed|4096|…|secrets" produced
// a second, invented entry that recursive clients would descend into.
func TestListScriptResistsNewlineInjection(t *testing.T) {
	dir := t.TempDir()

	hostile := "note.txt\n41ed|4096|1700000000|secrets"
	if err := os.WriteFile(filepath.Join(dir, hostile), []byte("x"), 0o600); err != nil {
		t.Skipf("filesystem rejects newlines in filenames: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "real.txt"), []byte("y"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	names, err := runListScript(t, dir)
	if err != nil {
		t.Fatalf("list script: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("got %d entries %q, want exactly the 2 real ones", len(names), names)
	}
	for _, name := range names {
		if name == "secrets" {
			t.Fatal("the crafted filename fabricated a phantom entry")
		}
	}
}

// A directory large enough to blow past the kernel's argument limit must
// still list. `stat -- *` fails outright there, and the previous script hid
// that behind `2>/dev/null; exit 0` — reporting an empty, successful listing.
func TestListScriptHandlesHugeDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("creates 30k files")
	}
	dir := t.TempDir()

	const count = 30000
	for i := range count {
		// Long names so the expansion comfortably exceeds ARG_MAX.
		name := fmt.Sprintf("entry-%05d-%s", i, strings.Repeat("x", 200))
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	names, err := runListScript(t, dir)
	if err != nil {
		t.Fatalf("list script failed on a large directory: %v", err)
	}
	if len(names) != count {
		t.Fatalf("got %d entries, want %d", len(names), count)
	}

	// Show the contrast: the shape the old implementation used cannot do it.
	old := exec.Command("sh", "-c", `cd -- "$1" || exit 1; stat -c '%f|%s|%Y|%n' -- * .* 2>/dev/null; exit 0`, "sh", dir)
	oldOut, oldErr := old.Output()
	if oldErr == nil && len(oldOut) == 0 {
		t.Log("confirmed: the previous script reports this directory as empty-and-successful")
	}
}

// Hidden entries belong in the listing; "." and ".." do not.
func TestListScriptIncludesHiddenAndSkipsDots(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"visible", ".hidden", ".config"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	names, err := runListScript(t, dir)
	if err != nil {
		t.Fatalf("list script: %v", err)
	}

	got := map[string]bool{}
	for _, name := range names {
		got[name] = true
	}
	for _, want := range []string{"visible", ".hidden", ".config"} {
		if !got[want] {
			t.Errorf("entry %q missing from listing %q", want, names)
		}
	}
	if got["."] || got[".."] {
		t.Errorf("listing contains . or ..: %q", names)
	}
}

// An empty directory lists as empty, without error.
func TestListScriptOnEmptyDirectory(t *testing.T) {
	names, err := runListScript(t, t.TempDir())
	if err != nil {
		t.Fatalf("list script on empty dir: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("got %q, want no entries", names)
	}
}

// A missing directory must fail rather than look empty.
func TestListScriptFailsOnMissingDirectory(t *testing.T) {
	if _, err := runListScript(t, filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("listing a missing directory reported success")
	}
}

// SSH_FXP_RENAME must fail when the destination exists — the previous `mv`
// silently overwrote it, so two clients using the standard "upload to .tmp,
// then rename" pattern clobbered each other.
func TestRenameScriptRefusesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("original"), 0o600); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	cmd := exec.Command("sh", "-c", renameScript, "sh", src, dst)
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != renameDestExistsCode {
		t.Fatalf("rename over an existing file: got %v, want exit %d", err, renameDestExistsCode)
	}

	content, readErr := os.ReadFile(dst)
	if readErr != nil {
		t.Fatalf("read dst: %v", readErr)
	}
	if string(content) != "original" {
		t.Fatalf("destination was overwritten: %q", content)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Fatalf("source should still exist: %v", statErr)
	}
}

// Renaming onto an existing *directory* must fail too: mv would move the
// file into it and report success, leaving the client to believe the path
// now names its file when it actually names a directory.
func TestRenameScriptRefusesExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dstdir")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cmd := exec.Command("sh", "-c", renameScript, "sh", src, dst)
	var exitErr *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exitErr) || exitErr.ExitCode() != renameDestExistsCode {
		t.Fatalf("rename onto a directory: got %v, want exit %d", err, renameDestExistsCode)
	}
	if _, err := os.Stat(filepath.Join(dst, "src")); err == nil {
		t.Fatal("the file was moved into the directory instead of being refused")
	}
}

// The normal case still has to work.
func TestRenameScriptMovesToFreeDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	cmd := exec.Command("sh", "-c", renameScript, "sh", src, dst)
	if err := cmd.Run(); err != nil {
		t.Fatalf("rename to a free destination failed: %v", err)
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(content) != "payload" {
		t.Fatalf("destination content = %q", content)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source still exists after a successful rename")
	}
}

// An aborted transfer must leave the remote file alone. The handlers carry
// no Kubernetes clients here on purpose: an implementation that still tried
// to upload would panic instead of returning.
func TestSpoolFileAbortedTransferDoesNotUpload(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "spool-*")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	spool := &spoolFile{
		handlers: &sftpHandlers{logger: slog.Default()},
		file:     file,
		path:     "/remote/file",
		writable: true,
	}

	if _, err := spool.WriteAt([]byte("partial"), 0); err != nil {
		t.Fatalf("WriteAt: %v", err)
	}
	spool.TransferError(errors.New("connection lost"))

	if err := spool.Close(); err != nil {
		t.Fatalf("Close after an aborted transfer: %v", err)
	}
	if _, err := os.Stat(file.Name()); !os.IsNotExist(err) {
		t.Error("the spool file was left behind")
	}
}

// A read handle never uploads, so closing it must not reach the cluster.
func TestSpoolFileReadHandleDoesNotUpload(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "spool-*")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	spool := &spoolFile{handlers: &sftpHandlers{logger: slog.Default()}, file: file, path: "/remote/file"}

	if err := spool.Close(); err != nil {
		t.Fatalf("Close on a read handle: %v", err)
	}
}

func TestSpoolFileRejectsOversizedWrite(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "spool-*")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer func() { _ = file.Close() }()
	spool := &spoolFile{handlers: &sftpHandlers{logger: slog.Default()}, file: file, writable: true}

	if _, err := spool.WriteAt([]byte("x"), maxTransferBytes); !errors.Is(err, errTransferTooLarge) {
		t.Fatalf("write past the cap returned %v, want errTransferTooLarge", err)
	}
}

func TestLimitedWriterStopsAtCap(t *testing.T) {
	var sink strings.Builder
	w := &limitedWriter{w: &sink, remaining: 4}

	if _, err := w.Write([]byte("abcd")); err != nil {
		t.Fatalf("write within the cap: %v", err)
	}
	if _, err := w.Write([]byte("e")); !errors.Is(err, errTransferTooLarge) {
		t.Fatalf("write past the cap returned %v, want errTransferTooLarge", err)
	}
	if !w.exceeded {
		t.Error("exceeded flag not set")
	}
	if sink.String() != "abcd" {
		t.Errorf("sink = %q, want %q", sink.String(), "abcd")
	}
}

func TestContainerPath(t *testing.T) {
	tests := []struct{ dir, name, want string }{
		{"/home/app", "file.txt", "/home/app/file.txt"},
		{"/home/app", "-rf", "/home/app/-rf"},
		{"", "/abs/path", "/abs/path"},
		{"/home/app", "", "/home/app"},
		// A relative result must not start with a dash, or the tool it is
		// handed to reads it as an option.
		{"", "-rf", "./-rf"},
		{"", "relative", "./relative"},
	}
	for _, tc := range tests {
		if got := containerPath(tc.dir, tc.name); got != tc.want {
			t.Errorf("containerPath(%q, %q) = %q, want %q", tc.dir, tc.name, got, tc.want)
		}
	}
}

func TestParseStatLine(t *testing.T) {
	info, err := parseStatLine("41ed|4096|1700000000")
	if err != nil {
		t.Fatalf("parseStatLine: %v", err)
	}
	if !info.IsDir() {
		t.Error("0x41ed should be a directory")
	}
	if info.Size() != 4096 {
		t.Errorf("size = %d, want 4096", info.Size())
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("perm = %o, want 755", info.Mode().Perm())
	}
	if info.ModTime().Unix() != 1700000000 {
		t.Errorf("mtime = %d", info.ModTime().Unix())
	}

	// A name is no longer part of the format, so a line carrying one is
	// malformed rather than silently accepted.
	if _, err := parseStatLine("41ed|4096|1700000000|injected"); err == nil {
		t.Error("a four-field line was accepted")
	}
	if _, err := parseStatLine("nothex|4096|1700000000"); err == nil {
		t.Error("a non-hex mode was accepted")
	}
}

func TestFileModeFromUnix(t *testing.T) {
	tests := []struct {
		raw  uint32
		want os.FileMode
	}{
		{0x81a4, 0o644},
		{0x41ed, os.ModeDir | 0o755},
		{0xa1ff, os.ModeSymlink | 0o777},
	}
	for _, tc := range tests {
		if got := fileModeFromUnix(tc.raw); got != tc.want {
			t.Errorf("fileModeFromUnix(%#x) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

// A direct-tcpip channel always lands on the target pod, so a destination
// naming some other host has to be refused: answering it with the pod's own
// port would connect the client somewhere it did not ask for.
func TestForwardableDestination(t *testing.T) {
	podLocal := []string{"", "localhost", "LOCALHOST", "127.0.0.1", "::1", "0.0.0.0"}
	for _, addr := range podLocal {
		if !forwardableDestination(addr) {
			t.Errorf("destination %q should be forwardable", addr)
		}
	}

	elsewhere := []string{"postgres.prod.svc", "10.0.0.5", "example.com", "kubernetes.default"}
	for _, addr := range elsewhere {
		if forwardableDestination(addr) {
			t.Errorf("destination %q should be refused; the forward cannot reach it", addr)
		}
	}
}
