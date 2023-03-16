package events

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// Noop is a no-op implementation of protocol.Consumer.
type Noop struct{}

var _ protocol.Consumer = (*Noop)(nil)

func NewNoop() *Noop {
	return &Noop{}
}

func (n Noop) BlockFinalized(block *flow.Header) {}

func (n Noop) BlockProcessable(block *flow.Header, certifyingQC *flow.QuorumCertificate) {}

func (n Noop) EpochTransition(newEpoch uint64, first *flow.Header) {}

func (n Noop) EpochSetupPhaseStarted(epoch uint64, first *flow.Header) {}

func (n Noop) EpochCommittedPhaseStarted(epoch uint64, first *flow.Header) {}

func (n Noop) EpochEmergencyFallbackTriggered() {}
