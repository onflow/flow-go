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

func (n Noop) BlockFinalized(*flow.UnsignedHeader) {}

func (n Noop) BlockProcessable(*flow.UnsignedHeader, *flow.QuorumCertificate) {}

func (n Noop) EpochTransition(uint64, *flow.UnsignedHeader) {}

func (n Noop) EpochSetupPhaseStarted(uint64, *flow.UnsignedHeader) {}

func (n Noop) EpochCommittedPhaseStarted(uint64, *flow.UnsignedHeader) {}

func (n Noop) EpochFallbackModeTriggered(uint64, *flow.UnsignedHeader) {}

func (n Noop) EpochFallbackModeExited(uint64, *flow.UnsignedHeader) {}

func (n Noop) EpochExtended(uint64, *flow.UnsignedHeader, flow.EpochExtension) {}

func (n Noop) ActiveClustersChanged(flow.ChainIDList) {}
