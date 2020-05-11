package metrics

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/trace"
)

// Consensus spans.
const (
	// a duration metrics
	// from a collection being received
	// to being included in a finalized block
	consensusCollectionToFinalized = "consensus_collection_to_finalized"

	// a duration metrics
	// from a seal being received
	// to being included in a finalized block
	consensusBlockToSeal = "consensus_block_to_seal"
)

type ConsensusCollector struct {
	tracer *trace.OpenTracer
}

func NewConsensusCollector(tracer *trace.OpenTracer) *ConsensusCollector {

	cc := &ConsensusCollector{
		tracer: tracer,
	}

	return cc
}

// StartCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
func (cc *ConsensusCollector) StartCollectionToFinalized(collectionID flow.Identifier) {
	cc.tracer.StartSpan(collectionID, consensusCollectionToFinalized)
}

// FinishCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
func (cc *ConsensusCollector) FinishCollectionToFinalized(collectionID flow.Identifier) {
	cc.tracer.FinishSpan(collectionID, consensusCollectionToFinalized)
}

// StartBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
func (cc *ConsensusCollector) StartBlockToSeal(blockID flow.Identifier) {
	cc.tracer.StartSpan(blockID, consensusBlockToSeal)
}

// FinishBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
func (cc *ConsensusCollector) FinishBlockToSeal(blockID flow.Identifier) {
	cc.tracer.FinishSpan(blockID, consensusBlockToSeal)
}
