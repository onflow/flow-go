package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type CollectionCollector struct {
	module.TransactionValidationMetrics
	tracer               module.Tracer
	transactionsIngested prometheus.Counter       // tracks the number of ingested transactions
	finalizedHeight      *prometheus.GaugeVec     // tracks the finalized height
	maxCollectionSize    prometheus.Gauge         // tracks the maximum collection size
	proposals            *prometheus.HistogramVec // tracks the number/size of PROPOSED collections
	guarantees           *prometheus.HistogramVec // counts the number/size of FINALIZED collections
	collectionSize       *prometheus.HistogramVec
	priorityTxns         *prometheus.HistogramVec
}

var _ module.CollectionMetrics = (*CollectionCollector)(nil)

func NewCollectionCollector(tracer module.Tracer) *CollectionCollector {

	cc := &CollectionCollector{
		TransactionValidationMetrics: NewTransactionValidationCollector(),
		tracer:                       tracer,
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

		maxCollectionSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Name:      "max_collection_size",
			Help:      "last used max collection size",
		}),

		proposals: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Buckets:   []float64{1, 2, 5, 10, 20},
			Name:      "proposals_size_transactions",
			Help:      "size/number of proposed collections",
		}, []string{LabelChain}),

		guarantees: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Buckets:   []float64{1, 2, 5, 10, 20},
			Name:      "guarantees_size_transactions",
			Help:      "size/number of guaranteed/finalized collections",
		}, []string{LabelChain}),

		collectionSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Buckets:   []float64{1, 2, 5, 10, 20},
			Name:      "collection_size",
			Help:      "number of transactions included in the block",
		}, []string{LabelChain}),

		priorityTxns: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Buckets:   []float64{1, 2, 5, 10, 20},
			Name:      "priority_transactions_count",
			Help:      "number of priority transactions included in the block",
		}, []string{LabelChain}),
	}

	return cc
}

// TransactionIngested starts a span to trace the duration of a transaction
// from being created to being included as part of a collection.
func (cc *CollectionCollector) TransactionIngested(txID flow.Identifier) {
	cc.transactionsIngested.Inc()
}

// ClusterBlockProposed tracks the size and number of proposals, as well as
// starting the collection->guarantee span.
func (cc *CollectionCollector) ClusterBlockProposed(block *cluster.Block) {
	cc.proposals.
		With(prometheus.Labels{LabelChain: block.ChainID.String()}).
		Observe(float64(block.Payload.Collection.Len()))
}

// ClusterBlockFinalized updates the guaranteed collection size gauge and
// finishes the tx->collection span for each constituent transaction.
func (cc *CollectionCollector) ClusterBlockFinalized(block *cluster.Block) {
	chainID := block.ChainID

	cc.finalizedHeight.
		With(prometheus.Labels{LabelChain: chainID.String()}).
		Set(float64(block.Height))
	cc.guarantees.
		With(prometheus.Labels{
			LabelChain: chainID.String(),
		}).
		Observe(float64(block.Payload.Collection.Len()))
}

// CollectionMaxSize measures the current maximum size of a collection.
func (cc *CollectionCollector) CollectionMaxSize(size uint) {
	cc.maxCollectionSize.Set(float64(size))
}

// ClusterBlockCreated informs about cluster block being created.
// It reports several metrics, specifically how many transactions have been included and how many of them are priority txns.
func (cc *CollectionCollector) ClusterBlockCreated(block *cluster.Block, priorityTxnsCount uint) {
	chainID := block.ChainID

	cc.collectionSize.
		With(prometheus.Labels{LabelChain: chainID.String()}).
		Observe(float64(block.Payload.Collection.Len()))

	cc.priorityTxns.
		With(prometheus.Labels{LabelChain: chainID.String()}).
		Observe(float64(priorityTxnsCount))
}
