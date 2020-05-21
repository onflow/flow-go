package metrics

import (
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/trace"
)

// Collection spans.
const (

	// span from a transaction being received to being included in a block
	spanTransactionToCollection = "transaction_to_collection"

	// span from a collection being proposed to being finalized (eg. guaranteed)
	spanCollectionToGuarantee = "collection_to_guarantee"
)

type CollectionCollector struct {
	tracer               *trace.OpenTracer
	transactionsIngested prometheus.Counter       // tracks the number of ingested transactions
	finalizedHeight      *prometheus.GaugeVec     // tracks the finalized height
	proposals            *prometheus.HistogramVec // tracks the number/size of PROPOSED collections
	guarantees           *prometheus.HistogramVec // counts the number/size of FINALIZED collections
}

func NewCollectionCollector(tracer *trace.OpenTracer) *CollectionCollector {

	cc := &CollectionCollector{
		tracer: tracer,

		transactionsIngested: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceCollection,
			Name:      "ingested_transactions_total",
			Help:      "count of transactions ingested by this node",
		}),

		finalizedHeight: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Name:      "finalized_height",
			Help:      "tracks the latest finalized height",
		}, []string{LabelChain}),

		proposals: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Buckets:   []float64{5, 10, 50, 100}, //TODO(andrew) update once collection limits are known
			Name:      "proposals_size_transactions",
			Help:      "size/number of proposed collections",
		}, []string{LabelChain}),

		guarantees: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Buckets:   []float64{5, 10, 50, 100}, //TODO(andrew) update once collection limits are known
			Name:      "guarantees_size_transactions",
			Help:      "size/number of guaranteed/finalized collections",
		}, []string{LabelChain}),
	}

	return cc
}

// TransactionIngested starts a span to trace the duration of a transaction
// from being created to being included as part of a collection.
func (cc *CollectionCollector) TransactionIngested(txID flow.Identifier) {
	cc.transactionsIngested.Inc()
	cc.tracer.StartSpan(txID, spanTransactionToCollection)
}

// ClusterBlockProposed tracks the size and number of proposals, as well as
// starting the collection->guarantee span.
func (cc *CollectionCollector) ClusterBlockProposed(block *cluster.Block) {
	collection := block.Payload.Collection.Light()

	cc.proposals.
		With(prometheus.Labels{LabelChain: block.Header.ChainID}).
		Observe(float64(collection.Len()))

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

// ClusterBlockFinalized updates the guaranteed collection size gauge and
// finishes the tx->collection span for each constituent transaction.
func (cc *CollectionCollector) ClusterBlockFinalized(block *cluster.Block) {
	collection := block.Payload.Collection.Light()
	chainID := block.Header.ChainID

	cc.finalizedHeight.
		With(prometheus.Labels{LabelChain: chainID}).
		Set(float64(block.Header.Height))
	cc.guarantees.
		With(prometheus.Labels{LabelChain: block.Header.ChainID}).
		Observe(float64(collection.Len()))

	for _, txID := range collection.Transactions {
		cc.tracer.FinishSpan(txID, spanTransactionToCollection)
	}
	cc.tracer.FinishSpan(collection.ID(), spanCollectionToGuarantee)
}
