package pruner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

type SealedBlocksProducer struct {
	events.Noop

	state    protocol.State
	producer *BlockProcessedProducer
}

func NewSealedBlocksProducer(state protocol.State, producer *BlockProcessedProducer) *SealedBlocksProducer {
	return &SealedBlocksProducer{
		state:    state,
		producer: producer,
	}
}

func (p *SealedBlocksProducer) BlockFinalized(header *flow.Header) {
	sealed, err := p.state.Sealed().Head()
	if err != nil {
		panic("failed to get sealed header")
	}

	p.producer.OnBlockProcessed(sealed.Height)
}
