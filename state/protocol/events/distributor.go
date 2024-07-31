package events

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// Distributor implements the `protocol.Consumer` interface for ingesting notifications emitted
// by the protocol state. It distributes the notifications to all registered consumers.
type Distributor struct {
	subscribers []protocol.Consumer
	mu          sync.RWMutex
}

var _ protocol.Consumer = (*Distributor)(nil)

// NewDistributor returns a new events distributor.
func NewDistributor() *Distributor {
	return &Distributor{}
}

func (d *Distributor) AddConsumer(consumer protocol.Consumer) {
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

func (d *Distributor) BlockProcessable(block *flow.Header, certifyingQC *flow.QuorumCertificate) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.BlockProcessable(block, certifyingQC)
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

func (d *Distributor) EpochFallbackModeTriggered(epochCounter uint64, header *flow.Header) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.EpochFallbackModeTriggered(epochCounter, header)
	}
}

func (d *Distributor) EpochFallbackModeExited(epochCounter uint64, header *flow.Header) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.EpochFallbackModeExited(epochCounter, header)
	}
}

func (d *Distributor) EpochExtended(epochCounter uint64, header *flow.Header, extension flow.EpochExtension) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, sub := range d.subscribers {
		sub.EpochExtended(epochCounter, header, extension)
	}
}
