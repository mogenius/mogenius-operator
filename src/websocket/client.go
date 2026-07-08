package websocket

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"mogenius-operator/src/assert"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	gorillaWebsocket "github.com/gorilla/websocket"
	"mogenius-operator/src/metrics"
)

// writeDeadline bounds how long a single websocket write may block.
// Without this a slow/stuck peer wedges the entire write thread.
const writeDeadline = 10 * time.Second

// livenessTimeout bounds how long a connection that is actively read may go
// without any inbound frame or pong before the watchdog forces a reconnect.
// Generous on purpose: pongs are only processed while a read is pending, so
// message-processing backpressure (all workers busy, no read in flight) must
// not be mistaken for a dead peer.
const livenessTimeout = 90 * time.Second

// reconnectBackoff returns an exponentially increasing wait time with
// +/-20% jitter, capped at 30s. Avoids hammering the platform API
// during outages and prevents synchronized reconnect storms across
// multiple operator replicas.
func reconnectBackoff(attempt int) time.Duration {
	const base = 500 * time.Millisecond
	const cap = 30 * time.Second
	shift := max(attempt-1, 0)
	if shift > 6 { // 500ms << 6 = 32s, clamp before overflow risk
		shift = 6
	}
	d := min(base<<shift, cap)
	jitter := 1.0 + (rand.Float64()*0.4 - 0.2)
	return time.Duration(float64(d) * jitter)
}

// #############################
// # +-----------------------+ #
// # | Interface Declaration | #
// # +-----------------------+ #
// #############################

type WebsocketClient interface {
	// connect the internal websocket connection and start worker threads
	Connect() error

	// close the internal websocket connection but keep the WebsocketClient around
	Disconnect() error

	// close the WebsocketClient unrecoverably
	Terminate()

	// check if the WebsocketClient is done
	IsTerminated() bool

	SetUrl(url url.URL) error
	GetUrl() (url.URL, error)
	SetHeader(header http.Header) error
	GetHeader() (http.Header, error)

	WriteJSON(data any) error
	ReadJSON(buf any) error

	// WriteRaw queues an already-marshaled JSON text frame for sending. The
	// marshaling happens in the caller's goroutine instead of on the single
	// write thread, so concurrent callers no longer serialize on encoding.
	WriteRaw(data []byte) error

	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
}

// ################################
// # +--------------------------+ #
// # | Interface Implementation | #
// # +--------------------------+ #
// ################################

func (self *websocketClient) SetUrl(url url.URL) error {
	select {
	case <-self.ctx.Done():
		return fmt.Errorf("WebsocketClient is terminated")
	case self.apiSetUrlTx <- url:
		select {
		case <-self.ctx.Done():
			return fmt.Errorf("WebsocketClient is terminated")
		case err := <-self.apiSetUrlRx:
			return err
		}
	}
}

func (self *websocketClient) GetUrl() (url.URL, error) {
	select {
	case <-self.ctx.Done():
		return url.URL{}, fmt.Errorf("WebsocketClient is terminated")
	case self.apiGetUrlTx <- struct{}{}:
		select {
		case <-self.ctx.Done():
			return url.URL{}, fmt.Errorf("WebsocketClient is terminated")
		case url := <-self.apiGetUrlRx:
			return url, nil
		}
	}
}

func (self *websocketClient) SetHeader(header http.Header) error {
	select {
	case <-self.ctx.Done():
		return fmt.Errorf("WebsocketClient is terminated")
	case self.apiSetHeaderTx <- header:
		select {
		case <-self.ctx.Done():
			return fmt.Errorf("WebsocketClient is terminated")
		case err := <-self.apiSetHeaderRx:
			return err
		}
	}
}

func (self *websocketClient) GetHeader() (http.Header, error) {
	select {
	case <-self.ctx.Done():
		return http.Header{}, fmt.Errorf("WebsocketClient is terminated")
	case self.apiGetHeaderTx <- struct{}{}:
		select {
		case <-self.ctx.Done():
			return http.Header{}, fmt.Errorf("WebsocketClient is terminated")
		case header := <-self.apiGetHeaderRx:
			return header, nil
		}
	}
}

func (self *websocketClient) Connect() error {
	select {
	case <-self.ctx.Done():
		return fmt.Errorf("WebsocketClient is terminated")
	case self.apiConnectTx <- struct{}{}:
		select {
		case <-self.ctx.Done():
			return fmt.Errorf("WebsocketClient is terminated")
		case err := <-self.apiConnectRx:
			return err
		}
	}
}

func (self *websocketClient) Disconnect() error {
	select {
	case <-self.ctx.Done():
		return fmt.Errorf("WebsocketClient is terminated")
	case self.apiDisconnectTx <- struct{}{}:
		select {
		case <-self.ctx.Done():
			return fmt.Errorf("WebsocketClient is terminated")
		case err := <-self.apiDisconnectRx:
			return err
		}
	}
}

func (self *websocketClient) Terminate() {
	select {
	case <-self.ctx.Done():
		return
	case self.apiTerminateTx <- struct{}{}:
		select {
		case <-self.ctx.Done():
			return
		case <-self.apiTerminateRx:
			return
		}
	}
}

func (self *websocketClient) IsTerminated() bool {
	return self.terminated.Load()
}

func (self *websocketClient) WriteMessage(messageType int, data []byte) error {
	select {
	case <-self.ctx.Done():
		return fmt.Errorf("WebsocketClient is terminated")
	case self.apiWriteMessageTx <- websocketWriteMessageInput{messageType, data}:
		select {
		case <-self.ctx.Done():
			return fmt.Errorf("WebsocketClient is terminated")
		case err := <-self.apiWriteMessageRx:
			self.apiLogger.Debug("WriteMessage", "messageType", messageType, "data", data, "error", err)
			return err
		}
	}
}

func (self *websocketClient) ReadMessage() (messageType int, p []byte, err error) {
	select {
	case <-self.ctx.Done():
		return 0, []byte{}, fmt.Errorf("WebsocketClient is terminated")
	case self.apiReadMessageTx <- struct{}{}:
		select {
		case <-self.ctx.Done():
			return 0, []byte{}, fmt.Errorf("WebsocketClient is terminated")
		case result := <-self.apiReadMessageRx:
			// string(result.p) copies the whole payload; slog evaluates args
			// eagerly, so only pay for it when debug logging is enabled.
			if self.apiLogger.Enabled(context.Background(), slog.LevelDebug) {
				self.apiLogger.Debug("ReadMessage", "messageType", result.messageType, "data", string(result.p), "error", result.err)
			}
			return result.messageType, result.p, result.err
		}
	}
}

func (self *websocketClient) WriteJSON(data any) error {
	// Marshal in the caller and enqueue the pre-encoded frame: encoding on
	// the single write thread serialized all concurrent WriteJSON senders,
	// and the old queue-full fallback channel could reorder frames.
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("WriteJSON marshal: %w", err)
	}
	return self.WriteRaw(raw)
}

func (self *websocketClient) WriteRaw(data []byte) error {
	// Fast path: try a non-blocking enqueue.
	select {
	case <-self.ctx.Done():
		return fmt.Errorf("WebsocketClient is terminated")
	case self.writeQueue <- data:
		return nil
	default:
		// Queue full: block until a slot frees up. This backpressures the
		// caller instead of dropping or unbounded-buffering frames.
		select {
		case <-self.ctx.Done():
			return fmt.Errorf("WebsocketClient is terminated")
		case self.writeQueue <- data:
			return nil
		}
	}
}

func (self *websocketClient) ReadJSON(buf any) error {
	select {
	case <-self.ctx.Done():
		return fmt.Errorf("WebsocketClient is terminated")
	case self.apiReadJsonTx <- buf:
		select {
		case <-self.ctx.Done():
			return fmt.Errorf("WebsocketClient is terminated")
		case err := <-self.apiReadJsonRx:
			self.apiLogger.Debug("ReadJSON", "result", buf, "error", err)
			return err
		}
	}
}

// ##########################
// # +--------------------+ #
// # | Client Declaration | #
// # +--------------------+ #
// ##########################

type websocketClient struct {
	// logger for the write thread
	readLogger *slog.Logger

	// logger for the read thread
	writeLogger *slog.Logger

	// logger for the runtime thread
	runtimeLogger *slog.Logger

	// logger for calls to the apis outside of all internally managed threads
	apiLogger *slog.Logger

	// initialized to `false` on creation, set to `true` on shutdown
	//
	// if set to true means all internal threads have been stopped and the websocketClient is done
	terminated atomic.Bool

	// label for the websocket_connected prometheus metric; empty disables it
	name string

	// needed for the reconnect method to know if it should attempt reconnecting
	enableReconnecting atomic.Bool

	// debounce multiple reconnect requests at once
	reconnectRequested atomic.Bool

	// unix-nano timestamp of the last inbound activity (successful read or pong)
	lastActivity atomic.Int64

	// set once the first read is served; the liveness watchdog only applies to
	// clients that are actively read, because pongs are only processed during
	// reads (a write-only client like the events client would otherwise be
	// falsely detected as dead)
	hasReader atomic.Bool

	// signal responder for api functions to reject requests when the WebsocketClient has been terminated
	ctx       context.Context
	ctxCancel context.CancelFunc

	// internally managed websocket connection
	connection *gorillaWebsocket.Conn

	// shutdown for the websocket write thread
	writeThreadShutdownTx chan struct{}
	writeThreadShutdownRx chan struct{}

	// shutdown for the websocket read thread
	readThreadShutdownTx chan struct{}
	readThreadShutdownRx chan struct{}

	// internal: self.sendCloseMessage()
	internalSendCloseMessageTx chan struct{}
	internalSendCloseMessageRx chan error

	// api: self.SetUrl()
	apiSetUrlTx chan url.URL
	apiSetUrlRx chan error

	// api: self.GetUrl()
	apiGetUrlTx chan struct{}
	apiGetUrlRx chan url.URL

	// api: self.SetHeader()
	apiSetHeaderTx chan http.Header
	apiSetHeaderRx chan error

	// api: self.GetHeader()
	apiGetHeaderTx chan struct{}
	apiGetHeaderRx chan http.Header

	// api: self.Connect()
	apiConnectTx chan struct{}
	apiConnectRx chan error

	// api: self.Disconnect()
	apiDisconnectTx chan struct{}
	apiDisconnectRx chan error

	// api: self.WriteMessage()
	apiWriteMessageTx chan websocketWriteMessageInput
	apiWriteMessageRx chan error

	// api: self.ReadMessage()
	apiReadMessageTx chan struct{}
	apiReadMessageRx chan websocketReadMessageOutput

	// api: self.ReadJson()
	apiReadJsonTx chan any
	apiReadJsonRx chan error

	// api: self.Terminate()
	apiTerminateTx chan struct{}
	apiTerminateRx chan struct{}

	// async write queue for non-blocking writes
	writeQueue     chan any
	writeQueueSize int
}

type websocketWriteMessageInput struct {
	messageType int
	data        []byte
}

type websocketReadMessageOutput struct {
	messageType int
	p           []byte
	err         error
}

func NewWebsocketClient(logger *slog.Logger, name string) WebsocketClient {
	self := &websocketClient{}

	self.readLogger = logger.With("scope", "read")
	self.writeLogger = logger.With("scope", "write")
	self.runtimeLogger = logger.With("scope", "runtime")
	self.apiLogger = logger.With("scope", "api")

	self.connection = nil
	self.name = name
	metrics.SetWebsocketConnected(name, false) // initialize series to 0

	self.terminated = atomic.Bool{}
	self.terminated.Store(false)

	self.reconnectRequested = atomic.Bool{}
	self.reconnectRequested.Store(false)

	self.lastActivity = atomic.Int64{}
	self.lastActivity.Store(0)

	self.hasReader = atomic.Bool{}
	self.hasReader.Store(false)

	self.enableReconnecting = atomic.Bool{}
	self.enableReconnecting.Store(false)

	self.ctx, self.ctxCancel = context.WithCancel(context.Background())

	self.writeThreadShutdownTx = make(chan struct{})
	self.writeThreadShutdownRx = make(chan struct{})

	self.readThreadShutdownTx = make(chan struct{})
	self.readThreadShutdownRx = make(chan struct{})

	self.internalSendCloseMessageTx = make(chan struct{})
	self.internalSendCloseMessageRx = make(chan error)

	self.apiSetUrlTx = make(chan url.URL)
	self.apiSetUrlRx = make(chan error)
	self.apiGetUrlTx = make(chan struct{})
	self.apiGetUrlRx = make(chan url.URL)
	self.apiSetHeaderTx = make(chan http.Header)
	self.apiSetHeaderRx = make(chan error)
	self.apiConnectTx = make(chan struct{})
	self.apiConnectRx = make(chan error)
	self.apiDisconnectTx = make(chan struct{})
	self.apiDisconnectRx = make(chan error)
	self.apiWriteMessageTx = make(chan websocketWriteMessageInput)
	self.apiWriteMessageRx = make(chan error)
	self.apiReadMessageTx = make(chan struct{})
	self.apiReadMessageRx = make(chan websocketReadMessageOutput)
	self.apiReadJsonTx = make(chan any)
	self.apiReadJsonRx = make(chan error)
	self.apiTerminateTx = make(chan struct{})
	self.apiTerminateRx = make(chan struct{})

	self.writeQueueSize = 100
	self.writeQueue = make(chan any, self.writeQueueSize)

	go self.startRuntime()

	return self
}

// ###############################
// # +-------------------------+ #
// # | Internal Implementation | #
// # +-------------------------+ #
// ###############################

// generally gorilla websockets support concurrency for read and write operations but not for concurrent reads or writes
//
// our approach is to start three routines: a runtime, a reader and a writer routine
//
// runtime: handle WebsocketClient connect/disconnect, state and orchestrate the writer/reader internally
// writer: handle websocket reads
// reader: handle websocket writes
//
// for details why this is necessary refer to the gorilla websocket documentation:
// https://pkg.go.dev/github.com/gorilla/websocket#hdr-Concurrency
func (self *websocketClient) startRuntime() {
	isRunning := false
	connectionUrl := &url.URL{}
	header := &http.Header{}

	for {
		select {
		case <-self.apiTerminateTx:
			alreadyTerminated := self.terminated.Swap(true)
			self.enableReconnecting.Store(false)
			metrics.SetWebsocketConnected(self.name, false)
			if !alreadyTerminated && isRunning {
				err := self.sendCloseMessage()
				if err != nil {
					self.runtimeLogger.Error("failed to send close message", "error", err)
				}
				// close the connection BEFORE waiting for the worker threads: a
				// read blocked on a half-open connection only returns once the
				// underlying connection is closed, and shutdownWorkerThreads
				// waits for exactly that read to come back to its select
				err = self.connection.Close()
				if err != nil {
					self.runtimeLogger.Error("failed to close internal connection", "error", err)
				}
				self.shutdownWorkerThreads()
				self.connection = nil
			}
			self.apiTerminateRx <- struct{}{}
			self.ctxCancel()
			return
		case newUrl := <-self.apiSetUrlTx:
			connectionUrl = &newUrl
			if isRunning {
				go self.requestReconnect()
			}
			self.apiSetUrlRx <- nil
		case <-self.apiGetUrlTx:
			self.apiGetUrlRx <- *connectionUrl
		case newHeader := <-self.apiSetHeaderTx:
			header = &newHeader
			if isRunning {
				go self.requestReconnect()
			}
			self.apiSetHeaderRx <- nil
		case <-self.apiGetHeaderTx:
			self.apiGetHeaderRx <- *header
		case <-self.apiConnectTx:
			if isRunning {
				self.apiConnectRx <- fmt.Errorf("already connected")
				continue
			}
			var dialer *gorillaWebsocket.Dialer = gorillaWebsocket.DefaultDialer
			// permessage-deflate is intentionally OFF: large payloads are
			// already zlib-compressed at the application layer (see
			// handlePatternRequest), so transport-level deflate would just burn
			// CPU re-compressing incompressible (already-compressed or base64)
			// bytes.
			dialer.EnableCompression = false
			skipTlsVerification := strings.ToLower(os.Getenv("MO_SKIP_TLS_VERIFICATION"))
			if skipTlsVerification == "true" {
				dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			}
			conn, _, err := dialer.Dial(connectionUrl.String(), *header)
			if err != nil {
				self.apiConnectRx <- err
				continue
			}
			self.runtimeLogger.Info("established websocket connection", "url", connectionUrl.String(), "localAddr", conn.LocalAddr())
			self.connection = conn
			self.lastActivity.Store(time.Now().UnixNano())
			go self.startReadThread()
			go self.startWriteThread()
			self.enableReconnecting.Store(true)
			metrics.SetWebsocketConnected(self.name, true)
			isRunning = true
			self.apiConnectRx <- err
		case <-self.apiDisconnectTx:
			if !isRunning {
				self.apiDisconnectRx <- fmt.Errorf("not connected")
				continue
			}
			err := self.sendCloseMessage()
			if err != nil {
				self.runtimeLogger.Error("failed to send close message", "error", err)
			}
			// same ordering as in the terminate case: closing first unblocks a
			// read stuck on a half-open connection so the worker shutdown below
			// cannot deadlock the runtime thread (and with it every reconnect)
			err = self.connection.Close()
			if err != nil {
				self.runtimeLogger.Error("failed to close internal connection", "error", err)
			}
			self.shutdownWorkerThreads()
			self.connection = nil
			self.enableReconnecting.Store(false)
			metrics.SetWebsocketConnected(self.name, false)
			isRunning = false
			self.apiDisconnectRx <- nil
		}
	}
}

// worker thread to run reads on the internal websocket connection
func (self *websocketClient) startReadThread() {
	self.connection.SetCloseHandler(func(code int, text string) error {
		self.readLogger.Debug("CloseHandler: received close message", "code", code, "text", text)
		go self.requestReconnect()
		return nil
	})

	// pongs are answers to the pings sent by the write thread; they are only
	// processed while a read is pending, which is why the liveness watchdog
	// is gated on hasReader
	self.connection.SetPongHandler(func(string) error {
		self.lastActivity.Store(time.Now().UnixNano())
		return nil
	})

	for {
		select {
		case <-self.readThreadShutdownTx:
			self.readThreadShutdownRx <- struct{}{}
			return
		case <-self.apiReadMessageTx:
			assert.Assert(self.connection != nil)
			self.hasReader.Store(true)
			self.readLogger.Debug("ReadMessage")
			messageType, p, err := self.connection.ReadMessage()
			if err == nil {
				self.lastActivity.Store(time.Now().UnixNano())
			}
			// string(p) copies the whole payload; only pay for it when debug
			// logging is enabled.
			if self.readLogger.Enabled(context.Background(), slog.LevelDebug) {
				self.readLogger.Debug("ReadMessage", "type", messageType, "data", string(p), "error", err)
			}
			self.apiReadMessageRx <- websocketReadMessageOutput{messageType, p, err}
			self.healthcheck(err)
		case buf := <-self.apiReadJsonTx:
			assert.Assert(self.connection != nil)
			self.hasReader.Store(true)
			self.readLogger.Debug("ReadJSON")
			err := self.connection.ReadJSON(buf)
			if err == nil {
				self.lastActivity.Store(time.Now().UnixNano())
			}
			self.readLogger.Debug("ReadJSON", "error", err)
			self.apiReadJsonRx <- err
			self.healthcheck(err)
		}
	}
}

// worker thread to run writes on the internal websocket connection
func (self *websocketClient) startWriteThread() {
	pingInterval := 3 * time.Second
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()
	for {
		select {
		case <-self.writeThreadShutdownTx:
			self.writeThreadShutdownRx <- struct{}{}
			return
		case <-pingTicker.C:
			assert.Assert(self.connection != nil)
			_ = self.connection.SetWriteDeadline(time.Now().Add(writeDeadline))
			err := self.connection.WriteMessage(gorillaWebsocket.PingMessage, nil)
			if err != nil {
				self.writeLogger.Debug("failed to send ping", "error", err)
			}
			self.healthcheck(err)
			// liveness watchdog: on a half-open connection pings keep landing
			// in the kernel buffer without error, so successful writes prove
			// nothing - only inbound frames/pongs do. Reads are the only place
			// pongs get processed, hence the hasReader gate (write-only clients
			// like the events client are covered by the write-timeout path in
			// healthcheck instead).
			if self.hasReader.Load() {
				lastActivity := time.Unix(0, self.lastActivity.Load())
				if time.Since(lastActivity) > livenessTimeout {
					self.writeLogger.Warn("no inbound frame or pong within liveness timeout - triggering reconnect", "lastActivity", lastActivity)
					go self.requestReconnect()
				}
			}
		case data := <-self.writeQueue:
			// Process queued writes without blocking the caller. Pre-marshaled
			// frames (from WriteRaw) are written as-is; the write thread does
			// pure socket I/O and no JSON encoding, so it is never the
			// serialization bottleneck for concurrent responses.
			assert.Assert(self.connection != nil)
			self.writeLogger.Debug("write from queue", "remainingInQueue", len(self.writeQueue))
			_ = self.connection.SetWriteDeadline(time.Now().Add(writeDeadline))
			var err error
			if raw, ok := data.([]byte); ok {
				err = self.connection.WriteMessage(gorillaWebsocket.TextMessage, raw)
			} else {
				err = self.connection.WriteJSON(data)
			}
			if err != nil {
				self.writeLogger.Error("write from queue failed", "error", err)
			}
			self.healthcheck(err)
		case val := <-self.apiWriteMessageTx:
			assert.Assert(self.connection != nil)
			self.writeLogger.Debug("WriteMessage", "type", val.messageType, "data", val.data)
			_ = self.connection.SetWriteDeadline(time.Now().Add(writeDeadline))
			err := self.connection.WriteMessage(val.messageType, val.data)
			self.writeLogger.Debug("WriteMessage", "error", err)
			self.apiWriteMessageRx <- err
			self.healthcheck(err)
		case <-self.internalSendCloseMessageTx:
			assert.Assert(self.connection != nil)
			self.writeLogger.Debug("sending close message")
			_ = self.connection.SetWriteDeadline(time.Now().Add(writeDeadline))
			err := self.connection.WriteMessage(
				gorillaWebsocket.CloseMessage,
				gorillaWebsocket.FormatCloseMessage(
					gorillaWebsocket.CloseNormalClosure,
					"",
				),
			)
			if err != nil {
				self.writeLogger.Error("failed to send close message", "error", err)
				self.internalSendCloseMessageRx <- err
				continue
			}
			self.writeLogger.Debug("close message was sent")
			self.internalSendCloseMessageRx <- nil
		}
	}
}

func (self *websocketClient) shutdownWorkerThreads() {
	self.readThreadShutdownTx <- struct{}{}
	self.writeThreadShutdownTx <- struct{}{}
	<-self.readThreadShutdownRx
	<-self.writeThreadShutdownRx
}

func (self *websocketClient) sendCloseMessage() error {
	self.internalSendCloseMessageTx <- struct{}{}
	err := <-self.internalSendCloseMessageRx
	if err != nil {
		return err
	}
	return nil
}

func (self *websocketClient) requestReconnect() {
	select {
	case <-self.ctx.Done():
		return
	default:
		isAlreadyReconnecting := self.reconnectRequested.Swap(true)
		if isAlreadyReconnecting {
			return
		}
		defer self.reconnectRequested.Store(false)
		shouldReconnect := self.enableReconnecting.Load()

		if shouldReconnect {
			self.apiLogger.Warn("Reconnect has been triggered.")
		}
		defer func() {
			if shouldReconnect {
				self.apiLogger.Warn("Reconnect has finished.")
			}
		}()

		err := self.Disconnect()
		if err != nil {
			self.apiLogger.Error("disconnect failed", "error", err)
		}
		if shouldReconnect {
			err = self.Connect()
			if err != nil {
				self.apiLogger.Error("connect failed", "error", err)
				attempts := 0
				for !self.IsTerminated() {
					attempts += 1
					time.Sleep(reconnectBackoff(attempts))
					err = self.Connect()
					if err != nil {
						if attempts%10 == 0 {
							self.apiLogger.Error("connect failed", "error", err)
						}
						continue
					}
					return
				}
			}
		}
	}
}

func (self *websocketClient) healthcheck(err error) {
	if err == nil {
		return
	}
	if self.IsTerminated() {
		return
	}
	select {
	case <-self.ctx.Done():
		return
	default:
		if gorillaWebsocket.IsUnexpectedCloseError(err) {
			self.runtimeLogger.Debug("detected close error - triggering reconnect", "error", err)
			go self.requestReconnect()
			return
		}
		// a write deadline exceeded means the peer stopped draining the
		// connection (e.g. half-open after an LB drop); without this the
		// client would keep the dead connection forever because none of the
		// syscall errors below ever fire on a blackholed connection
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			self.runtimeLogger.Debug("detected timeout error - triggering reconnect", "error", err)
			go self.requestReconnect()
			return
		}
		if errors.Is(err, syscall.ECONNRESET) {
			self.runtimeLogger.Debug("detected connection reset error - triggering reconnect", "error", err)
			go self.requestReconnect()
			return
		}
		if errors.Is(err, syscall.ECONNREFUSED) {
			self.runtimeLogger.Debug("detected connection refused error - triggering reconnect", "error", err)
			go self.requestReconnect()
			return
		}
		if errors.Is(err, syscall.ECONNABORTED) {
			self.runtimeLogger.Debug("detected connection aborted error - triggering reconnect", "error", err)
			go self.requestReconnect()
			return
		}
		if errors.Is(err, syscall.EPIPE) {
			self.runtimeLogger.Debug("detected broken pipe error - triggering reconnect", "error", err)
			go self.requestReconnect()
			return
		}
	}
}
