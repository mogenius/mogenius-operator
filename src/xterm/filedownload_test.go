package xterm

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// recordingSend collects frames like a socket would, optionally failing.
type recordingSend struct {
	mu     sync.Mutex
	frames [][]byte
	fail   error
}

func (r *recordingSend) send(messageType int, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail != nil {
		return r.fail
	}
	if messageType != websocket.BinaryMessage {
		return errors.New("data must go out as binary frames")
	}
	r.frames = append(r.frames, data)
	return nil
}

func (r *recordingSend) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, f := range r.frames {
		n += len(f)
	}
	return n
}

func TestCreditWriterSplitsIntoChunksWithinTheWindow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink := &recordingSend{}
	w := &creditWriter{window: newCreditWindow(ctx, 3*fdsChunkBytes), send: sink.send}

	payload := bytes.Repeat([]byte{7}, 2*fdsChunkBytes+10)
	n, err := w.Write(payload)
	if err != nil || n != len(payload) {
		t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(payload))
	}
	if len(sink.frames) != 3 {
		t.Fatalf("frames = %d, want 3", len(sink.frames))
	}
	for i, f := range sink.frames[:2] {
		if len(f) != fdsChunkBytes {
			t.Fatalf("frame %d = %d bytes, want %d", i, len(f), fdsChunkBytes)
		}
	}
	if len(sink.frames[2]) != 10 {
		t.Fatalf("last frame = %d bytes, want 10", len(sink.frames[2]))
	}
	if w.window.credit != fdsChunkBytes-10 {
		t.Fatalf("credit left = %d, want %d", w.window.credit, fdsChunkBytes-10)
	}
}

func TestCreditWriterBlocksUntilCreditArrives(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink := &recordingSend{}
	w := &creditWriter{window: newCreditWindow(ctx, fdsChunkBytes), send: sink.send}

	done := make(chan error, 1)
	go func() {
		_, err := w.Write(bytes.Repeat([]byte{1}, 2*fdsChunkBytes))
		done <- err
	}()

	// one chunk fits the initial window, the second must wait
	deadline := time.Now().Add(2 * time.Second)
	for sink.total() < fdsChunkBytes && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if sink.total() != fdsChunkBytes {
		t.Fatalf("sent %d bytes before any credit, want exactly %d", sink.total(), fdsChunkBytes)
	}
	select {
	case err := <-done:
		t.Fatalf("Write returned %v without credit for the second chunk", err)
	case <-time.After(50 * time.Millisecond):
	}

	w.window.add(fdsChunkBytes)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Write failed after credit: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Write did not finish after credit arrived")
	}
	if sink.total() != 2*fdsChunkBytes {
		t.Fatalf("sent %d bytes, want %d", sink.total(), 2*fdsChunkBytes)
	}
}

func TestCreditWriterFailsWhenTheDownloadIsAborted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sink := &recordingSend{}
	w := &creditWriter{window: newCreditWindow(ctx, 0), send: sink.send}

	done := make(chan error, 1)
	go func() {
		_, err := w.Write([]byte("blocked"))
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, errDownloadAborted) {
			t.Fatalf("err = %v, want errDownloadAborted", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Write did not return after cancel")
	}
	if sink.total() != 0 {
		t.Fatalf("sent %d bytes after abort, want 0", sink.total())
	}
}

func TestCreditWriterFailsWhenTheSocketWriteFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink := &recordingSend{fail: errors.New("socket closed")}
	w := &creditWriter{window: newCreditWindow(ctx, fdsInitialWindowBytes), send: sink.send}

	n, err := w.Write([]byte("data"))
	if n != 0 || !errors.Is(err, errDownloadAborted) {
		t.Fatalf("Write = %d, %v; want 0, errDownloadAborted", n, err)
	}
}
