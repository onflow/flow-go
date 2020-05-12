package metrics

import (
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/trace"
)

// Collection spans.
const (

	// span from a transaction being received to being included in a block
	spanTransactionToCollection = "transaction_to_collection"

	// span from a collection being proposed to being finalized (eg. guaranteed)
	// TODO not used -- this doesn't really make sense until we use hotstuff
	spanCollectionToGuarantee = "collection_to_guarantee"
)

type CollectionCollector struct {
	tracer                   *trace.OpenTracer
	transactionsIngested     prometheus.Counter
	proposals                prometheus.Counter
	proposalSize             prometheus.Histogram
	guaranteedCollectionSize prometheus.Histogram
	pendingClusterBlocks     prometheus.Gauge
}

func NewCollectionCollector(tracer *trace.OpenTracer) *CollectionCollector {

	cc := &CollectionCollector{
		tracer: tracer,

		transactionsIngested: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceCollection,
			Name:      "ingested_transactions_total",
			Help:      "count of transactions ingested by this node",
		}),

		proposals: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceCollection,
			Name:      "proposals_total",
			Help:      "count of collection proposals",
		}),

		proposalSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Buckets:   []float64{5, 10, 50, 100}, //TODO(andrew) update once collection limits are known
			Name:      "proposal_size_transactions",
			Help:      "number of transactions in proposed collections",
		}),

		guaranteedCollectionSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Buckets:   []float64{5, 10, 50, 100}, //TODO(andrew) update once collection limits are known
			Name:      "guarantee_size_transactions",
			Help:      "number of transactions in guaranteed collections",
		}),

		pendingClusterBlocks: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceCollection,
			Name:      "pending_blocks_total",
			Help:      "number of cluster blocks in pending cache of collection proposal engine",
		}),
	}

	return cc
}

// TransactionReceived starts a span to trace the duration of a transaction
// from being created to being included as part of a collection.
func (cc *CollectionCollector) TransactionReceived(txID flow.Identifier) {
	cc.transactionsIngested.Inc()
	cc.tracer.StartSpan(txID, spanTransactionToCollection)
}

// CollectionProposed tracks the size and number of proposals.
func (cc *CollectionCollector) CollectionProposed(collection flow.LightCollection) {
	cc.proposalSize.Observe(float64(collection.Len()))
	cc.proposals.Inc()
}

// CollectionGuaranteed updates the guaranteed collection size gauge and
// finishes the tx->collection span for each constituent transaction.
func (cc *CollectionCollector) CollectionGuaranteed(collection flow.LightCollection) {
	cc.guaranteedCollectionSize.Observe(float64(collection.Len()))
	for _, txID := range collection.Transactions {
		cc.tracer.FinishSpan(txID, spanTransactionToCollection)
	}
}

// PendingClusterBlocks sets the number of cluster blocks in the pending cache.
func (cc *CollectionCollector) PendingClusterBlocks(n uint) {
	cc.pendingClusterBlocks.Set(float64(n))
}

// StartCollectionToGuarantee starts a span to trace the duration of a collection
// from being created to being submitted as a collection guarantee
// TODO not used, revisit once HotStuff is in use
func (cc *CollectionCollector) StartCollectionToGuarantee(collection flow.LightCollection) {

	followsFrom := make([]opentracing.StartSpanOption, 0, len(collection.Transactions))
	for _, txID := range collection.Transactions {
		if txSpan, exists := cc.tracer.GetSpan(txID, spanTransactionToCollection); exists {
			// link its transactions' spans
			followsFrom = append(followsFrom, opentracing.FollowsFrom(txSpan.Context()))
		}
	}

	cc.tracer.StartSpan(collection.ID(), spanCollectionToGuarantee, followsFrom...).
		SetTag("collection_id", collection.ID().String()).
		SetTag("collection_txs", collection.Transactions)
}

// FinishCollectionToGuarantee finishes a span to trace the duration of a collection
// from being proposed to being finalized (eg. guaranteed).
// TODO not used, revisit once HotStuff is in use
func (cc *CollectionCollector) FinishCollectionToGuarantee(collectionID flow.Identifier) {
	cc.tracer.FinishSpan(collectionID, spanCollectionToGuarantee)
}
