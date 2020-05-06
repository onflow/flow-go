package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Consensus Metrics
const (
	// a duration metrics
	// from a collection being received
	// to being included in a finalized block
	consensusCollectionToFinalized = "consensus_collection_to_finalized"

	// a duration metrics
	// from a seal being received
	// to being included in a finalized block
	consensusBlockToSeal = "consensus_block_to_seal"

	// HotStuff committee formed by the consensus nodes
	mainConsensusCommittee = "main_consensus"
)

var (
	finalizedSealCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "seals_per_finalized_block",
		Namespace: namespaceConsensus,
		Help:      "The number of seals included in the finalized block",
	})
	collectionsPerBlock = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "collections_per_block",
		Namespace: namespaceConsensus,
		Help:      "the number of collections per block",
	})
	collectionsPerFinalizedBlockCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "collections_per_finalized_block",
		Namespace: namespaceConsensus,
		Help:      "The number of collections included in the finalized block",
	})
	mempoolGuaranteesSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "mempool_guarantees_size",
		Namespace: namespaceConsensus,
		Help:      "the size of the guarantees mempool",
	})
	mempoolReceiptsSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "mempool_receipts_size",
		Namespace: namespaceConsensus,
		Help:      "the size of the receipts mempool",
	})
	mempoolApprovalsSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "mempool_approvals_size",
		Namespace: namespaceConsensus,
		Help:      "the size of the approvals mempool",
	})
	mempoolSealsSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "mempool_seals_size",
		Namespace: namespaceConsensus,
		Help:      "the size of the seals mempool",
	})
	pendingBlocksGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceConsensus,
		Subsystem: subsystemCompliance,
		Name:      "pending_blocks_total",
		Help:      "number of blocks in pending cache of compliance engine",
	})
)

// PendingBlocks sets the number of blocks in the pending cache.
func (c *Collector) PendingBlocks(n uint) {
	pendingBlocksGauge.Set(float64(n))
}

// StartCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
func (c *BaseMetrics) StartCollectionToFinalized(collectionID flow.Identifier) {
	c.tracer.StartSpan(collectionID, consensusCollectionToFinalized)
}

// FinishCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
func (c *BaseMetrics) FinishCollectionToFinalized(collectionID flow.Identifier) {
	c.tracer.FinishSpan(collectionID, consensusCollectionToFinalized)
}

// CollectionsInFinalizedBlock reports Metric C2: Counter: Total number of Collections included in finalized Blocks (converted later to rate)
func (c *BaseMetrics) CollectionsInFinalizedBlock(count int) {
	collectionsPerFinalizedBlockCounter.Add(float64(count))
}

// CollectionsPerBlock reports Metric C3: Gauge type: number of Collections per Block
func (c *BaseMetrics) CollectionsPerBlock(count int) {
	collectionsPerBlock.Set(float64(count))
}

// StartBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
func (c *BaseMetrics) StartBlockToSeal(blockID flow.Identifier) {
	c.tracer.StartSpan(blockID, consensusBlockToSeal)
}

// FinishBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
func (c *BaseMetrics) FinishBlockToSeal(blockID flow.Identifier) {
	c.tracer.FinishSpan(blockID, consensusBlockToSeal)
}

// SealsInFinalizedBlock reports Metrics C5 Counter: Total number of Blocks which are sealed by finalized blocks (converted later to rate)
func (c *BaseMetrics) SealsInFinalizedBlock(count int) {
	finalizedSealCounter.Add(float64(count))
}

// MempoolGuaranteesSize reports the size of the guarantees mempool
func (c *BaseMetrics) MempoolGuaranteesSize(size uint) {
	mempoolGuaranteesSizeGauge.Set(float64(size))
}

// MempoolReceiptsSize reports the size of the receipts mempool
func (c *BaseMetrics) MempoolReceiptsSize(size uint) {
	mempoolReceiptsSizeGauge.Set(float64(size))
}

// MempoolApprovalsSize reports the size of the approvals mempool
func (c *BaseMetrics) MempoolApprovalsSize(size uint) {
	mempoolApprovalsSizeGauge.Set(float64(size))
}

// MempoolSealsSize reports the size of the seals mempool
func (c *BaseMetrics) MempoolSealsSize(size uint) {
	mempoolSealsSizeGauge.Set(float64(size))
}
