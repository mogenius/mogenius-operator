package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"k8s.io/client-go/tools/remotecommand"
)

// The terminal session against a local pty: the shell sees a real TTY, so
// echo, prompts and resizes behave as in the container.
func useLocalPty(t *testing.T) {
	t.Helper()
	// registered before useLocalShell's cleanup so it runs after it (LIFO):
	// the hook is only restored once every shell has ended
	old := sessionTTYShellFn
	t.Cleanup(func() { sessionTTYShellFn = old })
	useLocalShell(t)
	sessionTTYShellFn = func(ctx context.Context, plan *execPlan, _, _ string, stdin io.Reader, output io.Writer, sizes remotecommand.TerminalSizeQueue) (int, error) {
		cmd := exec.CommandContext(ctx, plan.shell)
		cmd.Env = []string{"PS1=$ ", "TERM=xterm", "PATH=/usr/bin:/bin"}
		f, err := pty.Start(cmd)
		if err != nil {
			return 0, err
		}
		go func() { _, _ = io.Copy(f, stdin) }()
		go func() { _, _ = io.Copy(output, f) }()
		// the pty is closed below while sizes may still arrive: resize under the same lock
		var fMu sync.Mutex
		fClosed := false
		go func() {
			for {
				size := sizes.Next()
				if size == nil {
					return
				}
				fMu.Lock()
				if !fClosed {
					_ = pty.Setsize(f, &pty.Winsize{Rows: size.Height, Cols: size.Width})
				}
				fMu.Unlock()
			}
		}()
		err = cmd.Wait()
		fMu.Lock()
		fClosed = true
		_ = f.Close()
		fMu.Unlock()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() < 0 {
				return SessionEndedExitCode, nil
			}
			return exitErr.ExitCode(), nil
		}
		return 0, err
	}
}

// readUntil collects output until it contains want or the deadline passes.
func readUntil(t *testing.T, output <-chan []byte, want string) string {
	t.Helper()
	var buf bytes.Buffer
	deadline := time.After(5 * time.Second)
	for {
		if strings.Contains(buf.String(), want) {
			return buf.String()
		}
		select {
		case data, ok := <-output:
			if !ok {
				t.Fatalf("output closed before %q arrived; got %q", want, buf.String())
			}
			buf.Write(data)
		case <-deadline:
			t.Fatalf("no %q within 5s; got %q", want, buf.String())
		}
	}
}

func terminalRequest(id string) SessionCreateRequest {
	return SessionCreateRequest{SessionRequest: sessionRef(id), Tty: true}
}

// The acceptance case from the ticket: detach, re-attach, the shell and its
// output are still there.
func TestTerminalSessionSurvivesDetach(t *testing.T) {
	useLocalPty(t)

	first, err := AttachTerminalSession(terminalRequest("term"))
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Replay()) != 0 {
		t.Errorf("a new session has no scrollback, got %q", first.Replay())
	}
	if err := first.Input([]byte("export FOO=bar; echo marker-$FOO\n")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, first.Output(), "marker-bar")
	first.Detach()

	second, err := AttachTerminalSession(terminalRequest("term"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(second.Replay()), "marker-bar") {
		t.Errorf("replay = %q, want the earlier output", second.Replay())
	}
	if err := second.Input([]byte("echo again-$FOO\n")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, second.Output(), "again-bar")
	second.Detach()

	list := ListSessions(sessionRef("term").SessionPodRequest)
	if len(list) != 1 || !list[0].Tty || list[0].SessionId != "term" {
		t.Errorf("list = %+v", list)
	}
	if _, err := ExecSessionCommand(context.Background(), SessionExecRequest{SessionRequest: sessionRef("term"), Command: "true"}); err != ErrSessionIsTerminal {
		t.Errorf("exec on a terminal session = %v", err)
	}
}

func TestTerminalSessionResizeAndDelete(t *testing.T) {
	useLocalPty(t)

	attachment, err := AttachTerminalSession(terminalRequest("size"))
	if err != nil {
		t.Fatal(err)
	}
	attachment.Resize(120, 40)
	if err := attachment.Input([]byte("stty size\n")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, attachment.Output(), "40 120")

	if err := DeleteSession(sessionRef("size")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-attachment.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session did not end after delete")
	}
	if _, err := AttachTerminalSession(SessionCreateRequest{SessionRequest: sessionRef("size")}); err != nil {
		// a fresh session of the same id can be opened right away
		t.Errorf("re-create after delete = %v", err)
	}
}

// Someone attached keeps the session from idling out; a detached one idles.
func TestTerminalSessionIdlesOnlyWhenDetached(t *testing.T) {
	useLocalPty(t)
	attachment, err := AttachTerminalSession(terminalRequest("idle-term"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if idle := sessions.idleSince(time.Now()); len(idle) != 0 {
		t.Errorf("an attached terminal counted as idle: %d", len(idle))
	}
	attachment.Detach()
	time.Sleep(20 * time.Millisecond)
	if idle := sessions.idleSince(time.Now()); len(idle) != 1 {
		t.Errorf("a detached terminal did not count as idle: %d", len(idle))
	}
}

// A resize that races the end of the session is dropped; it must not panic
// on the closed size queue. Detaching twice, or after the session ended, is
// harmless too.
func TestTerminalSessionToleratesLateResizeAndDetach(t *testing.T) {
	useLocalPty(t)
	attachment, err := AttachTerminalSession(terminalRequest("late"))
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteSession(sessionRef("late")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-attachment.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session did not end")
	}
	attachment.Resize(100, 30)
	attachment.Detach()
	attachment.Detach()
	if !attachment.Ended() {
		t.Error("Ended() = false after the shell ended")
	}
}

// Many clients attaching, typing, resizing and leaving at once: the race
// detector checks the fan-out, and nothing may block the shell.
func TestTerminalSessionConcurrentAttachments(t *testing.T) {
	useLocalPty(t)
	first, err := AttachTerminalSession(terminalRequest("busy-term"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a, err := AttachTerminalSession(terminalRequest("busy-term"))
			if err != nil {
				t.Error(err)
				return
			}
			a.Resize(uint16(80+i), 24)
			_ = a.Input([]byte("echo x\n"))
			// never read: lets the fan-out drop it for falling behind while it detaches
			a.Detach()
		}(i)
	}
	wg.Wait()
	if err := first.Input([]byte("echo still-alive\n")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, first.Output(), "still-alive")
	first.Detach()
}

// The list says how many terminals hang on a session, so a client can resume
// one nobody is using instead of mirroring one that is open elsewhere.
func TestTerminalSessionCountsAttachedTerminals(t *testing.T) {
	useLocalPty(t)
	first, err := AttachTerminalSession(terminalRequest("count"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := AttachTerminalSession(terminalRequest("count"))
	if err != nil {
		t.Fatal(err)
	}
	attached := func() int {
		info, err := GetSession(sessionRef("count"))
		if err != nil {
			t.Fatal(err)
		}
		return info.Attached
	}
	if got := attached(); got != 2 {
		t.Errorf("attached = %d, want 2", got)
	}
	first.Detach()
	second.Detach()
	if got := attached(); got != 0 {
		t.Errorf("attached after detaching = %d, want 0", got)
	}
}
