package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
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

// ConsensusCollector ...
type ConsensusCollector struct {
	tracer *trace.OpenTracer

	// The duration of the full sealing check
	checkSealingDuration prometheus.Histogram

	// The number of emergency seals
	emergencySealedBlocks prometheus.Counter
}

// NewConsensusCollector created a new consensus collector
func NewConsensusCollector(tracer *trace.OpenTracer, registerer prometheus.Registerer) *ConsensusCollector {
	checkSealingDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceConsensus,
		Subsystem: subsystemMatchEngine,
		Name:      "check_sealing_duration_s",
		Help:      "duration of consensus match engine sealing check in seconds",
	})
	emergencySealedBlocks := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "emergency_sealed_blocks_total",
		Namespace: namespaceConsensus,
		Subsystem: subsystemCompliance,
		Help:      "the number of blocks sealed in emergency mode",
	})
	registerer.MustRegister(checkSealingDuration, emergencySealedBlocks)
	cc := &ConsensusCollector{
		tracer:                tracer,
		checkSealingDuration:  checkSealingDuration,
		emergencySealedBlocks: emergencySealedBlocks,
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

// CheckSealingDuration records absolute time for the full sealing check by the consensus match engine
func (cc *ConsensusCollector) CheckSealingDuration(duration time.Duration) {
	cc.checkSealingDuration.Observe(duration.Seconds())
}

// EmergencySeal increments the counter of emergency seals.
func (cc *ConsensusCollector) EmergencySeal() {
	cc.emergencySealedBlocks.Inc()
}
