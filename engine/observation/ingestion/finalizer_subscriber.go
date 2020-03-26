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
	log     zerolog.Logger
	engine  network.Engine  // an engine to whom the block should be deliverd
	headers storage.Headers // the storage db
}

func NewFinalizerSubscriber(log zerolog.Logger, engine network.Engine, headers storage.Headers) *FinalizerSubscriber {
	return &FinalizerSubscriber{
		log:     log,
		engine:  engine,
		headers: headers,
	}
}

func (f FinalizerSubscriber) OnFinalizedBlock(hb *model.Block) {
	id := hb.BlockID
	header, err := f.headers.ByBlockID(id)
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
