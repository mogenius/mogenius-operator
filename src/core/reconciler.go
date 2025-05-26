package core

import (
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"sync/atomic"
	"time"
)

type Reconciler interface {
	Link(leaderElector LeaderElector)
	Run()
	Start()
	Stop()
}

type reconciler struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider
	leaderElector  LeaderElector
	active         atomic.Bool
}

func NewReconciler(
	logger *slog.Logger,
	config config.ConfigModule,
	clientProvider k8sclient.K8sClientProvider,
) Reconciler {
	self := &reconciler{}
	self.logger = logger
	self.config = config
	self.clientProvider = clientProvider
	self.active = atomic.Bool{}

	return self
}

func (self *reconciler) Link(leaderElector LeaderElector) {
	self.leaderElector = leaderElector
}

func (self *reconciler) Run() {
	assert.Assert(self.leaderElector != nil)

	self.leaderElector.OnLeading(self.Start)
	self.leaderElector.OnLeadingEnded(self.Stop)

	go func() {
		updateTicker := time.NewTicker(2 * time.Second)
		defer updateTicker.Stop()

		for {
			select {
			case <-updateTicker.C:
				if !self.active.Load() {
					continue
				}
				self.reconcile()
			}
		}
	}()
}

func (self *reconciler) Start() {
	self.active.Store(true)
}

func (self *reconciler) Stop() {
	self.active.Store(false)
}

func (self *reconciler) reconcile() {
	self.logger.Debug("tick")
}
