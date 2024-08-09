package pruner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/heightrecorder"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

type SealedBlocksRecorder struct {
	heightrecorder.ProcessedHeightRecorder
	events.Noop

	state protocol.State
}

func NewSealedBlocksRecorder(state protocol.State, producer heightrecorder.ProcessedHeightRecorder) *SealedBlocksRecorder {
	return &SealedBlocksRecorder{
		ProcessedHeightRecorder: producer,
		state:                   state,
	}
}

func (p *SealedBlocksRecorder) BlockFinalized(_ *flow.Header) {
	sealed, err := p.state.Sealed().Head()
	if err != nil {
		panic("failed to get sealed header in SealedBlocksRecorder")
	}

	p.OnBlockProcessed(sealed.Height)
}
