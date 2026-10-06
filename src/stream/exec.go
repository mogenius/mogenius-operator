package stream

import (
	"context"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

// Exec stream (MOG-4691): one command in a pod's container, run without a TTY
// and relayed live. Opened by the operator towards the stream gateway like a
// terminal; the client reads binary frames (`binary=1`). Output goes out as
// binary frames behind a one-byte stream tag, control messages as text:
//
//	operator → client:  [0]<bytes>       a chunk of stdout
//	                    [1]<bytes>       a chunk of stderr
//	                    TRUNCATED        a stream hit the output cap; the rest was dropped
//	                    TIMEOUT          the command was stopped at its deadline
//	                    EXIT:<code>      the command ended; the socket closes after this
//	                    ERROR:<message>  the command could not run at all
//	client → operator:  nothing; the command's stdin is /dev/null
//
// A close from the client's side reaches this socket as a close from the
// gateway, which ends the command (see k8sexec.BuildStreamShellCommand).
const (
	execCmdType = "exec"
	// ExecStreamTagStdout and ExecStreamTagStderr lead every output frame.
	ExecStreamTagStdout byte = 0
	ExecStreamTagStderr byte = 1

	execExitPrefix     = "EXIT:"
	execErrorPrefix    = "ERROR:"
	execTruncatedFrame = "TRUNCATED"
	execTimeoutFrame   = "TIMEOUT"
	// how long the final close handshake may take before the socket is dropped
	execCloseHandshakeTimeout = 5 * time.Second
)

// ExecStreamRequest names the stream socket to open: the channel the API
// waits on, and the pod the command runs in (for the socket's headers).
type ExecStreamRequest struct {
	WsConnection WsConnectionRequest
	Namespace    string
	Pod          string
	Container    string
}

// ExecStreamOutcome is what the client learns once the command has ended.
type ExecStreamOutcome struct {
	ExitCode int
	// TimedOut: the command was stopped at its deadline; ExitCode is then
	// what the kill left behind.
	TimedOut bool
}

// ExecStream connects to the stream gateway and runs `run` with a stdin pipe
// the command's watchdog watches and writers that frame stdout and stderr,
// each capped at maxOutputBytes (0 for no cap). It returns when the stream is
// over, whichever side ended it; the handler runs it in a goroutine after
// answering the datagram.
func ExecStream(
	request ExecStreamRequest,
	maxOutputBytes int,
	run func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) (ExecStreamOutcome, error),
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	websocketUrl := url.URL{
		Scheme: request.WsConnection.WebsocketScheme,
		Host:   request.WsConnection.WebsocketHost,
		Path:   GatewayPath,
	}
	readMessages, conn, connWriteLock, _, err := GenerateWsConnection(
		execCmdType, request.Namespace, "", request.Pod, request.Container,
		websocketUrl, request.WsConnection, ctx, cancel,
	)
	if err != nil || conn == nil {
		streamLogger.Error("[ExecStream] unable to connect to the stream gateway", "channelId", request.WsConnection.ChannelId, "error", err)
		return
	}
	// GenerateWsConnection sets a 30 minute read deadline for the ack; the
	// command's own timeout bounds the run.
	_ = conn.SetReadDeadline(time.Time{})

	send := func(messageType int, data []byte) error {
		connWriteLock.Lock()
		defer connWriteLock.Unlock()
		return conn.WriteMessage(messageType, data)
	}

	// The command's stdin: nothing is ever written to it, its closing is what
	// stops the command. Closed when the socket closes (the client is gone)
	// and again, harmlessly, once the command has ended on its own.
	stdinReader, stdinWriter := io.Pipe()
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		// the channel closes when the socket does (oncloseWs); nothing flows in
		for range *readMessages {
		}
		_ = stdinWriter.Close()
		cancel()
	}()

	stdout := &taggedWriter{tag: ExecStreamTagStdout, send: send, limit: maxOutputBytes}
	stderr := &taggedWriter{tag: ExecStreamTagStderr, send: send, limit: maxOutputBytes}
	outcome, runErr := run(ctx, stdinReader, stdout, stderr)
	_ = stdinWriter.Close()

	switch {
	case ctx.Err() != nil:
		// the client went away or the socket died: nothing left to tell
	case runErr != nil:
		streamLogger.Error("[ExecStream] command could not run", "channelId", request.WsConnection.ChannelId, "error", runErr)
		_ = send(websocket.TextMessage, []byte(execErrorPrefix+runErr.Error()))
	default:
		if stdout.truncated || stderr.truncated {
			_ = send(websocket.TextMessage, []byte(execTruncatedFrame))
		}
		if outcome.TimedOut {
			_ = send(websocket.TextMessage, []byte(execTimeoutFrame))
		}
		_ = send(websocket.TextMessage, []byte(execExitPrefix+strconv.Itoa(outcome.ExitCode)))
	}
	// Same close as a terminal: the relay closes the client with 1000 on this
	// reason. Wait for the gateway's answer before tearing the socket down so
	// the last frames are not discarded by a RST (see FileDownloadStream).
	_ = send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER"))
	select {
	case <-readerDone:
	case <-time.After(execCloseHandshakeTimeout):
		streamLogger.Warn("[ExecStream] gateway did not answer the close in time", "channelId", request.WsConnection.ChannelId)
	}
	_ = conn.Close()
}

// taggedWriter frames one output stream: every Write becomes one binary frame
// behind the stream's tag. Past limit bytes it drops the rest while still
// reporting the write as consumed — like k8sexec's cappedBuffer, so the
// command runs to its end regardless of how much it prints. A failed send
// ends the exec.
type taggedWriter struct {
	tag       byte
	send      func(messageType int, data []byte) error
	limit     int
	written   int
	truncated bool
}

func (w *taggedWriter) Write(p []byte) (int, error) {
	chunk := p
	if w.limit > 0 {
		remaining := w.limit - w.written
		if remaining <= 0 {
			if len(p) > 0 {
				w.truncated = true
			}
			return len(p), nil
		}
		if len(chunk) > remaining {
			w.truncated = true
			chunk = chunk[:remaining]
		}
	}
	// the exec reuses its buffer after Write returns; the frame must own its bytes
	frame := make([]byte, 1+len(chunk))
	frame[0] = w.tag
	copy(frame[1:], chunk)
	if err := w.send(websocket.BinaryMessage, frame); err != nil {
		return 0, err
	}
	w.written += len(chunk)
	return len(p), nil
}
