package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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
	tracer module.Tracer

	// Total time spent in onReceipt excluding checkSealing
	onReceiptDuration prometheus.Counter

	// Total time spent in onApproval excluding checkSealing
	onApprovalDuration prometheus.Counter

	// Total time spent in checkSealing
	checkSealingDuration prometheus.Counter

	// The number of emergency seals
	emergencySealedBlocks prometheus.Counter
}

// NewConsensusCollector created a new consensus collector
func NewConsensusCollector(tracer module.Tracer, registerer prometheus.Registerer) *ConsensusCollector {
	onReceiptDuration := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "push_receipts_on_receipt_duration_seconds_total",
		Namespace: namespaceConsensus,
		Subsystem: subsystemMatchEngine,
		Help:      "time spent in consensus matching engine's onReceipt method in seconds",
	})
	onApprovalDuration := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "on_approval_duration_seconds_total",
		Namespace: namespaceConsensus,
		Subsystem: subsystemMatchEngine,
		Help:      "time spent in consensus matching engine's onApproval method in seconds",
	})
	checkSealingDuration := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "check_sealing_duration_seconds_total",
		Namespace: namespaceConsensus,
		Subsystem: subsystemMatchEngine,
		Help:      "time spent in consensus matching engine's checkSealing method in seconds",
	})
	emergencySealedBlocks := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "emergency_sealed_blocks_total",
		Namespace: namespaceConsensus,
		Subsystem: subsystemCompliance,
		Help:      "the number of blocks sealed in emergency mode",
	})
	registerer.MustRegister(
		onReceiptDuration,
		onApprovalDuration,
		checkSealingDuration,
		emergencySealedBlocks,
	)
	cc := &ConsensusCollector{
		tracer:                tracer,
		onReceiptDuration:     onReceiptDuration,
		onApprovalDuration:    onApprovalDuration,
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

// EmergencySeal increments the counter of emergency seals.
func (cc *ConsensusCollector) EmergencySeal() {
	cc.emergencySealedBlocks.Inc()
}

// OnReceiptProcessingDuration increases the number of seconds spent processing receipts
func (cc *ConsensusCollector) OnReceiptProcessingDuration(duration time.Duration) {
	cc.onReceiptDuration.Add(duration.Seconds())
}

// OnApprovalProcessingDuration increases the number of seconds spent processing approvals
func (cc *ConsensusCollector) OnApprovalProcessingDuration(duration time.Duration) {
	cc.onApprovalDuration.Add(duration.Seconds())
}

// CheckSealingDuration increases the number of seconds spent in checkSealing
func (cc *ConsensusCollector) CheckSealingDuration(duration time.Duration) {
	cc.checkSealingDuration.Add(duration.Seconds())
}
