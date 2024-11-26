package k8sexec

import (
	"context"
	"io"
)

// EOFReader wraps an input stream to detect EOF (CTRL+D).
type eofReader struct {
	reader io.Reader
	cancel context.CancelFunc
}

func (e *eofReader) Read(p []byte) (int, error) {
	n, err := e.reader.Read(p)
	if err == io.EOF {
		e.cancel() // Trigger context cancellation on EOF
	}
	return n, err
}
