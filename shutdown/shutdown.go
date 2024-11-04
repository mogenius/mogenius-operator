package shutdown

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var DefaultShutdown = New()

func Add(fn func()) {
	DefaultShutdown.Add(fn)
}

func Listen() {
	DefaultShutdown.Listen()
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

func (s *Shutdown) Listen() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	s.mutex.Lock()
	defer s.mutex.Unlock()
	var wg sync.WaitGroup
	for _, fn := range s.hooks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	wg.Wait()
	os.Exit(0)
}
