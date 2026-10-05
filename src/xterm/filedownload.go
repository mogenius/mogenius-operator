package xterm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// File download stream (FDS): one stream socket per download, opened by the
// operator towards the platform's stream gateway like a terminal or a
// port-forward. The file's bytes go out as binary frames; text frames carry
// the control protocol:
//
//	operator → api:  FDS:END                 every byte was sent
//	                 FDS:ERR:<message>       the exec failed mid-stream
//	api → operator:  FDS:CREDIT:<bytes>      the browser side drained that many bytes
//	                 CLOSE_CONNECTION_FROM_PEER  the browser is gone
//
// The credit window is the flow control the relay lacks: the platform
// forwards frames through Redis Pub/Sub, which buffers for a slow consumer
// until Redis drops it. The operator may have at most fdsInitialWindowBytes
// unacknowledged on the wire; past that, Write blocks, and with it `cat` or
// `tar` inside the container.
const (
	fdsCmdType            = "files-download"
	fdsCreditPrefix       = "FDS:CREDIT:"
	fdsEndFrame           = "FDS:END"
	fdsErrPrefix          = "FDS:ERR:"
	fdsClosedByPeerFrame  = "CLOSE_CONNECTION_FROM_PEER"
	fdsInitialWindowBytes = 8 << 20
	fdsChunkBytes         = 256 << 10
)

// FileDownloadStreamRequest names the stream socket to open: the channel the
// API waits on, and the pod whose file is streamed (for the socket's headers).
type FileDownloadStreamRequest struct {
	WsConnection WsConnectionRequest
	Namespace    string
	Pod          string
	Container    string
}

// FileDownloadStream connects to the stream gateway and runs produce with a
// writer that turns bytes into binary frames under the credit window. It
// returns when the stream is over, whichever side ended it; the handler runs
// it in a goroutine after answering the datagram.
func FileDownloadStream(request FileDownloadStreamRequest, produce func(ctx context.Context, w io.Writer) error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	websocketUrl := url.URL{
		Scheme: request.WsConnection.WebsocketScheme,
		Host:   request.WsConnection.WebsocketHost,
		Path:   "/xterm-stream",
	}
	readMessages, conn, connWriteLock, _, err := GenerateWsConnection(
		fdsCmdType, request.Namespace, "", request.Pod, request.Container,
		websocketUrl, request.WsConnection, ctx, cancel,
	)
	if err != nil || conn == nil {
		xtermLogger.Error("[FileDownloadStream] unable to connect to the stream gateway", "channelId", request.WsConnection.ChannelId, "error", err)
		return
	}
	// GenerateWsConnection sets a 30 minute read deadline for the ack; a
	// download has no fixed length, the API's stall timeout bounds silence.
	_ = conn.SetReadDeadline(time.Time{})

	send := func(messageType int, data []byte) error {
		connWriteLock.Lock()
		defer connWriteLock.Unlock()
		return conn.WriteMessage(messageType, data)
	}
	window := newCreditWindow(ctx, fdsInitialWindowBytes)

	// Control frames from the API: credits grow the window, a close ends
	// everything. The channel closes when the socket does (oncloseWs).
	go func() {
		defer cancel()
		for message := range *readMessages {
			if message.Err != nil {
				return
			}
			if message.MessageType != websocket.TextMessage {
				continue
			}
			text := string(message.Data)
			switch {
			case strings.HasPrefix(text, fdsCreditPrefix):
				credit, parseErr := strconv.ParseInt(strings.TrimPrefix(text, fdsCreditPrefix), 10, 64)
				if parseErr != nil || credit <= 0 {
					xtermLogger.Warn("[FileDownloadStream] ignoring malformed credit frame", "frame", text)
					continue
				}
				window.add(credit)
			case text == fdsClosedByPeerFrame:
				xtermLogger.Debug("[FileDownloadStream] peer closed the download", "channelId", request.WsConnection.ChannelId)
				return
			}
		}
	}()

	writer := &creditWriter{window: window, send: send}
	produceErr := produce(ctx, writer)

	closeReason := ""
	switch {
	case ctx.Err() != nil:
		// the peer went away or the socket died: nothing left to tell
	case produceErr != nil:
		xtermLogger.Error("[FileDownloadStream] exec failed", "channelId", request.WsConnection.ChannelId, "error", produceErr)
		closeReason = produceErr.Error()
		_ = send(websocket.TextMessage, []byte(fdsErrPrefix+closeReason))
	default:
		_ = send(websocket.TextMessage, []byte(fdsEndFrame))
	}
	// 1000 either way: the API already knows from END/ERR how it went, and a
	// close reason is capped at 123 bytes, so the message rides in the frame.
	if len(closeReason) > 100 {
		closeReason = closeReason[:100]
	}
	_ = send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, closeReason))
	_ = conn.Close()
}

// creditWindow is the number of bytes the operator may still send before the
// API has to acknowledge. take blocks until it can reserve the chunk or the
// context ends.
type creditWindow struct {
	mu     sync.Mutex
	cond   *sync.Cond
	credit int64
	ctx    context.Context
}

func newCreditWindow(ctx context.Context, initial int64) *creditWindow {
	w := &creditWindow{credit: initial, ctx: ctx}
	w.cond = sync.NewCond(&w.mu)
	// a cond has no context: wake every waiter once so they see ctx.Err()
	go func() {
		<-ctx.Done()
		w.mu.Lock()
		w.cond.Broadcast()
		w.mu.Unlock()
	}()
	return w
}

func (w *creditWindow) add(n int64) {
	w.mu.Lock()
	w.credit += n
	w.cond.Broadcast()
	w.mu.Unlock()
}

func (w *creditWindow) take(n int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for w.credit < n {
		if err := w.ctx.Err(); err != nil {
			return err
		}
		w.cond.Wait()
	}
	if err := w.ctx.Err(); err != nil {
		return err
	}
	w.credit -= n
	return nil
}

// creditWriter is the io.Writer the exec streams into: it splits the output
// into frames of at most fdsChunkBytes and reserves each one in the window
// before sending it. A failed send or a cancelled context makes Write fail,
// which ends the exec.
type creditWriter struct {
	window *creditWindow
	send   func(messageType int, data []byte) error
}

var errDownloadAborted = errors.New("download aborted")

func (w *creditWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		chunk := p
		if len(chunk) > fdsChunkBytes {
			chunk = p[:fdsChunkBytes]
		}
		if err := w.window.take(int64(len(chunk))); err != nil {
			return written, fmt.Errorf("%w: %v", errDownloadAborted, err)
		}
		// the exec reuses its buffer after Write returns; the frame must own its bytes
		frame := make([]byte, len(chunk))
		copy(frame, chunk)
		if err := w.send(websocket.BinaryMessage, frame); err != nil {
			return written, fmt.Errorf("%w: %v", errDownloadAborted, err)
		}
		written += len(chunk)
		p = p[len(chunk):]
	}
	return written, nil
}
