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

func (n Noop) BlockFinalized(*flow.Header) {}

func (n Noop) BlockProcessable(*flow.Header, *flow.QuorumCertificate) {}

func (n Noop) EpochTransition(uint64, *flow.Header) {}

func (n Noop) EpochSetupPhaseStarted(uint64, *flow.Header) {}

func (n Noop) EpochCommittedPhaseStarted(_ uint64, _ *flow.Header) {}

func (n Noop) EpochFallbackModeTriggered(uint64, *flow.Header) {}

func (n Noop) EpochFallbackModeExited(uint64, *flow.Header) {}

func (n Noop) EpochExtended(_ uint64, _ *flow.Header, _ flow.EpochExtension) {}

func (n Noop) ActiveClustersChanged(flow.ChainIDList) {}
