package events

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Noop is a no-op implementation of protocol.Events.
type Noop struct{}

func NewNoop() *Noop {
	return &Noop{}
}

func (n Noop) BlockFinalized(block *flow.Header) {
}

func (n Noop) EpochTransition(newEpoch uint64, first *flow.Header) {
}

func (n Noop) EpochSetupPhaseStarted(epoch uint64, first *flow.Header) {
}

func (n Noop) EpochCommittedPhaseStarted(epoch uint64, first *flow.Header) {
}
