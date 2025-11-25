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
	guarantees           *prometheus.HistogramVec // counts the number/size of FINALIZED collections
	collectionSize       *prometheus.HistogramVec // number of transactions included ONLY in the cluster blocks proposed by this node
	priorityTxns         *prometheus.HistogramVec // number of priority transactions included ONLY in cluster blocks proposed by this node
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
			Help:      "number of transactions included ONLY in the cluster blocks proposed by this node",
		}, []string{LabelChain}),

		priorityTxns: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceCollection,
			Subsystem: subsystemProposal,
			Buckets:   []float64{1, 2, 5, 10, 20},
			Name:      "priority_transactions",
			Help:      "number of priority transactions included ONLY in cluster blocks proposed by this node",
		}, []string{LabelChain}),
	}

	return cc
}

// TransactionIngested starts a span to trace the duration of a transaction
// from being created to being included as part of a collection.
func (cc *CollectionCollector) TransactionIngested(txID flow.Identifier) {
	cc.transactionsIngested.Inc()
}

// ClusterBlockFinalized updates the guaranteed collection size gauge and
// finishes the tx->collection span for each constituent transaction.
func (cc *CollectionCollector) ClusterBlockFinalized(block *cluster.Block) {
	chainID := block.ChainID.String()

	cc.finalizedHeight.
		With(prometheus.Labels{LabelChain: chainID}).
		Set(float64(block.Height))
	cc.guarantees.
		With(prometheus.Labels{
			LabelChain: chainID,
		}).
		Observe(float64(block.Payload.Collection.Len()))
}

// CollectionMaxSize measures the current maximum size of a collection.
func (cc *CollectionCollector) CollectionMaxSize(size uint) {
	cc.maxCollectionSize.Set(float64(size))
}

// ClusterBlockCreated informs about cluster block being proposed by this node.
// CAUTION: These metrics will represent a partial picture of cluster block creation across the network,
// as each node will only report on cluster blocks where they are the proposer.
// It reports several metrics, specifically how many transactions have been included and how many of them are priority txns.
func (cc *CollectionCollector) ClusterBlockCreated(block *cluster.Block, priorityTxnsCount uint) {
	chainID := block.ChainID.String()

	cc.collectionSize.
		With(prometheus.Labels{LabelChain: chainID}).
		Observe(float64(block.Payload.Collection.Len()))

	cc.priorityTxns.
		With(prometheus.Labels{LabelChain: chainID}).
		Observe(float64(priorityTxnsCount))
}
