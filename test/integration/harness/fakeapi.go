package harness

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Datagram mirrors the operator's structs.Datagram for test assertions.
// Using a local copy avoids pulling the full operator package tree into the harness.
type Datagram struct {
	Id      string `json:"id"`
	Pattern string `json:"pattern"`
	Payload any    `json:"payload,omitempty"`
	Zlib    bool   `json:"zlib,omitempty"`
}

// PayloadStatus extracts the "status" field from the operator's response payload.
// The operator wraps all responses as Payload: {"status":"success"|"error","data":...,"message":...}.
func (d Datagram) PayloadStatus() string {
	m, ok := d.Payload.(map[string]any)
	if !ok {
		return ""
	}
	s, _ := m["status"].(string)
	return s
}

// PayloadData extracts the "data" map from the operator's response payload.
func (d Datagram) PayloadData() (map[string]any, bool) {
	m, ok := d.Payload.(map[string]any)
	if !ok {
		return nil, false
	}
	data, ok := m["data"].(map[string]any)
	return data, ok
}

// PayloadMessage extracts the "message" string from the operator's response payload.
func (d Datagram) PayloadMessage() string {
	m, ok := d.Payload.(map[string]any)
	if !ok {
		return ""
	}
	s, _ := m["message"].(string)
	return s
}

// FakeAPIServer is a WebSocket server that stands in for the mogenius platform API.
// The operator connects to it; tests use Send to dispatch commands and receive responses.
type FakeAPIServer struct {
	// URL is the WebSocket URL to configure the operator against (ws://host:port).
	URL string

	server    *httptest.Server
	upgrader  websocket.Upgrader
	mu        sync.Mutex
	conn      *websocket.Conn
	pending   map[string]chan Datagram
	connected chan struct{}
	once      sync.Once
}

// NewFakeAPIServer starts a fake WebSocket server and returns it immediately.
// Call Close() when done (typically via t.Cleanup).
func NewFakeAPIServer() *FakeAPIServer {
	f := &FakeAPIServer{
		pending:   make(map[string]chan Datagram),
		connected: make(chan struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handleWS))
	f.URL = "ws" + f.server.URL[len("http"):]
	return f
}

func (f *FakeAPIServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := f.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()
	f.once.Do(func() { close(f.connected) })

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var d Datagram
		if err := json.Unmarshal(msg, &d); err != nil {
			continue
		}
		if d.Zlib {
			if decompressed, err := zlibDecompressPayload(d.Payload); err == nil {
				d.Payload = decompressed
				d.Zlib = false
			}
		}
		f.mu.Lock()
		ch, ok := f.pending[d.Id]
		f.mu.Unlock()
		if ok {
			ch <- d
		}
	}
}

// zlibDecompressPayload base64-decodes and zlib-decompresses an operator payload.
// When zlib:true the operator JSON-encodes []byte as a base64 string.
func zlibDecompressPayload(payload any) (any, error) {
	b64, ok := payload.(string)
	if !ok {
		return payload, nil
	}
	compressed, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// WaitConnected blocks until the operator establishes its WebSocket connection or ctx expires.
func (f *FakeAPIServer) WaitConnected(ctx context.Context) error {
	select {
	case <-f.connected:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("operator did not connect to fake API server: %w", ctx.Err())
	}
}

// Send dispatches pattern with payload to the connected operator and returns the response.
// It blocks until a matching response arrives or ctx expires.
func (f *FakeAPIServer) Send(ctx context.Context, pattern string, payload any) (Datagram, error) {
	id := fmt.Sprintf("test-%d", time.Now().UnixNano())
	ch := make(chan Datagram, 1)

	f.mu.Lock()
	f.pending[id] = ch
	conn := f.conn
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.pending, id)
		f.mu.Unlock()
	}()

	d := Datagram{Id: id, Pattern: pattern, Payload: payload}
	data, err := json.Marshal(d)
	if err != nil {
		return Datagram{}, fmt.Errorf("marshal datagram: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return Datagram{}, fmt.Errorf("write datagram: %w", err)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return Datagram{}, fmt.Errorf("timeout waiting for %q response: %w", pattern, ctx.Err())
	}
}

// Close shuts down the fake server. Typically called via t.Cleanup.
func (f *FakeAPIServer) Close() {
	f.server.Close()
}
