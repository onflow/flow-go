package events

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// Distributor distributes events to a list of subscribers.
type Distributor struct {
	subscribers []protocol.Consumer
	mu          sync.RWMutex
}

// NewDistributor returns a new events distributor.
func NewDistributor() *Distributor {
	return &Distributor{}
}

func (d *Distributor) AddConsumer(consumer protocol.Consumer) {
	fmt.Println("Distributor.AddConsumer")
	d.mu.Lock()
	defer d.mu.Unlock()
	d.subscribers = append(d.subscribers, consumer)
}

func (d *Distributor) BlockFinalized(block *flow.Header) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.BlockFinalized(block)
	}
}

func (d *Distributor) BlockProcessable(block *flow.Header) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.BlockProcessable(block)
	}
}

func (d *Distributor) EpochTransition(newEpoch uint64, first *flow.Header) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.EpochTransition(newEpoch, first)
	}
}

func (d *Distributor) EpochSetupPhaseStarted(epoch uint64, first *flow.Header) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.EpochSetupPhaseStarted(epoch, first)
	}
}

func (d *Distributor) EpochCommittedPhaseStarted(epoch uint64, first *flow.Header) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.EpochCommittedPhaseStarted(epoch, first)
	}
}
