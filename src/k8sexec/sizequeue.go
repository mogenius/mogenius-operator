package k8sexec

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
	"k8s.io/client-go/tools/remotecommand"
)

type sizeQueue struct {
	resize chan os.Signal
}

func NewSizeQueue() *sizeQueue {
	resize := make(chan os.Signal, 1)
	signal.Notify(resize, syscall.SIGWINCH)
	return &sizeQueue{resize: resize}
}

func (s *sizeQueue) Next() *remotecommand.TerminalSize {
	<-s.resize
	size, err := getTerminalSize()
	if err != nil {
		return nil
	}
	return size
}

func getTerminalSize() (*remotecommand.TerminalSize, error) {
	fd := int(os.Stdin.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		return nil, err
	}
	return &remotecommand.TerminalSize{
		Width:  uint16(width),
		Height: uint16(height),
	}, nil
}
