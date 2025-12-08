package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	gorillaWebsocket "github.com/gorilla/websocket"
)

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
			self.apiLogger.Debug("ReadMessage", "messageType", result.messageType, "data", string(result.p), "error", result.err)
			return result.messageType, result.p, result.err
		}
	}
}

func (self *websocketClient) WriteJSON(data any) error {
	select {
	case <-self.ctx.Done():
		return fmt.Errorf("WebsocketClient is terminated")
	case self.apiWriteJsonTx <- data:
		select {
		case <-self.ctx.Done():
			return fmt.Errorf("WebsocketClient is terminated")
		case err := <-self.apiWriteJsonRx:
			self.apiLogger.Debug("WriteJSON", "data", data, "error", err)
			return err
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

	// needed for the reconnect method to know if it should attempt reconnecting
	enableReconnecting atomic.Bool

	// debounce multiple reconnect requests at once
	reconnectRequested atomic.Bool

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

	// api: self.WriteJson()
	apiWriteJsonTx chan any
	apiWriteJsonRx chan error

	// api: self.ReadJson()
	apiReadJsonTx chan any
	apiReadJsonRx chan error

	// api: self.Terminate()
	apiTerminateTx chan struct{}
	apiTerminateRx chan struct{}
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

func NewWebsocketClient(logger *slog.Logger) WebsocketClient {
	self := &websocketClient{}

	self.readLogger = logger.With("scope", "read")
	self.writeLogger = logger.With("scope", "write")
	self.runtimeLogger = logger.With("scope", "runtime")
	self.apiLogger = logger.With("scope", "api")

	self.connection = nil

	self.terminated = atomic.Bool{}
	self.terminated.Store(false)

	self.reconnectRequested = atomic.Bool{}
	self.reconnectRequested.Store(false)

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
	self.apiWriteJsonTx = make(chan any)
	self.apiWriteJsonRx = make(chan error)
	self.apiReadJsonTx = make(chan any)
	self.apiReadJsonRx = make(chan error)
	self.apiTerminateTx = make(chan struct{})
	self.apiTerminateRx = make(chan struct{})

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
			if !alreadyTerminated && isRunning {
				err := self.sendCloseMessage()
				if err != nil {
					self.runtimeLogger.Error("failed to send close message", "error", err)
				}
				self.shutdownWorkerThreads()
				err = self.connection.Close()
				if err != nil {
					self.runtimeLogger.Error("failed to close internal connection", "error", err)
				}
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
			go self.startReadThread()
			go self.startWriteThread()
			self.enableReconnecting.Store(true)
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
			self.shutdownWorkerThreads()
			err = self.connection.Close()
			if err != nil {
				self.runtimeLogger.Error("failed to close internal connection", "error", err)
			}
			self.connection = nil
			self.enableReconnecting.Store(false)
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

	for {
		select {
		case <-self.readThreadShutdownTx:
			self.readThreadShutdownRx <- struct{}{}
			return
		case <-self.apiReadMessageTx:
			assert.Assert(self.connection != nil)
			self.readLogger.Debug("ReadMessage")
			messageType, p, err := self.connection.ReadMessage()
			self.readLogger.Debug("ReadMessage", "type", messageType, "data", string(p), "error", err)
			self.apiReadMessageRx <- websocketReadMessageOutput{messageType, p, err}
			self.healthcheck(err)
		case buf := <-self.apiReadJsonTx:
			assert.Assert(self.connection != nil)
			self.readLogger.Debug("ReadJSON")
			err := self.connection.ReadJSON(buf)
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
	for {
		select {
		case <-self.writeThreadShutdownTx:
			self.writeThreadShutdownRx <- struct{}{}
			return
		case <-pingTicker.C:
			assert.Assert(self.connection != nil)
			err := self.connection.WriteMessage(gorillaWebsocket.PingMessage, nil)
			if err != nil {
				self.writeLogger.Debug("failed to send ping", "error", err)
			}
			self.healthcheck(err)
		case val := <-self.apiWriteMessageTx:
			assert.Assert(self.connection != nil)
			self.writeLogger.Debug("WriteMessage", "type", val.messageType, "data", val.data)
			err := self.connection.WriteMessage(val.messageType, val.data)
			self.writeLogger.Debug("WriteMessage", "error", err)
			self.apiWriteMessageRx <- err
			self.healthcheck(err)
		case data := <-self.apiWriteJsonTx:
			assert.Assert(self.connection != nil)
			self.writeLogger.Debug("WriteJSON", "data", data)
			err := self.connection.WriteJSON(data)
			self.writeLogger.Debug("WriteJSON", "error", err)
			self.apiWriteJsonRx <- err
			self.healthcheck(err)
		case <-self.internalSendCloseMessageTx:
			assert.Assert(self.connection != nil)
			self.writeLogger.Debug("sending close message")
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
					time.Sleep(500 * time.Millisecond)
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
