package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"mogenius-operator/src/k8sexec"
	"mogenius-operator/src/stream"
	"mogenius-operator/src/utils"
)

// Pod sessions (MOG-4692): one long-lived shell per sessionId in a pod's
// container, owned by the operator. Commands are fed to it one at a time
// through stdin, so `cd` and `export` carry over; its output is split into
// commands by marker lines and buffered per command, for the synchronous
// answer, the logs route and the log follow stream. A session lives until it
// is deleted, idles out, or its shell ends — the pod is gone, or a command
// ran `exit`, which then reports the shell's status as its own. The registry is in this process: the chart runs one
// operator replica, and a session dies with the replica anyway.
const (
	// SessionConfigKeyIdleTimeout is how long a session without a running
	// command is kept before the janitor closes it.
	SessionConfigKeyIdleTimeout = "MO_SESSION_IDLE_TIMEOUT_SECONDS"
	sessionDefaultIdleTimeout   = 30 * time.Minute
	sessionJanitorInterval      = time.Minute
	// sessionDefaultWait is how long a synchronous exec waits for its command
	// when the request names no timeout.
	sessionDefaultWait = 60 * time.Second
	// SessionEndedExitCode is recorded for a command the session ended under,
	// the code a shell reports for a killed process.
	SessionEndedExitCode = k8sexec.KilledExitCode

	// Marker lines the shell prints around every command. The end marker
	// carries the command's exit status.
	sessionMarkerBegin = "__MO_B_"
	sessionMarkerEnd   = "__MO_E_"
	sessionMarkerTail  = "__"
)

// sessionIdPattern is what a session may be called — the platform's DTOs and
// mocli check the same, the SSH gateway's MO_SESSION only here.
var sessionIdPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`)

var (
	ErrSessionInvalidId       = errors.New("invalid session id: 1-63 letters, digits, '_', '-' or '.', starting with a letter or digit")
	ErrSessionNotFound        = errors.New("session not found")
	ErrSessionExists          = errors.New("session already exists")
	ErrSessionBusy            = errors.New("a command is still running in the session")
	ErrSessionCommandNotFound = errors.New("command not found in the session")
	ErrSessionIdle            = errors.New("no command is running in the session")
	ErrSessionEnded           = errors.New("the session has ended")
	ErrSessionIsTerminal      = errors.New("a terminal session takes no commands through the API; attach a terminal")
)

// SessionPodRequest names the pod a session request is about. Identity
// fields as in ExecRequest: the platform's judgement, set by the handler.
type SessionPodRequest struct {
	Namespace      string `json:"namespace" validate:"required"`
	Pod            string `json:"pod" validate:"required"`
	IsAdmin        bool   `json:"isAdmin"`
	IsClusterAdmin bool   `json:"isClusterAdmin"`
	UserEmail      string `json:"-"`
}

// SessionRequest names one session of a pod.
type SessionRequest struct {
	SessionPodRequest
	SessionId string `json:"sessionId" validate:"required"`
}

// SessionCreateRequest is the payload of `service/session-create`.
type SessionCreateRequest struct {
	SessionRequest
	// Container is optional; the pod's first container is used when empty.
	Container string `json:"container"`
	// Tty asks for a terminal session to attach a terminal to (MOG-4759).
	Tty bool `json:"tty"`
	// DebugContainer (terminal sessions): open the shell in the ephemeral
	// debug container targeting Container — the user's explicit choice for an
	// image without a shell. A terminal session never attaches one on its own;
	// it reports NO_SHELL_AVAILABLE instead, since the container stays on the
	// pod. Command sessions keep the exec request's rule and fall back.
	DebugContainer bool `json:"debugContainer"`
}

// SessionExecRequest is the payload of `service/session-exec`.
type SessionExecRequest struct {
	SessionRequest
	Command string `json:"command" validate:"required"`
	// RunAsync returns right away with the command id.
	RunAsync bool `json:"runAsync"`
	// WaitSeconds bounds how long a synchronous call waits; 0 means the
	// default. The command is not stopped when the wait is over.
	WaitSeconds int `json:"timeout"`
}

// SessionCommandRequest names one command of a session.
type SessionCommandRequest struct {
	SessionRequest
	CmdId string `json:"cmdId" validate:"required"`
}

// SessionInputRequest is the payload of `service/session-input`: text for
// the standard input of the running command, written verbatim.
type SessionInputRequest struct {
	SessionCommandRequest
	Data string `json:"data"`
}

// SessionLogStreamRequest is the payload of
// `service/session-log-stream-connection-request`: the command's output so
// far and then live, over a stream socket with the exec stream's frames.
type SessionLogStreamRequest struct {
	SessionCommandRequest
	WsConnection stream.WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
}

// SessionInfo describes a session and its commands, oldest first.
type SessionInfo struct {
	SessionId string `json:"sessionId"`
	Namespace string `json:"namespace"`
	PodName   string `json:"podName"`
	Container string `json:"container"`
	Tty       bool   `json:"tty"`
	// Attached counts the terminals attached right now (terminal sessions);
	// a client resuming its last session prefers one nobody is using.
	Attached   int                  `json:"attached"`
	CreatedAt  time.Time            `json:"createdAt"`
	LastUsedAt time.Time            `json:"lastUsedAt"`
	Commands   []SessionCommandInfo `json:"commands"`
}

// SessionCommandInfo describes one command; ExitCode and FinishedAt are nil
// while it runs.
type SessionCommandInfo struct {
	Id         string     `json:"id"`
	Command    string     `json:"command"`
	ExitCode   *int       `json:"exitCode"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// SessionExecResponse answers a session exec. ExitCode is nil for a
// background run and for a command still running when the wait was over;
// Output is absent for a background run.
type SessionExecResponse struct {
	CmdId    string  `json:"cmdId"`
	ExitCode *int    `json:"exitCode"`
	Output   *string `json:"output,omitempty"`
}

// SessionLogsResponse is what a command has printed so far.
type SessionLogsResponse struct {
	Output    string `json:"output"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  *int   `json:"exitCode"`
	Truncated bool   `json:"truncated"`
}

// SessionAuditSummary is what the audit log keeps of a session exec.
type SessionAuditSummary struct {
	SessionId string `json:"sessionId"`
	CmdId     string `json:"cmdId"`
	ExitCode  *int   `json:"exitCode"`
}

// Hooks the tests replace: how a session finds its target and how its shell
// is started. In production the target comes from the requester's clients
// and the shell runs through the exec API like every other command.
var (
	sessionTargetFn = func(ctx context.Context, request SessionCreateRequest) (*execPlan, error) {
		fallback := debugFallbackAuto
		if request.Tty {
			fallback = debugFallbackNever
			if request.DebugContainer {
				fallback = debugFallbackAlways
			}
		}
		clients, shell, targetContainer, container, cwd, err := resolveTarget(ctx, k8sexec.Identity{
			Email:   request.UserEmail,
			IsAdmin: request.IsAdmin || request.IsClusterAdmin,
		}, request.Namespace, request.Pod, request.Container, "", fallback)
		if err != nil {
			return nil, fmt.Errorf("session: %w", err)
		}
		return &execPlan{clients: clients, shell: shell, container: container, targetContainer: targetContainer, cwd: cwd, maxOutput: ExecOutputCap()}, nil
	}
	// The shell's own exit status is returned: an `exit 3` in a command ends
	// the shell, and that command then reports 3.
	sessionShellFn = func(ctx context.Context, plan *execPlan, namespace, pod string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return k8sexec.Stream(ctx, plan.clients, k8sexec.RunRequest{
			Namespace: namespace,
			Pod:       pod,
			Container: plan.container,
			Command:   []string{plan.shell},
		}, stdin, stdout, stderr)
	}
)

/***********************************************************************************************************************
 * registry
 **********************************************************************************************************************/

type sessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]*podSession
}

var sessions = &sessionRegistry{sessions: map[string]*podSession{}}

func sessionKey(namespace, pod, id string) string {
	return namespace + "/" + pod + "/" + id
}

// get returns the session, if the requester owns it. Another user's session
// is not found rather than forbidden: the shell runs as its owner, so nobody
// else gets to type into it.
func (r *sessionRegistry) get(request SessionRequest) (*podSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[sessionKey(request.Namespace, request.Pod, request.SessionId)]
	if !ok || session.owner != request.UserEmail {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (r *sessionRegistry) remove(session *podSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessions[session.key] == session {
		delete(r.sessions, session.key)
	}
}

// CreateSession starts the session's shell and registers it.
func CreateSession(request SessionCreateRequest) (SessionInfo, error) {
	if !sessionIdPattern.MatchString(request.SessionId) {
		return SessionInfo{}, ErrSessionInvalidId
	}
	key := sessionKey(request.Namespace, request.Pod, request.SessionId)
	sessions.mu.Lock()
	if _, exists := sessions.sessions[key]; exists {
		sessions.mu.Unlock()
		return SessionInfo{}, ErrSessionExists
	}
	// reserve the key while the shell starts, so two creates cannot race
	placeholder := &podSession{key: key, owner: request.UserEmail}
	sessions.sessions[key] = placeholder
	sessions.mu.Unlock()

	session, err := startSession(request, key)
	sessions.mu.Lock()
	if err != nil {
		delete(sessions.sessions, key)
	} else {
		sessions.sessions[key] = session
	}
	sessions.mu.Unlock()
	if err != nil {
		return SessionInfo{}, err
	}
	return session.info(), nil
}

// ListSessions lists the requester's sessions in the pod.
func ListSessions(request SessionPodRequest) []SessionInfo {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	result := []SessionInfo{}
	prefix := sessionKey(request.Namespace, request.Pod, "")
	for key, session := range sessions.sessions {
		if !strings.HasPrefix(key, prefix) || session.owner != request.UserEmail || session.stdin == nil {
			continue
		}
		// an ended session is kept for its results, but it is nothing to list
		session.mu.Lock()
		ended := session.ended
		session.mu.Unlock()
		if !ended {
			result = append(result, session.info())
		}
	}
	return result
}

// GetSession describes one session.
func GetSession(request SessionRequest) (SessionInfo, error) {
	session, err := sessions.get(request)
	if err != nil {
		return SessionInfo{}, err
	}
	return session.info(), nil
}

// DeleteSession ends the session and forgets it: its shell loses stdin and
// the exec stream, which ends the shell once the running command, if any,
// has returned.
func DeleteSession(request SessionRequest) error {
	session, err := sessions.get(request)
	if err != nil {
		return err
	}
	session.close()
	sessions.remove(session)
	return nil
}

// ExecSessionCommand feeds one command to the session's shell. A synchronous
// call waits for it up to WaitSeconds and answers with what it printed; a
// background call answers right away. Either way the command keeps running
// in the shell, and its result stays available in the session.
func ExecSessionCommand(ctx context.Context, request SessionExecRequest) (SessionExecResponse, error) {
	session, err := sessions.get(request.SessionRequest)
	if err != nil {
		return SessionExecResponse{}, err
	}
	command, err := session.exec(request.Command)
	if err != nil {
		return SessionExecResponse{}, err
	}
	if request.RunAsync {
		return SessionExecResponse{CmdId: command.id}, nil
	}
	wait := sessionDefaultWait
	if request.WaitSeconds > 0 {
		wait = time.Duration(request.WaitSeconds) * time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	command.waitDone(waitCtx)
	logs := command.logs()
	return SessionExecResponse{CmdId: command.id, ExitCode: logs.ExitCode, Output: &logs.Output}, nil
}

// GetSessionCommand describes one command of a session.
func GetSessionCommand(request SessionCommandRequest) (SessionCommandInfo, error) {
	command, err := findSessionCommand(request)
	if err != nil {
		return SessionCommandInfo{}, err
	}
	return command.info(), nil
}

// GetSessionCommandLogs is what the command has printed so far.
func GetSessionCommandLogs(request SessionCommandRequest) (SessionLogsResponse, error) {
	command, err := findSessionCommand(request)
	if err != nil {
		return SessionLogsResponse{}, err
	}
	return command.logs(), nil
}

// SendSessionInput writes to the standard input of the running command.
func SendSessionInput(request SessionInputRequest) error {
	session, err := sessions.get(request.SessionRequest)
	if err != nil {
		return err
	}
	return session.input(request.CmdId, request.Data)
}

// FollowSessionCommand writes what the command has printed so far to the
// writers, then everything it prints until it ends or ctx is done. The exit
// code is the command's; a session that ended under the command reports
// SessionEndedExitCode.
func FollowSessionCommand(ctx context.Context, request SessionCommandRequest, stdout, stderr io.Writer) (stream.ExecStreamOutcome, error) {
	command, err := findSessionCommand(request)
	if err != nil {
		return stream.ExecStreamOutcome{}, err
	}
	exitCode, err := command.follow(ctx, stdout, stderr)
	if err != nil {
		return stream.ExecStreamOutcome{}, err
	}
	return stream.ExecStreamOutcome{ExitCode: exitCode}, nil
}

func findSessionCommand(request SessionCommandRequest) (*sessionCommand, error) {
	session, err := sessions.get(request.SessionRequest)
	if err != nil {
		return nil, err
	}
	return session.command(request.CmdId)
}

// SessionIdleTimeout is the configured idle timeout, or the default.
func SessionIdleTimeout() time.Duration {
	if config != nil {
		if seconds, err := config.TryGetInt(SessionConfigKeyIdleTimeout); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return sessionDefaultIdleTimeout
}

// RunSessionJanitor closes sessions that have had no running command for
// the idle timeout and forgets sessions that ended that long ago, until ctx
// is done. Started once at operator start.
func RunSessionJanitor(ctx context.Context) {
	ticker := time.NewTicker(sessionJanitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			for _, session := range sessions.idleSince(now.Add(-SessionIdleTimeout())) {
				serviceLogger.Info("closing idle session", "namespace", session.namespace, "pod", session.pod, "sessionId", session.id)
				session.close()
				sessions.remove(session)
			}
		}
	}
}

// idleSince lists sessions without a running command (ended ones included)
// and not used since the given time.
func (r *sessionRegistry) idleSince(cutoff time.Time) []*podSession {
	r.mu.Lock()
	defer r.mu.Unlock()
	var idle []*podSession
	for _, session := range r.sessions {
		if session.stdin == nil {
			continue
		}
		attached := false
		if session.pty != nil {
			session.pty.mu.Lock()
			attached = len(session.pty.attachments) > 0
			session.pty.mu.Unlock()
		}
		session.mu.Lock()
		if session.current == nil && !attached && session.lastUsed.Before(cutoff) {
			idle = append(idle, session)
		}
		session.mu.Unlock()
	}
	return idle
}

/***********************************************************************************************************************
 * one session
 **********************************************************************************************************************/

type podSession struct {
	key       string
	id        string
	namespace string
	pod       string
	container string
	owner     string
	createdAt time.Time
	outputCap int

	stdin  io.WriteCloser
	cancel context.CancelFunc
	// done closes when the shell has ended
	done chan struct{}

	mu       sync.Mutex
	lastUsed time.Time
	commands []*sessionCommand
	byId     map[string]*sessionCommand
	// current is the command the shell is running; stdout lines between its
	// markers and stderr meanwhile belong to it
	current *sessionCommand
	ended   bool
	// partial is the start of a stdout line whose end has not arrived yet
	partial []byte

	// tty sessions only
	tty bool
	pty *ptyState
}

func startSession(request SessionCreateRequest, key string) (*podSession, error) {
	plan, err := sessionTargetFn(context.Background(), request)
	if err != nil {
		return nil, err
	}
	if request.Tty {
		return startTerminalSession(request, key, plan)
	}
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
		createdAt: now,
		lastUsed:  now,
		outputCap: plan.maxOutput,
		stdin:     stdinWriter,
		cancel:    cancel,
		done:      make(chan struct{}),
		byId:      map[string]*sessionCommand{},
	}
	go func() {
		defer close(session.done)
		exitCode, err := sessionShellFn(ctx, plan, request.Namespace, request.Pod, stdinReader, &sessionStdout{session: session}, &sessionStderr{session: session})
		if err != nil {
			if ctx.Err() == nil {
				serviceLogger.Info("session shell ended", "namespace", request.Namespace, "pod", request.Pod, "sessionId", request.SessionId, "error", err)
			}
			exitCode = SessionEndedExitCode
		}
		session.end(exitCode)
	}()
	// A debug container sees the target's filesystem under /proc/1/root; start there.
	if plan.cwd != "" {
		if _, err := io.WriteString(stdinWriter, "cd -- "+shellQuote(plan.cwd)+"\n"); err != nil {
			session.close()
			return nil, fmt.Errorf("session: %w", err)
		}
	}
	return session, nil
}

func (s *podSession) info() SessionInfo {
	attached := 0
	if s.pty != nil {
		s.pty.mu.Lock()
		attached = len(s.pty.attachments)
		s.pty.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	commands := make([]SessionCommandInfo, 0, len(s.commands))
	for _, command := range s.commands {
		commands = append(commands, command.info())
	}
	return SessionInfo{
		SessionId:  s.id,
		Namespace:  s.namespace,
		PodName:    s.pod,
		Container:  s.container,
		Tty:        s.tty,
		Attached:   attached,
		CreatedAt:  s.createdAt,
		LastUsedAt: s.lastUsed,
		Commands:   commands,
	}
}

func (s *podSession) command(id string) (*sessionCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	command, ok := s.byId[id]
	if !ok {
		return nil, ErrSessionCommandNotFound
	}
	return command, nil
}

// exec writes the command to the shell between its markers. The command
// travels base64-encoded inside one line, so any quoting, newline or
// unfinished construct in it is confined to the eval and cannot leave the
// shell waiting for more input with the end marker swallowed. `command eval`
// rather than plain eval: a syntax error in a special builtin ends a
// non-interactive POSIX shell (dash, busybox ash), `command` takes that
// special property away; `exit` still ends it.
func (s *podSession) exec(commandLine string) (*sessionCommand, error) {
	if s.tty {
		return nil, ErrSessionIsTerminal
	}
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return nil, ErrSessionEnded
	}
	if s.current != nil {
		s.mu.Unlock()
		return nil, ErrSessionBusy
	}
	command := newSessionCommand(utils.NanoIdSmallLowerCase(), commandLine, s.outputCap)
	s.commands = append(s.commands, command)
	s.byId[command.id] = command
	s.current = command
	s.lastUsed = time.Now()
	s.mu.Unlock()

	encoded := base64.StdEncoding.EncodeToString([]byte(commandLine))
	line := fmt.Sprintf(
		"printf '%s%s%s\\n'; command eval \"$(printf %%s '%s' | base64 -d)\"; printf '%s%s_%%d%s\\n' \"$?\"\n",
		sessionMarkerBegin, command.id, sessionMarkerTail,
		encoded,
		sessionMarkerEnd, command.id, sessionMarkerTail,
	)
	if _, err := io.WriteString(s.stdin, line); err != nil {
		s.finish(command, SessionEndedExitCode)
		return nil, fmt.Errorf("%w: %v", ErrSessionEnded, err)
	}
	return command, nil
}

func (s *podSession) input(cmdId, data string) error {
	s.mu.Lock()
	current := s.current
	if current == nil {
		s.mu.Unlock()
		return ErrSessionIdle
	}
	if current.id != cmdId {
		s.mu.Unlock()
		return ErrSessionCommandNotFound
	}
	s.lastUsed = time.Now()
	s.mu.Unlock()
	// Until the begin marker is out the shell may not have read the command
	// line yet, and dash or ash read stdin in blocks: input sent now would
	// land in the shell's own buffer and be run as commands instead of
	// reaching the command.
	select {
	case <-current.running:
	case <-s.done:
		return ErrSessionEnded
	}
	if current.info().ExitCode != nil {
		return ErrSessionIdle
	}
	_, err := io.WriteString(s.stdin, data)
	return err
}

// close ends the session on request: the shell loses its stdin and its exec
// stream. Removal from the registry happens in end, once the shell is gone.
func (s *podSession) close() {
	_ = s.stdin.Close()
	s.cancel()
}

// end is called once when the shell has ended, for whatever reason. A
// command still running gets the shell's exit status: that is what an
// `exit 3` in it produced, and SessionEndedExitCode when the session was
// closed from outside. The session stays known until the janitor sweeps it,
// so a client can still fetch the results of its commands.
func (s *podSession) end(exitCode int) {
	s.mu.Lock()
	s.ended = true
	s.lastUsed = time.Now()
	current := s.current
	s.current = nil
	s.mu.Unlock()
	if current != nil {
		current.finish(exitCode)
	}
	if s.pty != nil {
		s.endPTY()
	}
	_ = s.stdin.Close()
	s.cancel()
}

func (s *podSession) finish(command *sessionCommand, exitCode int) {
	s.mu.Lock()
	if s.current == command {
		s.current = nil
	}
	s.lastUsed = time.Now()
	s.mu.Unlock()
	command.finish(exitCode)
}

// feedStdout splits the shell's stdout into lines. Marker lines steer which
// command the output belongs to; every other line goes to the running
// command. The end marker is printed right after the command's output, so
// it may share a line with output that ended without a newline. An
// unfinished line is held back only while it could still turn out to hold a
// marker, so a prompt without a newline (`read -p`) reaches the client right
// away.
func (s *podSession) feedStdout(p []byte) {
	s.mu.Lock()
	data := append(s.partial, p...)
	s.partial = nil
	s.mu.Unlock()

	for {
		newline := bytes.IndexByte(data, '\n')
		if newline < 0 {
			break
		}
		s.handleStdoutLine(data[:newline], true)
		data = data[newline+1:]
	}
	if len(data) == 0 {
		return
	}
	// hold back from the first byte that could begin a marker
	hold := len(data)
	if at := bytes.Index(data, sessionMarkerPrefix); at >= 0 {
		hold = at
	} else {
		for k := min(len(data), len(sessionMarkerPrefix)-1); k > 0; k-- {
			if bytes.HasSuffix(data, sessionMarkerPrefix[:k]) {
				hold = len(data) - k
				break
			}
		}
	}
	if hold > 0 {
		s.handleStdoutLine(data[:hold], false)
	}
	if hold < len(data) {
		s.mu.Lock()
		s.partial = append([]byte(nil), data[hold:]...)
		s.mu.Unlock()
	}
}

// sessionMarkerPrefix is what both markers start with.
var sessionMarkerPrefix = []byte(sessionMarkerBegin[:5])

func (s *podSession) handleStdoutLine(line []byte, newline bool) {
	text := string(line)
	if strings.HasPrefix(text, sessionMarkerBegin) && strings.HasSuffix(text, sessionMarkerTail) {
		// the command's own output starts after this line; whatever the shell
		// printed before it (nothing, normally) is not the command's
		s.mu.Lock()
		command, ok := s.byId[strings.TrimSuffix(strings.TrimPrefix(text, sessionMarkerBegin), sessionMarkerTail)]
		s.mu.Unlock()
		if ok {
			command.markRunning()
		}
		return
	}
	if at := strings.Index(text, sessionMarkerEnd); at >= 0 && strings.HasSuffix(text, sessionMarkerTail) {
		body := strings.TrimSuffix(text[at+len(sessionMarkerEnd):], sessionMarkerTail)
		if split := strings.LastIndex(body, "_"); split > 0 {
			if exitCode, err := strconv.Atoi(body[split+1:]); err == nil {
				// output that ended without a newline shares the line with the marker
				if at > 0 {
					s.appendToCurrent([]byte(text[:at]))
				}
				s.mu.Lock()
				command, ok := s.byId[body[:split]]
				s.mu.Unlock()
				if ok {
					s.finish(command, exitCode)
				}
				return
			}
		}
	}
	if newline {
		s.appendToCurrent(append(append([]byte(nil), line...), '\n'))
	} else {
		s.appendToCurrent(append([]byte(nil), line...))
	}
}

func (s *podSession) appendToCurrent(data []byte) {
	s.mu.Lock()
	current := s.current
	s.mu.Unlock()
	if current != nil {
		current.append(stream.ExecStreamTagStdout, data)
	}
}

func (s *podSession) feedStderr(p []byte) {
	s.mu.Lock()
	current := s.current
	s.mu.Unlock()
	if current != nil {
		current.append(stream.ExecStreamTagStderr, p)
	}
}

// sessionStdout and sessionStderr are the writers the shell's exec stream
// gets; they copy, because the stream reuses its buffer after Write.
type sessionStdout struct{ session *podSession }

func (w *sessionStdout) Write(p []byte) (int, error) {
	w.session.feedStdout(append([]byte(nil), p...))
	return len(p), nil
}

type sessionStderr struct{ session *podSession }

func (w *sessionStderr) Write(p []byte) (int, error) {
	w.session.feedStderr(append([]byte(nil), p...))
	return len(p), nil
}

// shellQuote single-quotes a value for a POSIX shell.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

/***********************************************************************************************************************
 * one command
 **********************************************************************************************************************/

type sessionOutputChunk struct {
	tag  byte
	data []byte
}

type sessionCommand struct {
	id        string
	command   string
	startedAt time.Time
	limit     int

	mu         sync.Mutex
	chunks     []sessionOutputChunk
	size       int
	truncated  bool
	exitCode   *int
	finishedAt *time.Time
	// changed closes whenever output arrives or the command ends, and is
	// replaced right away; a waiter takes the current one under the lock
	changed chan struct{}
	// running closes once the shell has started the command (its begin
	// marker is out) or the command has ended without
	running chan struct{}
}

func newSessionCommand(id, command string, limit int) *sessionCommand {
	return &sessionCommand{id: id, command: command, startedAt: time.Now(), limit: limit, changed: make(chan struct{}), running: make(chan struct{})}
}

func (c *sessionCommand) markRunning() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.markRunningLocked()
}

func (c *sessionCommand) markRunningLocked() {
	select {
	case <-c.running:
	default:
		close(c.running)
	}
}

func (c *sessionCommand) notifyLocked() {
	close(c.changed)
	c.changed = make(chan struct{})
}

// append keeps the first limit bytes of output and drops the rest, like the
// exec request's buffers, so the command runs to its end regardless.
func (c *sessionCommand) append(tag byte, data []byte) {
	if len(data) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.exitCode != nil {
		return
	}
	if c.limit > 0 {
		remaining := c.limit - c.size
		if remaining <= 0 {
			c.truncated = true
			return
		}
		if len(data) > remaining {
			c.truncated = true
			data = data[:remaining]
		}
	}
	c.chunks = append(c.chunks, sessionOutputChunk{tag: tag, data: data})
	c.size += len(data)
	c.notifyLocked()
}

func (c *sessionCommand) finish(exitCode int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.exitCode != nil {
		return
	}
	now := time.Now()
	c.exitCode = &exitCode
	c.finishedAt = &now
	c.markRunningLocked()
	c.notifyLocked()
}

// snapshot hands out the chunks from index from on, whether the command has
// ended, and the channel that closes on the next change.
func (c *sessionCommand) snapshot(from int) ([]sessionOutputChunk, bool, <-chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var chunks []sessionOutputChunk
	if from < len(c.chunks) {
		chunks = append(chunks, c.chunks[from:]...)
	}
	return chunks, c.exitCode != nil, c.changed
}

func (c *sessionCommand) info() SessionCommandInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return SessionCommandInfo{Id: c.id, Command: c.command, ExitCode: c.exitCode, StartedAt: c.startedAt, FinishedAt: c.finishedAt}
}

func (c *sessionCommand) logs() SessionLogsResponse {
	c.mu.Lock()
	defer c.mu.Unlock()
	var output, stdout, stderr bytes.Buffer
	for _, chunk := range c.chunks {
		output.Write(chunk.data)
		if chunk.tag == stream.ExecStreamTagStderr {
			stderr.Write(chunk.data)
		} else {
			stdout.Write(chunk.data)
		}
	}
	return SessionLogsResponse{Output: output.String(), Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: c.exitCode, Truncated: c.truncated}
}

// waitDone returns when the command has ended or ctx is done.
func (c *sessionCommand) waitDone(ctx context.Context) {
	for {
		_, done, changed := c.snapshot(0)
		if done {
			return
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return
		}
	}
}

// follow replays the output so far and then relays new output as it comes,
// until the command ends (its exit code is returned) or ctx is done.
func (c *sessionCommand) follow(ctx context.Context, stdout, stderr io.Writer) (int, error) {
	from := 0
	for {
		chunks, done, changed := c.snapshot(from)
		for _, chunk := range chunks {
			writer := stdout
			if chunk.tag == stream.ExecStreamTagStderr {
				writer = stderr
			}
			if _, err := writer.Write(chunk.data); err != nil {
				return 0, err
			}
		}
		from += len(chunks)
		if done {
			c.mu.Lock()
			exitCode := *c.exitCode
			c.mu.Unlock()
			return exitCode, nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}
