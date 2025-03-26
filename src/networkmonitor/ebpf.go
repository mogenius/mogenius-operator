package networkmonitor

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/ebpf"
	"sync/atomic"
	"time"

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
	PacketCount uint64
	Bytes       uint64
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
	var objs ebpf.CounterObjects
	err := ebpf.LoadCounterObjects(&objs, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load counter object: %v", err)
	}
	defer func() {
		if err != nil {
			objs.Close()
		}
	}()

	// XDP
	linkXdp, err := link.AttachXDP(link.XDPOptions{
		Program:   objs.CountPackets,
		Interface: interfaceId,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach xdp: %v", err)
	}
	defer func() {
		if err != nil {
			linkXdp.Close()
		}
	}()

	outChannel := make(chan CountState)

	// Periodically fetch the packet counter from PktCount,
	// exit the program when interrupted.
	tick := time.Tick(tickInterval)

	go func() {
		defer objs.Close()
		defer linkXdp.Close()

		for {
			select {
			case <-tick:
				var count uint64
				err := objs.Counters.Lookup(uint32(0), &count)
				assert.Assert(err == nil, err)

				var bytes uint64
				err = objs.Counters.Lookup(uint32(1), &bytes)
				assert.Assert(err == nil, err)

				select {
				case outChannel <- CountState{PacketCount: count, Bytes: bytes}:
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
