package shutdown

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var logger *log.Logger = log.Default()

var DefaultShutdown = New()

func Add(fn func()) {
	DefaultShutdown.Add(fn)
}

func ExecuteShutdownHandlers() {
	DefaultShutdown.ExecuteShutdownHandlers()
}

func Listen() {
	DefaultShutdown.Listen()
}

func SendShutdownSignal(indicateFailure bool) {
	DefaultShutdown.SendShutdownSignal(indicateFailure)
}

type Shutdown struct {
	hooks                  []func()
	mutex                  *sync.Mutex
	internalShutdownSignal chan bool
}

func New() *Shutdown {
	self := &Shutdown{}
	self.hooks = []func(){}
	self.mutex = &sync.Mutex{}
	self.internalShutdownSignal = make(chan bool)
	return self
}

func (self *Shutdown) Add(fn func()) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	self.hooks = append(self.hooks, fn)
}

func (self *Shutdown) SendShutdownSignal(indicateFailure bool) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	logger.Println("sending request to shut down")
	self.internalShutdownSignal <- indicateFailure
}

func (self *Shutdown) ExecuteShutdownHandlers() chan struct{} {
	finishedSignaler := make(chan struct{})
	go func() {
		self.mutex.Lock()
		defer self.mutex.Unlock()
		var wg sync.WaitGroup
		for _, fn := range self.hooks {
			wg.Go(fn)
		}
		wg.Wait()
		finishedSignaler <- struct{}{}
	}()
	return finishedSignaler
}

const (
	ExitCodeOk      int = 0
	ExitCodeFailure int = 1
	ExitCodeTimeout int = 126
	ExitCodeUnknown int = 127
)

func (self *Shutdown) Listen() {
	osSignalListener := make(chan os.Signal, 1)
	signal.Notify(osSignalListener, syscall.SIGINT, syscall.SIGTERM)
	exitCode := ExitCodeUnknown

	select {
	case receivedSignal := <-osSignalListener:
		logger.Println("received signal to shut down: ", receivedSignal.String())
		switch receivedSignal {
		case syscall.SIGINT:
			exitCode = ExitCodeOk
		case syscall.SIGTERM:
			exitCode = ExitCodeFailure
		}
	case indicateFailure := <-self.internalShutdownSignal:
		logger.Printf("received internal request to shut down with failure(%v)", indicateFailure)
		if indicateFailure {
			exitCode = ExitCodeFailure
		} else {
			exitCode = ExitCodeOk
		}
	}

	select {
	case <-self.ExecuteShutdownHandlers():
		logger.Printf("shutdown routines finished")
	case <-time.After(30 * time.Second):
		logger.Printf("timeout reached while handling shutdown routines")
		exitCode = ExitCodeTimeout
	}

	os.Exit(exitCode)
}
