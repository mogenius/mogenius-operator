package shutdown

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var shutdownLogger = slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("component", "shutdown")

var DefaultShutdown = New()

func Add(fn func()) {
	DefaultShutdown.Add(fn)
}

func Listen() {
	DefaultShutdown.Listen()
}

func SendShutdownSignalAndBlockForever(indicateFailure bool) {
	DefaultShutdown.SendShutdownSignalAndBlockForever(indicateFailure)
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

func (s *Shutdown) Add(fn func()) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.hooks = append(s.hooks, fn)
}

func (s *Shutdown) SendShutdownSignalAndBlockForever(indicateFailure bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	shutdownLogger.Info("sending request to shut down", "indicateFailure", indicateFailure)
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
	select {} // block forever
}

func (s *Shutdown) Listen() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	receivedSignal := <-ch
	s.mutex.Lock()
	defer s.mutex.Unlock()
	shutdownLogger.Info("received request to shut down", "receivedSignal", receivedSignal.String())
	var wg sync.WaitGroup
	for _, fn := range s.hooks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)
	shutdownLogger.Info("finished shutdown routines", "receivedSignal", receivedSignal.String())
	switch receivedSignal {
	case syscall.SIGINT:
		os.Exit(0)
	case syscall.SIGTERM:
		os.Exit(1)
	default:
		os.Exit(255)
	}
}
