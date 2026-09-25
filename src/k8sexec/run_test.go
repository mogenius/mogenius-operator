package k8sexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// runLocally executes the argv BuildShellCommand produced against the local
// `sh`. The wrapper is plain POSIX sh, so this exercises exactly what the
// container would run — only the exec transport is missing.
func runLocally(t *testing.T, argv []string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(argv[0], argv[1:]...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		exitCode = exitErr.ExitCode()
	default:
		t.Fatalf("run %q: %v", argv, err)
	}
	return out.String(), errOut.String(), exitCode
}

// The acceptance case from the ticket: stdout, stderr and the exit code all
// arrive separately and unchanged.
func TestShellCommandSeparatesStreamsAndExitCode(t *testing.T) {
	argv := BuildShellCommand("sh", "echo out; echo err >&2; exit 3", "", nil, 0)
	stdout, stderr, code := runLocally(t, argv)
	if stdout != "out\n" {
		t.Errorf("stdout = %q, want %q", stdout, "out\n")
	}
	if stderr != "err\n" {
		t.Errorf("stderr = %q, want %q", stderr, "err\n")
	}
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

// Pipes, && and quotes must behave as in a shell: the command line is
// evaluated, not passed as a single word.
func TestShellCommandKeepsShellSyntax(t *testing.T) {
	argv := BuildShellCommand("sh", `printf 'a b\nc\n' | wc -l | tr -d ' ' && echo "done: it's fine"`, "", nil, 0)
	stdout, _, code := runLocally(t, argv)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if stdout != "2\ndone: it's fine\n" {
		t.Errorf("stdout = %q", stdout)
	}
}

// cwd and env take effect, and neither is interpreted by the wrapper: a
// directory with spaces, a value with quotes, a $ and a newline all survive.
func TestShellCommandAppliesCwdAndEnv(t *testing.T) {
	dir := t.TempDir() + "/with space"
	if _, _, code := runLocally(t, []string{"sh", "-c", `mkdir -- "$1"`, "sh", dir}); code != 0 {
		t.Fatalf("mkdir failed with %d", code)
	}
	env := map[string]string{
		"GREETING": `it's "quoted" $HOME`,
		"MULTI":    "line1\nline2",
		"_under":   "x",
	}
	argv := BuildShellCommand("sh", `pwd; printf '%s|%s|%s\n' "$GREETING" "$MULTI" "$_under"`, dir, env, 0)
	stdout, stderr, code := runLocally(t, argv)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	lines := strings.SplitN(stdout, "\n", 2)
	if !strings.HasSuffix(lines[0], "/with space") {
		t.Errorf("pwd = %q, want suffix %q", lines[0], "/with space")
	}
	want := `it's "quoted" $HOME|line1` + "\n" + `line2|x` + "\n"
	if lines[1] != want {
		t.Errorf("env output = %q, want %q", lines[1], want)
	}
}

// A working directory that does not exist is a distinct outcome, not a
// command that ran somewhere else.
func TestShellCommandReportsMissingCwd(t *testing.T) {
	argv := BuildShellCommand("sh", "echo ran", t.TempDir()+"/missing", nil, 0)
	stdout, stderr, code := runLocally(t, argv)
	if code != ChdirFailedExitCode {
		t.Errorf("exit code = %d, want %d", code, ChdirFailedExitCode)
	}
	if stdout != "" {
		t.Errorf("command ran despite failed cd: %q", stdout)
	}
	if stderr == "" {
		t.Error("no shell message about the failed cd on stderr")
	}
}

// The in-container timeout ends a runaway command with the SIGKILL code,
// which TimedOut then recognises. Skipped where the host has no `timeout`
// (macOS without coreutils); the fallback path is the operator's deadline.
func TestShellCommandUsesContainerTimeout(t *testing.T) {
	if _, err := exec.LookPath("timeout"); err != nil {
		t.Skip("no `timeout` binary on this host")
	}
	timeout := time.Second
	argv := BuildShellCommand("sh", "sleep 30", "", nil, timeout)
	start := time.Now()
	_, _, code := runLocally(t, argv)
	elapsed := time.Since(start)
	if elapsed > 10*time.Second {
		t.Fatalf("command was not stopped by the timeout (took %s)", elapsed)
	}
	if !TimedOut(nil, code, elapsed, timeout) {
		t.Errorf("exit code %d after %s not recognised as timeout", code, elapsed)
	}
}

func TestBuildShellCommandIsDeterministic(t *testing.T) {
	env := map[string]string{"B": "2", "A": "1", "C": "3"}
	argv := BuildShellCommand("bash", "true", "/tmp", env, 1500*time.Millisecond)
	want := []string{"bash", "-c", runScript, "bash", "/tmp", "2", "true", "A=1", "B=2", "C=3"}
	if strings.Join(argv, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("argv = %q, want %q", argv, want)
	}
}

func TestValidateEnvRejectsNonIdentifiers(t *testing.T) {
	if err := ValidateEnv(map[string]string{"OK_1": "x", "_x": "y"}); err != nil {
		t.Errorf("valid names rejected: %v", err)
	}
	for _, key := range []string{"", "1ABC", "A-B", "A B", "A=B", "$(id)", "a;b"} {
		if err := ValidateEnv(map[string]string{key: "x"}); err == nil {
			t.Errorf("key %q was accepted", key)
		}
	}
}

func TestCappedBufferTruncates(t *testing.T) {
	buf := newCappedBuffer(5)
	for _, chunk := range []string{"ab", "cd", "efg", "h"} {
		n, err := buf.Write([]byte(chunk))
		if err != nil || n != len(chunk) {
			t.Fatalf("write %q: n=%d err=%v; a short write would abort the exec stream", chunk, n, err)
		}
	}
	if got := buf.String(); got != "abcde" {
		t.Errorf("kept %q, want %q", got, "abcde")
	}
	if !buf.truncated {
		t.Error("truncation not flagged")
	}

	exact := newCappedBuffer(3)
	_, _ = exact.Write([]byte("abc"))
	if exact.truncated {
		t.Error("a write that exactly fills the cap is not a truncation")
	}

	unlimited := newCappedBuffer(0)
	_, _ = unlimited.Write(bytes.Repeat([]byte("x"), 4096))
	if unlimited.truncated || len(unlimited.String()) != 4096 {
		t.Error("limit 0 must mean unlimited")
	}
}

func TestTimedOut(t *testing.T) {
	timeout := 2 * time.Second
	cases := []struct {
		name    string
		err     error
		code    int
		elapsed time.Duration
		want    bool
	}{
		{"operator deadline", context.DeadlineExceeded, 0, 3 * time.Second, true},
		{"killed after timeout", nil, KilledExitCode, 2100 * time.Millisecond, true},
		{"gnu timeout code", nil, TimeoutExitCode, 2100 * time.Millisecond, true},
		{"killed early is not a timeout", nil, KilledExitCode, time.Second, false},
		{"ordinary failure", nil, 1, 3 * time.Second, false},
		{"transport error", errors.New("connection refused"), 0, 3 * time.Second, false},
		{"unbounded run", nil, KilledExitCode, time.Hour, false},
	}
	for _, tc := range cases {
		limit := timeout
		if tc.name == "unbounded run" {
			limit = 0
		}
		if got := TimedOut(tc.err, tc.code, tc.elapsed, limit); got != tc.want {
			t.Errorf("%s: TimedOut = %v, want %v", tc.name, got, tc.want)
		}
	}
}
