package sshgateway

import (
	"sync"

	"k8s.io/client-go/tools/remotecommand"
)

// sizeQueue is a channel-fed remotecommand.TerminalSizeQueue. Unlike the
// SIGWINCH-based queue in src/k8sexec it is driven by SSH pty-req and
// window-change requests. Latest-wins: a resize arriving while the previous
// one is still queued replaces it, so a burst of drags never lags behind.
type sizeQueue struct {
	mu     sync.Mutex
	ch     chan remotecommand.TerminalSize
	closed bool
}

func newSizeQueue() *sizeQueue {
	return &sizeQueue{ch: make(chan remotecommand.TerminalSize, 1)}
}

func (q *sizeQueue) push(size remotecommand.TerminalSize) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	// Drop the stale pending size, if any, then queue the new one.
	select {
	case <-q.ch:
	default:
	}
	q.ch <- size
}

func (q *sizeQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
}

// Next blocks until a new terminal size is available. Returning nil ends the
// remote resize loop (queue closed = session over).
func (q *sizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.ch
	if !ok {
		return nil
	}
	return &size
}
