package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"mogenius-operator/src/k8sexec"

	"k8s.io/client-go/tools/remotecommand"
)

// Terminal sessions (MOG-4759): a pod session with a pseudo-terminal, owned
// by the operator so a browser terminal or an SSH client can attach, go
// away and come back — like `tmux attach`. The shell runs on while nobody
// is attached; its output is kept in a scrollback buffer that a new
// attachment is replayed before it goes live. Commands are not fed to a
// terminal session through the API: people type into it.
const (
	// SessionConfigKeyScrollback caps how much of a terminal session's output
	// is kept for the next attachment.
	SessionConfigKeyScrollback   = "MO_SESSION_SCROLLBACK_BYTES"
	sessionDefaultScrollback     = 256 << 10
	sessionAttachmentQueueFrames = 256
)

var ErrSessionNotTerminal = errors.New("not a terminal session")

// sessionTTYShellFn starts a terminal session's shell; tests replace it
// with a local pty.
var sessionTTYShellFn = func(ctx context.Context, plan *execPlan, namespace, pod string, stdin io.Reader, output io.Writer, sizes remotecommand.TerminalSizeQueue) (int, error) {
	return k8sexec.StreamTTY(ctx, plan.clients, k8sexec.RunRequest{
		Namespace: namespace,
		Pod:       pod,
		Container: plan.container,
		Command:   []string{plan.shell},
	}, stdin, output, sizes)
}

// ptyState is what a terminal session has that a command session has not.
type ptyState struct {
	mu          sync.Mutex
	scrollback  []byte
	limit       int
	attachments map[*SessionAttachment]struct{}
	sizes       *ptySizeQueue
}

// SessionAttachment is one client attached to a terminal session. Output
// arrives on Output after the Replay; Done closes when the session ends.
type SessionAttachment struct {
	session *podSession
	output  chan []byte
	replay  []byte
	// closed: output is closed; guarded by the session's pty.mu, so a detach
	// and a drop for falling behind can meet without closing it twice
	closed bool
}

// closeLocked closes Output once and forgets the attachment; the caller
// holds the session's pty.mu.
func (a *SessionAttachment) closeLocked() {
	delete(a.session.pty.attachments, a)
	if !a.closed {
		a.closed = true
		close(a.output)
	}
}

// Replay is the scrollback at the time of attaching: what a terminal shows
// before live output continues.
func (a *SessionAttachment) Replay() []byte { return a.replay }

// Output delivers live output; the channel closes when the attachment is
// detached or fell behind (the client then re-attaches and gets the
// scrollback) or the session ended.
func (a *SessionAttachment) Output() <-chan []byte { return a.output }

// Done closes when the session's shell has ended.
func (a *SessionAttachment) Done() <-chan struct{} { return a.session.done }

// Input writes what the client typed to the shell.
func (a *SessionAttachment) Input(data []byte) error {
	_, err := a.session.stdin.Write(data)
	return err
}

// Resize hands a new window size to the shell.
func (a *SessionAttachment) Resize(cols, rows uint16) {
	a.session.pty.sizes.set(remotecommand.TerminalSize{Width: cols, Height: rows})
}

// Ended reports whether the session's shell has ended. When Output closes
// and the session has not ended, the attachment fell behind and the client
// should re-attach.
func (a *SessionAttachment) Ended() bool {
	a.session.mu.Lock()
	defer a.session.mu.Unlock()
	return a.session.ended
}

// Detach removes the attachment; the session goes on. Safe to call more
// than once and after the session has ended.
func (a *SessionAttachment) Detach() {
	a.session.pty.mu.Lock()
	a.closeLocked()
	a.session.pty.mu.Unlock()
	a.session.mu.Lock()
	a.session.lastUsed = time.Now()
	a.session.mu.Unlock()
}

// ptySizeQueue feeds window sizes to the exec API: the latest size always
// wins, and Next blocks until there is a new one. A resize that arrives
// after the session ended is dropped, not sent on the closed channel.
type ptySizeQueue struct {
	mu     sync.Mutex
	ch     chan remotecommand.TerminalSize
	closed bool
}

func newPtySizeQueue() *ptySizeQueue {
	return &ptySizeQueue{ch: make(chan remotecommand.TerminalSize, 1)}
}

func (q *ptySizeQueue) set(size remotecommand.TerminalSize) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	// drop a size nobody has read yet: only the latest matters
	select {
	case <-q.ch:
	default:
	}
	q.ch <- size
}

// close ends Next for the exec API; later sizes are dropped.
func (q *ptySizeQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
}

// Next implements remotecommand.TerminalSizeQueue.
func (q *ptySizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.ch
	if !ok {
		return nil
	}
	return &size
}

// AttachTerminalSession attaches to the terminal session, creating it when
// there is none of that id yet (`tmux new -A`). The caller owns the returned
// attachment and detaches it when its client goes away.
func AttachTerminalSession(request SessionCreateRequest) (*SessionAttachment, error) {
	request.Tty = true
	session, err := sessions.get(request.SessionRequest)
	if err != nil {
		if _, err := CreateSession(request); err != nil {
			return nil, err
		}
		session, err = sessions.get(request.SessionRequest)
		if err != nil {
			return nil, err
		}
	}
	return session.attach()
}

func (s *podSession) attach() (*SessionAttachment, error) {
	if s.pty == nil {
		return nil, ErrSessionNotTerminal
	}
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return nil, ErrSessionEnded
	}
	s.lastUsed = time.Now()
	s.mu.Unlock()

	s.pty.mu.Lock()
	defer s.pty.mu.Unlock()
	attachment := &SessionAttachment{
		session: s,
		output:  make(chan []byte, sessionAttachmentQueueFrames),
		replay:  append([]byte(nil), s.pty.scrollback...),
	}
	s.pty.attachments[attachment] = struct{}{}
	return attachment, nil
}

// feedPTY keeps the output in the scrollback and hands it to every
// attachment. An attachment that does not keep up is dropped rather than
// stalling the shell; its client re-attaches and reads the scrollback.
func (s *podSession) feedPTY(p []byte) {
	s.pty.mu.Lock()
	defer s.pty.mu.Unlock()
	s.pty.scrollback = append(s.pty.scrollback, p...)
	if s.pty.limit > 0 && len(s.pty.scrollback) > s.pty.limit {
		s.pty.scrollback = append([]byte(nil), s.pty.scrollback[len(s.pty.scrollback)-s.pty.limit:]...)
	}
	for attachment := range s.pty.attachments {
		select {
		case attachment.output <- p:
		default:
			attachment.closeLocked()
		}
	}
}

// endPTY is end's part for a terminal session: the attachments learn it
// through Done; their channels close so a reader loop ends either way.
func (s *podSession) endPTY() {
	s.pty.mu.Lock()
	defer s.pty.mu.Unlock()
	for attachment := range s.pty.attachments {
		attachment.closeLocked()
	}
	s.pty.sizes.close()
}

// startTerminalSession is startSession for tty: true.
func startTerminalSession(request SessionCreateRequest, key string, plan *execPlan) (*podSession, error) {
	ctx, cancel := context.WithCancel(context.Background())
	stdinReader, stdinWriter := io.Pipe()
	now := time.Now()
	session := &podSession{
		key:       key,
		id:        request.SessionId,
		namespace: request.Namespace,
		pod:       request.Pod,
		container: plan.targetContainer,
		owner:     request.UserEmail,
		tty:       true,
		createdAt: now,
		lastUsed:  now,
		stdin:     stdinWriter,
		cancel:    cancel,
		done:      make(chan struct{}),
		byId:      map[string]*sessionCommand{},
		pty: &ptyState{
			limit:       SessionScrollbackBytes(),
			attachments: map[*SessionAttachment]struct{}{},
			sizes:       newPtySizeQueue(),
		},
	}
	// a size before the first terminal reports its own
	session.pty.sizes.set(remotecommand.TerminalSize{Width: 80, Height: 24})
	go func() {
		defer close(session.done)
		exitCode, err := sessionTTYShellFn(ctx, plan, request.Namespace, request.Pod, stdinReader, &sessionPTYOutput{session: session}, session.pty.sizes)
		if err != nil {
			if ctx.Err() == nil {
				serviceLogger.Info("terminal session shell ended", "namespace", request.Namespace, "pod", request.Pod, "sessionId", request.SessionId, "error", err)
			}
			exitCode = SessionEndedExitCode
		}
		session.end(exitCode)
	}()
	if plan.cwd != "" {
		if _, err := io.WriteString(stdinWriter, "cd -- "+shellQuote(plan.cwd)+"\n"); err != nil {
			session.close()
			return nil, fmt.Errorf("session: %w", err)
		}
	}
	return session, nil
}

type sessionPTYOutput struct{ session *podSession }

func (w *sessionPTYOutput) Write(p []byte) (int, error) {
	w.session.feedPTY(append([]byte(nil), p...))
	return len(p), nil
}

// SessionScrollbackBytes is the configured scrollback cap, or the default.
func SessionScrollbackBytes() int {
	if config != nil {
		if bytes, err := config.TryGetInt(SessionConfigKeyScrollback); err == nil && bytes > 0 {
			return int(bytes)
		}
	}
	return sessionDefaultScrollback
}
