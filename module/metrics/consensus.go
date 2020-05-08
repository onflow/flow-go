package metrics

import (
	"time"

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

	HotstuffEventTypeTimeout    = "timeout"
	HotstuffEventTypeOnProposal = "onproposal"
	HotstuffEventTypeOnVote     = "onvote"
)

var (
	finalizedSealCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "seals_per_finalized_block",
		Namespace: namespaceConsensus,
		Help:      "The number of seals included in the finalized block",
	})
	finalizedBlockCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "finalized_blocks",
		Namespace: namespaceConsensus,
		Help:      "The number of finalized blocks",
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
	hotstuffBusyDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "busy_duration_seconds",
		Namespace: namespaceConsensus,
		Subsystem: "hotstuff",
		Help:      "the duration of how long hotstuff's event loop has been busy processing one event",
		Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
	}, []string{"event_type"})
	hotstuffIdleDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "idle_duration_seconds",
		Namespace: namespaceConsensus,
		Subsystem: "hotstuff",
		Help:      "the duration of how long hotstuff's event loop has been idle without processing any event",
		Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
	})
	hotstuffWaitDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "wait_duration_seconds",
		Namespace: namespaceConsensus,
		Subsystem: "hotstuff",
		Help:      "the duration of how long an event has been waited in the hotstuff event loop queue before being processed.",
		Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
	}, []string{"event_type"})
	newviewGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "cur_view",
		Namespace: namespaceConsensus,
		Subsystem: "hotstuff",
		Help:      "the current view that the event handler has entered",
	})
	newestKnownQC = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "view_of_newest_known_qc",
		Namespace: namespaceConsensus,
		Subsystem: "hotstuff",
		Help:      "The view of the newest known qc from hotstuff",
	})
	blockProposalCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "block_proposals",
		Namespace: namespaceConsensus,
		Help:      "the number of block proposals made",
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
func (c *Collector) StartCollectionToFinalized(collectionID flow.Identifier) {
	c.tracer.StartSpan(collectionID, consensusCollectionToFinalized)
}

// FinishCollectionToFinalized reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
func (c *Collector) FinishCollectionToFinalized(collectionID flow.Identifier) {
	c.tracer.FinishSpan(collectionID, consensusCollectionToFinalized)
}

// CollectionsInFinalizedBlock reports Metric C2: Counter: Total number of Collections included in finalized Blocks (converted later to rate)
func (c *Collector) CollectionsInFinalizedBlock(count int) {
	collectionsPerFinalizedBlockCounter.Add(float64(count))
}

// CollectionsPerBlock reports Metric C3: Gauge type: number of Collections per Block
func (c *Collector) CollectionsPerBlock(count int) {
	collectionsPerBlock.Set(float64(count))
}

// StartBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
func (c *Collector) StartBlockToSeal(blockID flow.Identifier) {
	c.tracer.StartSpan(blockID, consensusBlockToSeal)
}

// FinishBlockToSeal reports Metrics C4: Block Received by CCL → Block Seal in finalized block
func (c *Collector) FinishBlockToSeal(blockID flow.Identifier) {
	c.tracer.FinishSpan(blockID, consensusBlockToSeal)
}

// SealsInFinalizedBlock reports Metrics C5 Counter: Total number of Blocks which are sealed by finalized blocks (converted later to rate)
func (c *Collector) SealsInFinalizedBlock(count int) {
	finalizedSealCounter.Add(float64(count))
}

// HotStuffBusyDuration reports Metrics C6 HotStuff Busy Duration
func (c *Collector) HotStuffBusyDuration(duration time.Duration, event string) {
	hotstuffBusyDuration.WithLabelValues(event).Observe(duration.Seconds())
}

// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
func (c *Collector) HotStuffIdleDuration(duration time.Duration) {
	hotstuffIdleDuration.Observe(duration.Seconds())
}

// HotStuffWaitDuration reports Metrics C6 HotStuff Wait Duration
func (c *Collector) HotStuffWaitDuration(duration time.Duration, event string) {
	hotstuffWaitDuration.WithLabelValues(event).Observe(duration.Seconds())
}

// FinalizedBlocks reports Metric C7: Number of Blocks Finalized (per second)
func (c *Collector) FinalizedBlocks(count int) {
	finalizedBlockCounter.Add(float64(count))
}

// StartNewView reports Metrics C8: Current View
func (c *Collector) StartNewView(view uint64) {
	newviewGauge.Set(float64(view))
}

// NewestKnownQC reports Metrics C9: View of Newest Known QC
func (c *Collector) NewestKnownQC(view uint64) {
	newestKnownQC.Set(float64(view))
}

// MadeBlockProposal reports that a block proposal has been made
func (c *Collector) MadeBlockProposal() {
	blockProposalCounter.Inc()
}

// MempoolGuaranteesSize reports the size of the guarantees mempool
func (c *Collector) MempoolGuaranteesSize(size uint) {
	mempoolGuaranteesSizeGauge.Set(float64(size))
}

// MempoolReceiptsSize reports the size of the receipts mempool
func (c *Collector) MempoolReceiptsSize(size uint) {
	mempoolReceiptsSizeGauge.Set(float64(size))
}

// MempoolApprovalsSize reports the size of the approvals mempool
func (c *Collector) MempoolApprovalsSize(size uint) {
	mempoolApprovalsSizeGauge.Set(float64(size))
}

// MempoolSealsSize reports the size of the seals mempool
func (c *Collector) MempoolSealsSize(size uint) {
	mempoolSealsSizeGauge.Set(float64(size))
}
