package networkmonitor

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/ebpf"
	"sync/atomic"
	"time"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

type EbpfApi interface {
	WatchInterface(ctx context.Context, interfaceId int, tickInterval time.Duration) (chan CountState, error)
}

type ebpfApi struct {
	logger *slog.Logger

	initialized atomic.Bool
}

func NewEbpfApi(logger *slog.Logger) EbpfApi {
	self := &ebpfApi{}

	self.logger = logger
	self.initialized = atomic.Bool{}

	return self
}

type CountState struct {
	IngressPackets uint64
	IngressBytes   uint64
	EgressPackets  uint64
	EgressBytes    uint64
}

func (self *ebpfApi) lazyInit() {
	wasInitialized := self.initialized.Swap(true)
	if wasInitialized {
		return
	}

	// Remove resource limits for kernels <5.11.
	err := rlimit.RemoveMemlock()
	if err != nil {
		self.logger.Error("failed to remove memlock", "error", err)
	}
}

func (self *ebpfApi) WatchInterface(ctx context.Context, interfaceId int, tickInterval time.Duration) (chan CountState, error) {
	self.lazyInit()

	// Load the compiled eBPF ELF and load it into the kernel.
	var objs ebpf.EbpfModuleObjects
	err := ebpf.LoadEbpfModuleObjects(&objs, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load counter object: %v", err)
	}
	defer func() {
		if err != nil {
			objs.Close()
		}
	}()

	// TCX egress
	linkEgressTcx, err := link.AttachTCX(link.TCXOptions{
		Interface: interfaceId,
		Program:   objs.UpdateTcEgress,
		Attach:    cebpf.AttachTCXEgress,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach tcx egress: %v", err)
	}
	defer func() {
		if err != nil {
			linkEgressTcx.Close()
		}
	}()

	// TCX ingress
	linkIngressTcx, err := link.AttachTCX(link.TCXOptions{
		Interface: interfaceId,
		Program:   objs.UpdateTcIngress,
		Attach:    cebpf.AttachTCXIngress,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach tcx ingress: %v", err)
	}
	defer func() {
		if err != nil {
			linkIngressTcx.Close()
		}
	}()

	outChannel := make(chan CountState)

	// Periodically fetch the packet counter from PktCount,
	// exit the program when interrupted.
	tick := time.Tick(tickInterval)

	go func() {
		defer objs.Close()
		defer linkEgressTcx.Close()
		defer linkIngressTcx.Close()

		for {
			select {
			case <-tick:

				var ingressPackets uint64
				err = objs.IngressPktCount.Get(&ingressPackets)
				assert.Assert(err == nil, err)

				var ingressBytes uint64
				err = objs.IngressBytes.Get(&ingressBytes)
				assert.Assert(err == nil, err)

				var egressPackets uint64
				err = objs.EgressPktCount.Get(&egressPackets)
				assert.Assert(err == nil, err)

				var egressBytes uint64
				err = objs.EgressBytes.Get(&egressBytes)
				assert.Assert(err == nil, err)

				select {
				case outChannel <- CountState{
					IngressPackets: ingressPackets,
					IngressBytes:   ingressBytes,
					EgressPackets:  egressPackets,
					EgressBytes:    egressBytes,
				}:
				default:
				}
			case <-ctx.Done():
				close(outChannel)
				return
			}
		}
	}()

	return outChannel, nil
}
