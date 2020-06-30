package notifications

import (
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// ConsensusTracingConsumer is an implementation of the notifications consumer that adds tracing for consensus node hotstuff
type ConsensusTracingConsumer struct {
	// inherit from noop consumer in order to satisfy the full interface
	NoopConsumer
	log    zerolog.Logger
	tracer module.Tracer
	index  storage.Index
}

func NewConsensusTracingConsumer(log zerolog.Logger, tracer module.Tracer, index storage.Index) *ConsensusTracingConsumer {
	tc := &ConsensusTracingConsumer{
		log:    log,
		tracer: tracer,
		index:  index,
	}
	return tc
}

func (tc *ConsensusTracingConsumer) OnBlockIncorporated(block *model.Block) {
	if span, ok := tc.tracer.GetSpan(block.BlockID, trace.CONProcessBlock); ok {
		tc.tracer.StartSpan(block.BlockID, trace.CONHotFinalizeBlock, opentracing.ChildOf(span.Context()))
	}

	index, err := tc.index.ByBlockID(block.BlockID)
	if err != nil {
		tc.log.Error().
			Err(err).
			Uint64("block_view", block.View).
			Hex("block_id", logging.ID(block.BlockID)).
			Hex("proposer_id", logging.ID(block.ProposerID)).
			Hex("payload_hash", logging.ID(block.PayloadHash)).
			Msg("unable to find index for block to be finalized for tracing")
		return
	}

	for _, id := range index.CollectionIDs {
		if s, ok := tc.tracer.GetSpan(id, trace.CONProcessCollection); ok {
			tc.tracer.StartSpan(id, trace.CONHotFinalizeCollection, opentracing.ChildOf(s.Context()))
		}
	}
}

func (tc *ConsensusTracingConsumer) OnFinalizedBlock(block *model.Block) {
	tc.tracer.FinishSpan(block.BlockID, trace.CONHotFinalizeBlock)

	index, err := tc.index.ByBlockID(block.BlockID)
	if err != nil {
		tc.log.Error().
			Err(err).
			Uint64("block_view", block.View).
			Hex("block_id", logging.ID(block.BlockID)).
			Hex("proposer_id", logging.ID(block.ProposerID)).
			Hex("payload_hash", logging.ID(block.PayloadHash)).
			Msg("unable to find index for finalized block for tracing")
		return
	}

	for _, id := range index.CollectionIDs {
		tc.tracer.FinishSpan(id, trace.CONHotFinalizeCollection)
	}
}
