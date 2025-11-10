package core

import (
	"context"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/shutdown"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type LeaderElector interface {
	Run()
	Id() string
	IsLeading() bool
	OnLeading(cb func())
	OnLeadingEnded(cb func())
	OnLeaderChanged(cb func(newLeaderId string))
}

type leaderElector struct {
	logger *slog.Logger
	config config.ConfigModule

	id                  string
	leading             atomic.Bool
	running             atomic.Bool
	onLeading           []func()
	onLeadingLock       sync.RWMutex
	onLeadingEnded      []func()
	onLeadingEndedLock  sync.RWMutex
	onLeaderChanged     []func(newLeaderId string)
	onLeaderChangedLock sync.RWMutex

	clientProvider k8sclient.K8sClientProvider
}

func NewLeaderElector(
	logger *slog.Logger,
	configModule config.ConfigModule,
	clientProvider k8sclient.K8sClientProvider,
) LeaderElector {
	self := &leaderElector{}

	self.logger = logger
	self.config = configModule
	self.id = uuid.New().String()
	self.leading = atomic.Bool{}
	self.onLeading = []func(){}
	self.onLeadingLock = sync.RWMutex{}
	self.onLeadingEnded = []func(){}
	self.onLeadingEndedLock = sync.RWMutex{}
	self.onLeaderChanged = []func(newLeaderId string){}
	self.onLeaderChangedLock = sync.RWMutex{}

	self.clientProvider = clientProvider

	return self
}

func (self *leaderElector) Run() {
	wasRunnning := self.running.Swap(true)
	assert.Assert(!wasRunnning, "leader elector should only be started once")
	go func() {
		client := self.clientProvider.K8sClientSet()

		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      "mogenius-operator-lease",
				Namespace: self.config.Get("MO_OWN_NAMESPACE"),
			},
			Client: client.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: self.id,
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		shutdown.Add(func() {
			cancel()
		})

		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   300 * time.Second,
			RenewDeadline:   60 * time.Second,
			RetryPeriod:     30 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					self.leading.Store(true)
					self.logger.Info("instance started leading", "ownId", self.id)
					self.onLeadingLock.RLock()
					defer self.onLeadingLock.RUnlock()
					for _, cb := range self.onLeading {
						cb()
					}
				},
				OnStoppedLeading: func() {
					wasLeading := self.leading.Swap(false)
					if wasLeading {
						self.logger.Info("instance is not leading anymore", "ownId", self.id)
						self.onLeadingEndedLock.RLock()
						defer self.onLeadingEndedLock.RUnlock()
						for _, cb := range self.onLeadingEnded {
							cb()
						}
					}
				},
				OnNewLeader: func(identity string) {
					self.logger.Info("a new leader was elected", "ownId", self.id, "leaderId", identity)
					self.onLeaderChangedLock.RLock()
					defer self.onLeaderChangedLock.RUnlock()
					for _, cb := range self.onLeaderChanged {
						cb(identity)
					}
				},
			},
		})
	}()
}

func (self *leaderElector) IsLeading() bool {
	return self.leading.Load()
}

func (self *leaderElector) Id() string {
	return self.id
}

func (self *leaderElector) OnLeading(cb func()) {
	self.onLeadingLock.Lock()
	defer self.onLeadingLock.Unlock()
	self.onLeading = append(self.onLeading, cb)
}

func (self *leaderElector) OnLeadingEnded(cb func()) {
	self.onLeadingEndedLock.Lock()
	defer self.onLeadingEndedLock.Unlock()
	self.onLeadingEnded = append(self.onLeadingEnded, cb)
}

func (self *leaderElector) OnLeaderChanged(cb func(newLeaderId string)) {
	self.onLeaderChangedLock.Lock()
	defer self.onLeaderChangedLock.Unlock()
	self.onLeaderChanged = append(self.onLeaderChanged, cb)
}
