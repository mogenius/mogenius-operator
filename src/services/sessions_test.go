package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/k8sexec"
)

// localShell is dash where there is one, else `sh`: dash is stricter than
// bash (which is `sh` on macOS) and closer to a container's shell.
func localShell() string {
	if _, err := exec.LookPath("dash"); err == nil {
		return "dash"
	}
	return "sh"
}

// The session manager against a local shell: the shell is plain POSIX and
// reads its commands from stdin, so everything but the exec transport is
// exactly what runs in the container.
func useLocalShell(t *testing.T) {
	t.Helper()
	oldTarget, oldShell, oldLogger := sessionTargetFn, sessionShellFn, serviceLogger
	serviceLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionTargetFn = func(context.Context, SessionCreateRequest) (*execPlan, error) {
		return &execPlan{shell: localShell(), container: "sandbox", targetContainer: "sandbox", maxOutput: 1 << 20}, nil
	}
	sessionShellFn = func(ctx context.Context, plan *execPlan, _, _ string, env map[string]string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		argv := k8sexec.BuildSessionShellCommand(plan.shell, env)
		if runtime.GOOS != "linux" {
			// the wrapper's watchdog walks /proc; elsewhere only env is wrapped
			argv = k8sexec.BuildTerminalShellCommand(plan.shell, env)
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		// Not cmd.Stdin: Wait would then also wait for the stdin copy, which
		// only ends when the session closes its pipe — the exec API returns
		// as soon as the process is gone, and so must this.
		pipe, err := cmd.StdinPipe()
		if err != nil {
			return 0, err
		}
		go func() {
			_, _ = io.Copy(pipe, stdin)
			_ = pipe.Close()
		}()
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Start(); err != nil {
			return 0, err
		}
		waited := make(chan error, 1)
		go func() { waited <- cmd.Wait() }()
		select {
		case err = <-waited:
		case <-ctx.Done():
			// The exec API closes the container's stdin and lets go; stopping
			// the process is the wrapper's job, as in the container.
			_ = pipe.Close()
			if runtime.GOOS != "linux" {
				_ = cmd.Process.Kill()
			}
			return 0, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() < 0 {
				// killed: what a container runtime reports as 128+signal
				return SessionEndedExitCode, nil
			}
			return exitErr.ExitCode(), nil
		}
		return 0, err
	}
	t.Cleanup(func() {
		sessions.mu.Lock()
		var open []*podSession
		for _, session := range sessions.sessions {
			if session.stdin != nil {
				open = append(open, session)
			}
		}
		sessions.mu.Unlock()
		for _, session := range open {
			session.close()
			<-session.done
			sessions.remove(session)
		}
		sessionTargetFn, sessionShellFn, serviceLogger = oldTarget, oldShell, oldLogger
	})
}

func sessionRef(id string) SessionRequest {
	return SessionRequest{
		SessionPodRequest: SessionPodRequest{Namespace: "ns", Pod: "pod", UserEmail: "jane@example.com"},
		SessionId:         id,
	}
}

func createTestSession(t *testing.T, id string) SessionRequest {
	t.Helper()
	ref := sessionRef(id)
	if _, err := CreateSession(SessionCreateRequest{SessionRequest: ref}); err != nil {
		t.Fatalf("create: %v", err)
	}
	return ref
}

func execSync(t *testing.T, ref SessionRequest, command string) SessionExecResponse {
	t.Helper()
	response, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: ref, Command: command, WaitSeconds: 10})
	if err != nil {
		t.Fatalf("exec %q: %v", command, err)
	}
	return response
}

// The acceptance case from the ticket: state carries over between commands.
func TestSessionKeepsShellState(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "state")

	first := execSync(t, ref, "cd /tmp && export FOO=bar")
	if first.ExitCode == nil || *first.ExitCode != 0 {
		t.Fatalf("first exit = %v", first.ExitCode)
	}
	second := execSync(t, ref, "echo $FOO $PWD")
	if second.Output == nil || !strings.HasPrefix(*second.Output, "bar /") || !strings.Contains(*second.Output, "tmp") {
		t.Fatalf("output = %q, want bar /…/tmp", deref(second.Output))
	}
}

func TestSessionSeparatesStreamsAndExitCode(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "streams")

	response := execSync(t, ref, "echo out; echo err >&2; exit 3")
	if response.ExitCode == nil || *response.ExitCode != 3 {
		t.Fatalf("exit = %v, want 3", response.ExitCode)
	}
	logs, err := GetSessionCommandLogs(SessionCommandRequest{SessionRequest: ref, CmdId: response.CmdId})
	if err != nil {
		t.Fatal(err)
	}
	if logs.Stdout != "out\n" || logs.Stderr != "err\n" {
		t.Errorf("stdout = %q stderr = %q", logs.Stdout, logs.Stderr)
	}
	// `exit` ends the shell and with it the session; the command kept the
	// status, the session stays readable but takes nothing new
	if _, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: ref, Command: "true"}); err != ErrSessionEnded {
		t.Errorf("exec after exit = %v, want ErrSessionEnded", err)
	}
	if list := ListSessions(ref.SessionPodRequest); len(list) != 0 {
		t.Errorf("an ended session is still listed: %+v", list)
	}
	if err := DeleteSession(ref); err != nil {
		t.Errorf("delete of an ended session = %v", err)
	}
	if _, err := GetSession(ref); err != ErrSessionNotFound {
		t.Errorf("session after delete = %v, want gone", err)
	}
}

// A failing command (as opposed to `exit`) leaves the session usable.
func TestSessionGoesOnAfterAFailedCommand(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "fail")
	response := execSync(t, ref, "false")
	if response.ExitCode == nil || *response.ExitCode != 1 {
		t.Fatalf("exit = %v, want 1", response.ExitCode)
	}
	if ok := execSync(t, ref, "echo still here"); deref(ok.Output) != "still here\n" {
		t.Errorf("output = %q", deref(ok.Output))
	}
}

// Quoting, newlines and an unfinished construct stay inside the eval; the
// end marker still arrives and the shell keeps taking commands.
func TestSessionSurvivesBrokenQuoting(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "quoting")

	broken := execSync(t, ref, "echo 'unterminated")
	if broken.ExitCode == nil || *broken.ExitCode == 0 {
		t.Fatalf("broken command exit = %v, want non-zero", broken.ExitCode)
	}
	multi := execSync(t, ref, "printf '%s\\n' \"it's\" 'a \"quote\"'\necho second line")
	if deref(multi.Output) != "it's\na \"quote\"\nsecond line\n" {
		t.Errorf("output = %q", deref(multi.Output))
	}
}

func TestSessionRunsInBackgroundAndFollowsLogs(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "async")

	started, err := ExecSessionCommand(context.Background(), SessionExecRequest{
		SessionRequest: ref, Command: "for i in 1 2 3; do echo $i; sleep 0.1; done; echo err >&2; exit 4", RunAsync: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.ExitCode != nil || started.Output != nil {
		t.Fatalf("a background run answers with the id only, got %+v", started)
	}
	// a second command while one runs is refused, one at a time
	if _, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: ref, Command: "true", RunAsync: true}); err != ErrSessionBusy {
		t.Fatalf("second exec = %v, want ErrSessionBusy", err)
	}

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	outcome, err := FollowSessionCommand(ctx, SessionCommandRequest{SessionRequest: ref, CmdId: started.CmdId}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "1\n2\n3\n" || stderr.String() != "err\n" || outcome.ExitCode != 4 {
		t.Errorf("stdout = %q stderr = %q exit = %d", stdout.String(), stderr.String(), outcome.ExitCode)
	}
	info, err := GetSessionCommand(SessionCommandRequest{SessionRequest: ref, CmdId: started.CmdId})
	if err != nil || info.ExitCode == nil || *info.ExitCode != 4 || info.FinishedAt == nil {
		t.Errorf("command info = %+v, %v", info, err)
	}
	// following a finished command replays it in full
	var replay bytes.Buffer
	if _, err := FollowSessionCommand(ctx, SessionCommandRequest{SessionRequest: ref, CmdId: started.CmdId}, &replay, io.Discard); err != nil || replay.String() != "1\n2\n3\n" {
		t.Errorf("replay = %q, %v", replay.String(), err)
	}
}

// A synchronous call stops waiting but the command goes on; its result
// arrives in the session later.
func TestSessionSyncWaitDoesNotStopTheCommand(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "wait")

	response, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: ref, Command: "echo started; sleep 1.5; echo late", WaitSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	if response.ExitCode != nil || deref(response.Output) != "started\n" {
		t.Fatalf("after the wait: exit = %v output = %q", response.ExitCode, deref(response.Output))
	}
	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	outcome, err := FollowSessionCommand(ctx, SessionCommandRequest{SessionRequest: ref, CmdId: response.CmdId}, &out, io.Discard)
	if err != nil || outcome.ExitCode != 0 || out.String() != "started\nlate\n" {
		t.Errorf("follow: out = %q exit = %d err = %v", out.String(), outcome.ExitCode, err)
	}
}

// Output that ends without a newline shares its last line with the end
// marker and still arrives intact, without an added newline.
func TestSessionKeepsOutputWithoutTrailingNewline(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "nonl")
	response := execSync(t, ref, "printf 'no newline'")
	if deref(response.Output) != "no newline" || response.ExitCode == nil || *response.ExitCode != 0 {
		t.Errorf("output = %q exit = %v", deref(response.Output), response.ExitCode)
	}
	response = execSync(t, ref, "printf 'prompt: '; read x; echo")
	_ = response
}

func TestSessionInputReachesTheRunningCommand(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "input")

	started, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: ref, Command: "read answer; echo got:$answer", RunAsync: true})
	if err != nil {
		t.Fatal(err)
	}
	// without a running command there is nothing to type into
	if err := SendSessionInput(SessionInputRequest{SessionCommandRequest: SessionCommandRequest{SessionRequest: ref, CmdId: "nope"}}); err != ErrSessionCommandNotFound {
		t.Errorf("input to unknown command = %v", err)
	}
	if err := SendSessionInput(SessionInputRequest{SessionCommandRequest: SessionCommandRequest{SessionRequest: ref, CmdId: started.CmdId}, Data: "hello\n"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	outcome, err := FollowSessionCommand(ctx, SessionCommandRequest{SessionRequest: ref, CmdId: started.CmdId}, &out, io.Discard)
	if err != nil || outcome.ExitCode != 0 || out.String() != "got:hello\n" {
		t.Errorf("out = %q exit = %d err = %v", out.String(), outcome.ExitCode, err)
	}
	if err := SendSessionInput(SessionInputRequest{SessionCommandRequest: SessionCommandRequest{SessionRequest: ref, CmdId: started.CmdId}, Data: "x"}); err != ErrSessionIdle {
		t.Errorf("input after the command ended = %v, want ErrSessionIdle", err)
	}
}

func TestSessionIsBoundToItsOwner(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "owner")

	other := ref
	other.UserEmail = "someone@example.com"
	if _, err := GetSession(other); err != ErrSessionNotFound {
		t.Errorf("another user's lookup = %v, want not found", err)
	}
	if _, err := CreateSession(SessionCreateRequest{SessionRequest: ref}); err != ErrSessionExists {
		t.Errorf("duplicate create = %v, want exists", err)
	}
	list := ListSessions(ref.SessionPodRequest)
	if len(list) != 1 || list[0].SessionId != "owner" || list[0].Container != "sandbox" {
		t.Errorf("list = %+v", list)
	}
}

func TestSessionDeleteEndsTheShellAndTheRunningCommand(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "delete")

	started, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: ref, Command: "sleep 30", RunAsync: true})
	if err != nil {
		t.Fatal(err)
	}
	session, err := sessions.get(ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteSession(ref); err != nil {
		t.Fatal(err)
	}
	select {
	case <-session.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the shell did not end after delete")
	}
	if _, err := GetSession(ref); err != ErrSessionNotFound {
		t.Errorf("after delete = %v, want not found", err)
	}
	// the command that was running is closed out with the session's code
	if info := started.CmdId; info != "" {
		session.mu.Lock()
		command := session.byId[info]
		session.mu.Unlock()
		if got := command.info(); got.ExitCode == nil || *got.ExitCode != SessionEndedExitCode {
			t.Errorf("running command after delete = %+v", got)
		}
	}
	if _, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: ref, Command: "true"}); err != ErrSessionNotFound {
		t.Errorf("exec after delete = %v", err)
	}
}

// Env reaches every command of the session but is never part of what the
// session reports about itself.
func TestSessionEnvReachesCommandsButNotInfo(t *testing.T) {
	useLocalShell(t)
	ref := sessionRef("env")
	const value = "made-up-value-4711 with 'quotes' and $dollar"
	if _, err := CreateSession(SessionCreateRequest{SessionRequest: ref, Env: map[string]string{"MO_TEST_SECRET": value}}); err != nil {
		t.Fatalf("create: %v", err)
	}
	response := execSync(t, ref, `printf '%s\n' "$MO_TEST_SECRET"`)
	if deref(response.Output) != value+"\n" {
		t.Fatalf("output = %q, want the value", deref(response.Output))
	}
	info, err := GetSession(ref)
	if err != nil {
		t.Fatal(err)
	}
	serialized, _ := json.Marshal(info)
	if strings.Contains(string(serialized), "made-up-value-4711") {
		t.Errorf("session info carries the value: %s", serialized)
	}
}

func TestSessionCreateRejectsInvalidEnv(t *testing.T) {
	useLocalShell(t)
	for _, name := range []string{"1X", "A=B", "", "A B"} {
		ref := sessionRef("badenv")
		if _, err := CreateSession(SessionCreateRequest{SessionRequest: ref, Env: map[string]string{name: "x"}}); err == nil {
			t.Errorf("env name %q accepted", name)
			_ = DeleteSession(ref)
		}
	}
}

// Deleting a session stops what it runs, children included — a long agent
// run must not outlive a cancelled test.
func TestSessionDeleteKillsTheCommandTree(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the watchdog walks /proc")
	}
	useLocalShell(t)
	ref := createTestSession(t, "killtree")
	pids := filepath.Join(t.TempDir(), "pids")
	command := "sleep 300 & echo $! > " + pids + "; (sleep 300 & echo $! >> " + pids + "; wait) & wait"
	if _, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: ref, Command: command, RunAsync: true}); err != nil {
		t.Fatal(err)
	}
	var children []int
	for deadline := time.Now().Add(5 * time.Second); len(children) < 2; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("children did not start: %v", children)
		}
		data, _ := os.ReadFile(pids)
		children = children[:0]
		for _, field := range strings.Fields(string(data)) {
			if pid, err := strconv.Atoi(field); err == nil {
				children = append(children, pid)
			}
		}
	}
	session, err := sessions.get(ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteSession(ref); err != nil {
		t.Fatal(err)
	}
	select {
	case <-session.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the shell did not end after delete")
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		alive := []int{}
		for _, pid := range children {
			if processAlive(pid) {
				alive = append(alive, pid)
			}
		}
		if len(alive) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("still running after delete: %v", alive)
		}
	}
}

// A shell that ends on its own still reports its status through the
// wrapper, and does not wait for the session's stdin to close.
func TestSessionExitEndsTheSessionWithItsStatus(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "exit")
	session, err := sessions.get(ref)
	if err != nil {
		t.Fatal(err)
	}
	response := execSync(t, ref, "exit 3")
	if response.ExitCode == nil || *response.ExitCode != 3 {
		t.Fatalf("exit = %v, want 3", response.ExitCode)
	}
	select {
	case <-session.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the session did not end with its shell")
	}
}

// processAlive treats a zombie as gone: it is dead, just not yet reaped by
// whoever inherited it.
func processAlive(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
	return len(fields) > 0 && fields[0] != "Z"
}

func TestSessionIdleSweep(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "idle")
	execSync(t, ref, "true")
	createTestSession(t, "gone")

	busy, err := CreateSession(SessionCreateRequest{SessionRequest: sessionRef("busy")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: sessionRef("busy"), Command: "sleep 30", RunAsync: true}); err != nil {
		t.Fatal(err)
	}
	// an ended session counts as idle too, so the janitor forgets it eventually
	execSync(t, sessionRef("gone"), "exit 0")
	time.Sleep(20 * time.Millisecond)
	idle := sessions.idleSince(time.Now())
	ids := map[string]bool{}
	for _, session := range idle {
		ids[session.id] = true
	}
	if len(ids) != 2 || !ids["idle"] || !ids["gone"] {
		t.Errorf("idle sessions = %v, want idle and gone but not busy (%s)", ids, busy.SessionId)
	}
}

func TestSessionOutputCapTruncates(t *testing.T) {
	useLocalShell(t)
	oldTarget := sessionTargetFn
	sessionTargetFn = func(context.Context, SessionCreateRequest) (*execPlan, error) {
		return &execPlan{shell: localShell(), container: "sandbox", targetContainer: "sandbox", maxOutput: 8}, nil
	}
	t.Cleanup(func() { sessionTargetFn = oldTarget })
	ref := createTestSession(t, "cap")

	response := execSync(t, ref, "printf 0123456789; echo; echo more")
	request := SessionCommandRequest{SessionRequest: ref, CmdId: response.CmdId}
	logs, _ := GetSessionCommandLogs(request)
	if logs.Output != "01234567" || !logs.Truncated || response.ExitCode == nil || *response.ExitCode != 0 {
		t.Errorf("logs = %+v exit = %v", logs, response.ExitCode)
	}
	// the command and its log stream say so too, so a client can tell a cut from a lost connection
	info, err := GetSessionCommand(request)
	if err != nil || !info.Truncated {
		t.Errorf("command info = %+v, %v", info, err)
	}
	var followed bytes.Buffer
	outcome, err := FollowSessionCommand(context.Background(), request, &followed, io.Discard)
	if err != nil || !outcome.Truncated || outcome.ExitCode != 0 || followed.String() != "01234567" {
		t.Errorf("follow = %+v %q, %v", outcome, followed.String(), err)
	}

	short := execSync(t, ref, "echo ok")
	request = SessionCommandRequest{SessionRequest: ref, CmdId: short.CmdId}
	info, _ = GetSessionCommand(request)
	outcome, _ = FollowSessionCommand(context.Background(), request, io.Discard, io.Discard)
	if info.Truncated || outcome.Truncated {
		t.Errorf("a command under the cap is marked truncated: %+v %+v", info, outcome)
	}
}

// Session commands have their own cap, 16 MiB unless set, 0 for none; the
// exec request's stays apart.
func TestSessionCommandOutputCap(t *testing.T) {
	old := config
	t.Cleanup(func() { config = old })
	config = nil
	if got := SessionCommandOutputCap(); got != 16<<20 {
		t.Errorf("without config = %d", got)
	}
	module := cfg.NewConfig()
	for _, key := range []string{SessionConfigKeyMaxOutput, ExecRequestConfigKeyMaxOutput} {
		module.Declare(cfg.ConfigDeclaration{Key: key, Type: new(cfg.ConfigVariableTypeInt)})
	}
	module.Set(ExecRequestConfigKeyMaxOutput, "1048576")
	config = module
	for value, want := range map[string]int{"0": 0, "1048576": 1 << 20, "-1": 16 << 20} {
		module.Set(SessionConfigKeyMaxOutput, value)
		if got := SessionCommandOutputCap(); got != want {
			t.Errorf("%s → %d, want %d", value, got, want)
		}
	}
	module.Set(SessionConfigKeyMaxOutput, "0")
	if got := ExecOutputCap(); got != 1<<20 {
		t.Errorf("exec request cap = %d", got)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestSessionRejectsAnInvalidId(t *testing.T) {
	useLocalShell(t)
	for _, id := range []string{"", "-dash", "has space", "semi;colon", "x/y"} {
		if _, err := CreateSession(SessionCreateRequest{SessionRequest: sessionRef(id)}); err != ErrSessionInvalidId {
			t.Errorf("CreateSession(%q) = %v, want ErrSessionInvalidId", id, err)
		}
	}
}

// stdout and stderr arrive in no fixed order: stderr that comes in after the
// stdout end marker still belongs to the command, in either marker order.
func TestSessionStderrAfterTheStdoutEndMarker(t *testing.T) {
	for _, stderrMarkerFirst := range []bool{false, true} {
		session := &podSession{byId: map[string]*sessionCommand{}}
		command := newSessionCommand("c1", "broken", 0)
		session.byId[command.id] = command
		session.current = command

		session.feedStdout([]byte(sessionMarkerBegin + "c1" + sessionMarkerTail + "\n"))
		if stderrMarkerFirst {
			session.feedStderr([]byte("syntax error\n" + sessionMarkerEnd + "c1" + sessionMarkerTail + "\n"))
			session.feedStdout([]byte(sessionMarkerEnd + "c1_2" + sessionMarkerTail + "\n"))
		} else {
			session.feedStdout([]byte(sessionMarkerEnd + "c1_2" + sessionMarkerTail + "\n"))
			if command.info().ExitCode != nil {
				t.Fatal("finished before its stderr was in")
			}
			session.feedStderr([]byte("syntax error\n" + sessionMarkerEnd + "c1" + sessionMarkerTail + "\n"))
		}
		info := command.info()
		if info.ExitCode == nil || *info.ExitCode != 2 {
			t.Fatalf("stderr marker first = %v: exit = %v", stderrMarkerFirst, info.ExitCode)
		}
		if logs := command.logs(); logs.Stderr != "syntax error\n" || logs.Stdout != "" {
			t.Errorf("stderr marker first = %v: stdout = %q stderr = %q", stderrMarkerFirst, logs.Stdout, logs.Stderr)
		}
	}
}

// A command that closes its stderr never prints the stderr end marker; it
// finishes after the grace all the same.
func TestSessionFinishesWithoutStderr(t *testing.T) {
	useLocalShell(t)
	ref := createTestSession(t, "nostderr")
	response := execSync(t, ref, "exec 2>&-; echo hi")
	if response.ExitCode == nil || *response.ExitCode != 0 || deref(response.Output) != "hi\n" {
		t.Errorf("output = %q exit = %v", deref(response.Output), response.ExitCode)
	}
}
