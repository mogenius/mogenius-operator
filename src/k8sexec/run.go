package k8sexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"time"

	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

const (
	// DefaultRunTimeout applies when a request names no timeout. It matches
	// what the SDK's executeCommand assumes, so a ported client sees the same
	// behaviour.
	DefaultRunTimeout = 10 * time.Second

	// RunGrace is added to the operator-side deadline so a command that the
	// in-container `timeout` already killed can still deliver its exit code
	// before the stream itself is cut.
	RunGrace = 5 * time.Second

	// KilledExitCode is what a shell reports for a process ended by SIGKILL
	// (128 + 9); both GNU and busybox `timeout -s KILL` surface it.
	KilledExitCode = 137
	// TimeoutExitCode is GNU timeout's own code when it had to stop the
	// command and was not asked to preserve the command's status.
	TimeoutExitCode = 124
	// ChdirFailedExitCode is returned by the wrapper script when the requested
	// working directory cannot be entered; the shell's message is on stderr.
	// It is spelled out in runScript, which the tests hold to this value.
	ChdirFailedExitCode = 126
)

// envKeyPattern is what a POSIX shell accepts as a variable name. Anything
// else is rejected before it reaches the container, so `export "$kv"` in the
// wrapper cannot be turned into something other than an assignment.
var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// RunRequest is one non-interactive command in a container: no TTY, stdout
// and stderr kept apart. Run captures them, Stream hands them on as they
// come.
type RunRequest struct {
	Namespace string
	Pod       string
	Container string
	// Command is the argv handed to the exec API as-is; see BuildShellCommand
	// for the shell wrapper that turns a command line into one.
	Command []string
	// MaxOutputBytes caps each stream separately. Zero means unlimited.
	MaxOutputBytes int
}

// RunResult is the outcome of a finished command. Stdout and Stderr hold at
// most MaxOutputBytes each; Truncated says whether either was cut.
type RunResult struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Truncated bool
}

// Run executes req.Command in the container and waits for it to finish. A
// non-zero exit code is a result, not an error: the error return is reserved
// for the exec not happening at all (transport, RBAC, missing pod or binary)
// and for ctx ending first. On a context error the partial output collected
// so far is returned alongside it.
//
// Ending ctx closes the exec stream but does not by itself stop the process
// in the container — the kubelet has no signal to send without a TTY. Callers
// that need the process gone wrap the command with BuildShellCommand, whose
// script uses the container's `timeout` when it has one.
func Run(ctx context.Context, clients *Clients, req RunRequest) (RunResult, error) {
	stdout := newCappedBuffer(req.MaxOutputBytes)
	stderr := newCappedBuffer(req.MaxOutputBytes)
	exitCode, err := Stream(ctx, clients, req, nil, stdout, stderr)
	return RunResult{
		ExitCode:  exitCode,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		Truncated: stdout.truncated || stderr.truncated,
	}, err
}

// Stream executes req.Command in the container and writes stdout and stderr
// to the writers as the command produces them, so a caller can relay a long
// run live instead of after the fact. A non-nil stdin is connected to the
// command's standard input and the exec API closes the container's stdin
// when it ends — which is the signal the wrapper of BuildStreamShellCommand
// waits for to stop the command. The exit code and error contract are those
// of Run; a failing writer ends the exec.
func Stream(ctx context.Context, clients *Clients, req RunRequest, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if clients == nil || clients.Clientset == nil || clients.RestConfig == nil {
		return 0, errors.New("exec: no kubernetes clients")
	}
	if len(req.Command) == 0 {
		return 0, errors.New("exec: empty command")
	}

	request := clients.Clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(req.Pod).
		Namespace(req.Namespace).
		SubResource("exec").
		Param("container", req.Container).
		Param("stdout", "true").
		Param("stdin", strconv.FormatBool(stdin != nil)).
		Param("stderr", "true").
		Param("tty", "false")
	for _, arg := range req.Command {
		request = request.Param("command", arg)
	}
	executor, err := remotecommand.NewSPDYExecutor(clients.RestConfig, "POST", request.URL())
	if err != nil {
		return 0, fmt.Errorf("exec: create executor: %w", err)
	}

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: stdin, Stdout: stdout, Stderr: stderr})
	if err == nil {
		return 0, nil
	}
	var codeErr utilexec.CodeExitError
	if errors.As(err, &codeErr) {
		return codeErr.Code, nil
	}
	if errors.Is(err, io.EOF) {
		return 0, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return 0, ctxErr
	}
	return 0, err
}

// ValidateEnv rejects environment variable names a POSIX shell would not
// accept. Values are not constrained: they travel as a separate argv entry
// and are never interpreted by the wrapper.
func ValidateEnv(env map[string]string) error {
	for key := range env {
		if !envKeyPattern.MatchString(key) {
			return fmt.Errorf("invalid environment variable name %q", key)
		}
	}
	return nil
}

// runScript is the wrapper every command line goes through. It receives its
// inputs as positional arguments rather than being assembled from them, so a
// working directory, an environment value or the command itself can contain
// any character without becoming shell syntax of the wrapper:
//
//	$0  the shell (argv[0], re-invoked under `timeout`)
//	$1  working directory, empty to keep the container's
//	$2  timeout in whole seconds, 0 to run unbounded
//	$3  the command line, run through eval so pipes, && and quoting behave
//	    as they would in a shell
//	$4… KEY=VALUE pairs to export before the command runs
//
// When the image ships `timeout` (coreutils, busybox), the command is run
// under it with SIGKILL so a runaway process actually dies at the deadline;
// the operator's own deadline is only a backstop for images without it.
const runScript = `if [ -n "$1" ]; then cd -- "$1" || exit 126; fi
__mo_timeout=$2
__mo_cmd=$3
shift 3
for __mo_kv in "$@"; do export "$__mo_kv"; done
if [ "$__mo_timeout" -gt 0 ] 2>/dev/null && command -v timeout >/dev/null 2>&1; then
  exec timeout -s KILL "$__mo_timeout" "$0" -c "$__mo_cmd"
fi
eval "$__mo_cmd"`

// streamPrelude runs ahead of runScript when the command is streamed. The
// exec API has no signal to send a non-TTY process once its stream is gone,
// so the wrapper watches its own stdin instead: the operator keeps that pipe
// open for as long as the client is connected and closes it when the client
// goes away or the command has ended. The watchdog subshell then kills the
// wrapper's whole process tree, children first, by walking /proc — present
// in every Linux container, where `pkill` or `setsid` are not. Each process
// is stopped before its children are visited, so a shell cannot run on to
// its next command while its child dies under it; a stopped process still
// dies of KILL. The command itself gets /dev/null as stdin, so it neither
// competes with the watchdog for the pipe nor blocks on input nobody sends.
// The subshell writes to /dev/null so the exec's output streams close with
// the command, not with the watchdog. Without job control a shell gives a
// background list /dev/null as stdin, hence the explicit copy of fd 0 the
// subshell reads instead. `local` is not POSIX but every shell DetectShell
// can pick (bash, dash, ash) has it.
const streamPrelude = `__mo_kill_tree() {
  local __mo_root=$1 __mo_p __mo_stat
  kill -STOP "$__mo_root" 2>/dev/null
  for __mo_p in /proc/[0-9]*; do
    __mo_p=${__mo_p#/proc/}
    [ "$__mo_p" = "$__mo_self" ] && continue
    read -r __mo_stat < "/proc/$__mo_p/stat" 2>/dev/null || continue
    set -- ${__mo_stat##*) }
    [ "$2" = "$__mo_root" ] && __mo_kill_tree "$__mo_p"
  done
  kill -KILL "$__mo_root" 2>/dev/null
}
exec 3<&0
( exec 3<&-; read -r __mo_self _ < /proc/self/stat; cat >/dev/null 2>&1; __mo_kill_tree $$ ) <&3 >/dev/null 2>&1 &
exec 3<&- </dev/null
`

// BuildShellCommand turns a command line into the argv Run expects: the
// shell, the wrapper script and the script's positional inputs. env is
// emitted in sorted key order so identical requests produce identical argv.
// Callers validate env with ValidateEnv first.
func BuildShellCommand(shell, command, cwd string, env map[string]string, timeout time.Duration) []string {
	return buildShellCommand(runScript, shell, command, cwd, env, timeout)
}

// BuildStreamShellCommand is BuildShellCommand for a command run through
// Stream with a stdin pipe: the wrapper additionally stops the command when
// that pipe closes (see streamPrelude).
func BuildStreamShellCommand(shell, command, cwd string, env map[string]string, timeout time.Duration) []string {
	return buildShellCommand(streamPrelude+runScript, shell, command, cwd, env, timeout)
}

func buildShellCommand(script, shell, command, cwd string, env map[string]string, timeout time.Duration) []string {
	seconds := 0
	if timeout > 0 {
		seconds = int(math.Ceil(timeout.Seconds()))
	}
	argv := []string{shell, "-c", script, shell, cwd, strconv.Itoa(seconds), command}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		argv = append(argv, key+"="+env[key])
	}
	return argv
}

// TimedOut reports whether a finished Run was ended by its timeout: either
// the operator-side deadline passed, or the in-container `timeout` stopped
// the command — recognisable by its exit code once at least the timeout has
// elapsed, which keeps a command that was killed for another reason before
// then from being reported as slow.
func TimedOut(err error, exitCode int, elapsed, timeout time.Duration) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if err != nil || timeout <= 0 || elapsed < timeout {
		return false
	}
	return exitCode == KilledExitCode || exitCode == TimeoutExitCode
}

// cappedBuffer keeps the first limit bytes written to it and drops the rest
// while still reporting the full write as consumed — a short write would
// abort the exec stream, and the command should run to its end regardless
// of how much it prints.
type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.limit <= 0 {
		return c.buf.Write(p)
	}
	remaining := c.limit - c.buf.Len()
	if remaining <= 0 {
		if len(p) > 0 {
			c.truncated = true
		}
		return len(p), nil
	}
	if len(p) > remaining {
		c.truncated = true
		c.buf.Write(p[:remaining])
		return len(p), nil
	}
	return c.buf.Write(p)
}

func (c *cappedBuffer) String() string {
	return c.buf.String()
}
