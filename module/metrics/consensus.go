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
	hotstuffBusyDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "busy_duration",
		Namespace: namespaceConsensus,
		Subsystem: "hotstuff",
		Help:      "the duration of how long hotstuff's event loop has been busy processing one event",
	}, []string{"event_type"})
	hotstuffIdleDuration = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "idle_duration",
		Namespace: namespaceConsensus,
		Subsystem: "hotstuff",
		Help:      "the duration of how long hotstuff's event loop has been idle without processing any event",
	})
	hotstuffWaitDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "wait_duration",
		Namespace: namespaceConsensus,
		Subsystem: "hotstuff",
		Help:      "the duration of how long an event has been waited in the hotstuff event loop queue before being processed.",
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
)

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
	hotstuffBusyDuration.WithLabelValues(event).Set(float64(duration))
}

// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
func (c *Collector) HotStuffIdleDuration(duration time.Duration) {
	hotstuffIdleDuration.Set(float64(duration))
}

// HotStuffWaitDuration reports Metrics C6 HotStuff Wait Duration
func (c *Collector) HotStuffWaitDuration(duration time.Duration, event string) {
	hotstuffWaitDuration.WithLabelValues(event).Set(float64(duration))
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
