package ingestion

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage"
)

var _ notifications.FinalizationConsumer = FinalizerSubscriber{}

// FinalizerSubscriber delivers the finalized block to the given engine
type FinalizerSubscriber struct {
	log    zerolog.Logger
	engine network.Engine // an engine to whom the block should be delivered
	blocks storage.Blocks // the storage db
}

func NewFinalizerSubscriber(log zerolog.Logger, engine network.Engine, blocks storage.Blocks) *FinalizerSubscriber {
	return &FinalizerSubscriber{
		log:    log,
		engine: engine,
		blocks: blocks,
	}
}

func (f FinalizerSubscriber) OnFinalizedBlock(hb *model.Block) {
	id := hb.BlockID
	header, err := f.blocks.ByID(id)
	if err != nil {
		f.log.Error().Err(err).Hex("block_id", id[:]).Msg("failed to lookup block")
	}
	err = f.engine.ProcessLocal(header)
	if err != nil {
		f.log.Error().Err(err).Hex("block_id", id[:]).Msg("failed to process block")
	}
}

// Not used by the observation node
func (f FinalizerSubscriber) OnDoubleProposeDetected(*model.Block, *model.Block) {
}

// Not used by the observation node
func (f FinalizerSubscriber) OnBlockIncorporated(*model.Block) {
}
