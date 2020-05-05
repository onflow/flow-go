package metrics

import (
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Collection Metrics
const (

	// span from a transaction being received to being included in a block
	spanTransactionToCollection = "transaction_to_collection"

	// span from a collection being proposed to being finalized (eg. guaranteed)
	// TODO not used -- this doesn't really make sense until we use hotstuff
	spanCollectionToGuarantee = "collection_to_guarantee"
)

var (
	transactionsIngestedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceCollection,
		Name:      "ingested_transactions_total",
		Help:      "count of transactions ingested by this node",
	})
	proposalsCounter = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceCollection,
		Name:      "proposals_total",
		Help:      "count of collection proposals",
	})
	proposalSizeGauge = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceCollection,
		Buckets:   []float64{5, 10, 50, 100}, //TODO(andrew) update once collection limits are known
		Name:      "proposal_size_transactions",
		Help:      "number of transactions in proposed collections",
	})
	guaranteedCollectionSizeGauge = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceCollection,
		Buckets:   []float64{5, 10, 50, 100}, //TODO(andrew) update once collection limits are known
		Name:      "guarantee_size_transactions",
		Help:      "number of transactions in guaranteed collections",
	})
	pendingClusterBlocksGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCollection,
		Subsystem: subsystemProposal,
		Name:      "pending_blocks_total",
		Help:      "number of cluster blocks in pending cache of collection proposal engine",
	})
)

// PendingClusterBlocks sets the number of cluster blocks in the pending cache.
func (c *Collector) PendingClusterBlocks(n uint) {
	pendingClusterBlocksGauge.Set(float64(n))
}

// TransactionReceived starts a span to trace the duration of a transaction
// from being created to being included as part of a collection.
func (c *Collector) TransactionReceived(txID flow.Identifier) {
	transactionsIngestedCounter.Inc()
	c.tracer.StartSpan(txID, spanTransactionToCollection)
}

// CollectionProposed tracks the size and number of proposals.
func (c *Collector) CollectionProposed(collection flow.LightCollection) {
	proposalSizeGauge.Observe(float64(collection.Len()))
	proposalsCounter.Inc()
}

// CollectionGuaranteed updates the guaranteed collection size gauge and
// finishes the tx->collection span for each constituent transaction.
func (c *Collector) CollectionGuaranteed(collection flow.LightCollection) {
	guaranteedCollectionSizeGauge.Observe(float64(collection.Len()))
	for _, txID := range collection.Transactions {
		c.tracer.FinishSpan(txID, spanTransactionToCollection)
	}
}

// StartCollectionToGuarantee starts a span to trace the duration of a collection
// from being created to being submitted as a collection guarantee
// TODO not used, revisit once HotStuff is in use
func (c *Collector) StartCollectionToGuarantee(collection flow.LightCollection) {

	followsFrom := make([]opentracing.StartSpanOption, 0, len(collection.Transactions))
	for _, txID := range collection.Transactions {
		if txSpan, exists := c.tracer.GetSpan(txID, spanTransactionToCollection); exists {
			// link its transactions' spans
			followsFrom = append(followsFrom, opentracing.FollowsFrom(txSpan.Context()))
		}
	}

	c.tracer.StartSpan(collection.ID(), spanCollectionToGuarantee, followsFrom...).
		SetTag("collection_id", collection.ID().String()).
		SetTag("collection_txs", collection.Transactions)
}

// FinishCollectionToGuarantee finishes a span to trace the duration of a collection
// from being proposed to being finalized (eg. guaranteed).
// TODO not used, revisit once HotStuff is in use
func (c *Collector) FinishCollectionToGuarantee(collectionID flow.Identifier) {
	c.tracer.FinishSpan(collectionID, spanCollectionToGuarantee)
}
