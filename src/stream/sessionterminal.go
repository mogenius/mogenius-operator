package stream

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// TerminalSession is a terminal session of the services layer as the stream
// needs it (MOG-4759): what to replay, where live output comes from, where
// input goes, and how to let go without ending it. An interface, because
// the services layer imports this package.
type TerminalSession interface {
	// Replay is the scrollback at the time of attaching.
	Replay() []byte
	// Output delivers live output; it closes when the session ends or this
	// attachment fell behind.
	Output() <-chan []byte
	// Done closes when the session's shell has ended.
	Done() <-chan struct{}
	// Ended tells a closed Output apart: the session ended, or this
	// attachment fell behind and the client should re-attach.
	Ended() bool
	Input(data []byte) error
	Resize(cols, rows uint16)
	// Detach lets go of the session; the shell runs on.
	Detach()
}

// how long the close handshake may take before the socket is dropped
const terminalCloseHandshakeTimeout = 5 * time.Second

// ErrNoShellAvailable marks an attach that failed because the image ships no
// shell and the client did not ask for the debug container; the terminal
// gets the same NO_SHELL_AVAILABLE signal as a plain shell, and offers it.
var ErrNoShellAvailable = errors.New("NO_SHELL_AVAILABLE")

// AttachedTerminalConnection serves a browser terminal (an `exec-sh` stream)
// from a terminal session instead of a shell of its own: the scrollback is
// replayed, then output and input flow live, and when the socket closes the
// session is only detached. Frames are those of TerminalStreamConnection, so
// the terminal in the browser needs no change. attachErr, when set, is
// shown in the terminal instead, and the socket is closed.
func AttachedTerminalConnection(
	wsConnectionRequest WsConnectionRequest,
	namespace string,
	controller string,
	podName string,
	container string,
	session TerminalSession,
	attachErr error,
) {
	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: GatewayPath}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readMessages, conn, connWriteLock, _, err := GenerateWsConnection("exec-sh", namespace, controller, podName, container, websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil || conn == nil {
		if session != nil {
			session.Detach()
		}
		streamLogger.Error("[AttachedTerminalConnection] unable to connect to the stream gateway", "channelId", wsConnectionRequest.ChannelId, "error", err)
		return
	}
	// GenerateWsConnection sets a 30 minute read deadline for the ack; a
	// terminal has no fixed length
	_ = conn.SetReadDeadline(time.Time{})

	send := func(messageType int, data []byte) error {
		connWriteLock.Lock()
		defer connWriteLock.Unlock()
		return conn.WriteMessage(messageType, data)
	}
	closeSocket := func() {
		_ = send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER"))
		// let the gateway answer the close so the last frames are not lost to a RST
		timer := time.NewTimer(terminalCloseHandshakeTimeout)
		defer timer.Stop()
		for {
			select {
			case _, ok := <-*readMessages:
				if !ok {
					_ = conn.Close()
					return
				}
			case <-timer.C:
				_ = conn.Close()
				return
			}
		}
	}

	if attachErr != nil {
		if errors.Is(attachErr, ErrNoShellAvailable) {
			// same relay path as a plain shell's signal: the browser closes
			// with reason NO_SHELL_AVAILABLE and offers the debug container
			_ = send(websocket.TextMessage, []byte("NO_SHELL_AVAILABLE"))
		} else {
			_ = send(websocket.BinaryMessage, []byte("\r\n"+attachErr.Error()+"\r\n"))
		}
		closeSocket()
		return
	}
	defer session.Detach()

	clearScreen(conn, connWriteLock)
	if replay := session.Replay(); len(replay) > 0 {
		if err := send(websocket.BinaryMessage, replay); err != nil {
			return
		}
	}

	for {
		select {
		case data, ok := <-session.Output():
			if !ok {
				if !session.Ended() {
					// fell behind: the browser terminal answers CLOSE_ABNORMAL
					// with a reconnect, which re-attaches with the scrollback
					_ = send(websocket.TextMessage, []byte("CLOSE_ABNORMAL"))
				}
				closeSocket()
				return
			}
			if err := send(websocket.BinaryMessage, data); err != nil {
				return
			}
		case <-session.Done():
			closeSocket()
			return
		case msg, ok := <-*readMessages:
			if !ok || msg.Err != nil {
				// the client is gone: detach only (deferred)
				return
			}
			text := string(msg.Data)
			if text == "PEER_IS_READY" || text == "BROWSER_PING" {
				continue
			}
			if after, found := strings.CutPrefix(text, "\x04"); found && strings.Contains(after, "\"cols\":") {
				var size CmdWindowSize
				if json.Unmarshal([]byte(after), &size) == nil {
					session.Resize(size.Cols, size.Rows)
					continue
				}
			}
			if err := session.Input(msg.Data); err != nil {
				return
			}
		}
	}
}
