package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type CollectionCollector struct {
	tracer               module.Tracer
	transactionsIngested prometheus.Counter       // tracks the number of ingested transactions
	finalizedHeight      *prometheus.GaugeVec     // tracks the finalized height
	proposals            *prometheus.HistogramVec // tracks the number/size of PROPOSED collections
	guarantees           *prometheus.HistogramVec // counts the number/size of FINALIZED collections
}

func NewCollectionCollector(tracer module.Tracer) *CollectionCollector {

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
		}, []string{LabelChain, LabelProposer}),
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
	collection := block.Payload.Collection.Light()

	cc.proposals.
		With(prometheus.Labels{LabelChain: block.Header.ChainID.String()}).
		Observe(float64(collection.Len()))
}

// ClusterBlockFinalized updates the guaranteed collection size gauge and
// finishes the tx->collection span for each constituent transaction.
func (cc *CollectionCollector) ClusterBlockFinalized(block *cluster.Block) {
	collection := block.Payload.Collection.Light()
	chainID := block.Header.ChainID
	proposer := block.Header.ProposerID

	cc.finalizedHeight.
		With(prometheus.Labels{LabelChain: chainID.String()}).
		Set(float64(block.Header.Height))
	cc.guarantees.
		With(prometheus.Labels{
			LabelChain:    chainID.String(),
			LabelProposer: proposer.String(),
		}).
		Observe(float64(collection.Len()))
}
