package shutdown

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	hooks []func()
	mutex *sync.Mutex
}

func New() *Shutdown {
	return &Shutdown{
		hooks: []func(){},
		mutex: &sync.Mutex{},
	}
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
	if indicateFailure {
		err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		if err != nil {
			panic(fmt.Errorf("failed to send SIGTERM signal: %s", err.Error()))
		}
	} else {
		err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		if err != nil {
			panic(fmt.Errorf("failed to send SIGINT signal: %s", err.Error()))
		}
	}
}

func (self *Shutdown) ExecuteShutdownHandlers() {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	var wg sync.WaitGroup
	for _, fn := range self.hooks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	wg.Wait()
	logger.Println("finished shutdown routines")
}

func (self *Shutdown) Listen() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	receivedSignal := <-ch
	logger.Println("received request to shut down: ", receivedSignal.String())

	self.ExecuteShutdownHandlers()

	switch receivedSignal {
	case syscall.SIGINT:
		os.Exit(0)
	case syscall.SIGTERM:
		os.Exit(1)
	default:
		os.Exit(255)
	}
}
