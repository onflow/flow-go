package metrics

import (
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Collection Metrics
const (
	// collectionGuaranteeToProposal is a duration metrics
	// from a collection being created
	// to being submited in a proposal
	collectionGuaranteeToProposal = "collection_guarantee_proposal"

	// collectionTransactionToCollectionGuarantee is a duration metrics
	// from a transaction received being included
	// to being included in a collection guarantee
	collectionTransactionToCollectionGuarantee = "collection_transaction_to_collection_guarantee"
)

var (
	collectionsPerBlock = promauto.NewGauge(prometheus.GaugeOpts{
		Name:      "collections_per_block",
		Namespace: "consensus",
		Help:      "the number of collections per block",
	})
	collectionsPerFinalizedBlockCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "collections_per_finalized_block",
		Namespace: "consensus",
		Help:      "The number of collections included in the finalized block",
	})
)

// StartCollectionToGuarantee starts a span to trace the duration of a collection
// from being created to being submitted as a colleciton guarantee
func (c *Collector) StartCollectionToGuarantee(collection flow.LightCollection) {
	followsFrom := make([]opentracing.StartSpanOption, 0, len(collection.Transactions))
	for _, txID := range collection.Transactions {
		if txSpan, exists := c.tracer.GetSpan(txID, collectionTransactionToCollectionGuarantee); exists {
			// link its transactions' spans
			followsFrom = append(followsFrom, opentracing.FollowsFrom(txSpan.Context()))
		}
	}

	c.tracer.StartSpan(collection.ID(), collectionGuaranteeToProposal, followsFrom...).
		SetTag("collection_id", collection.ID().String()).
		SetTag("collection_txs", collection.Transactions)
}

// FinishCollectionToGuarantee finishes a span to trace the duration of a collection
// from being created to being submitted as a colleciton guarantee
func (c *Collector) FinishCollectionToGuarantee(collectionID flow.Identifier) {
	c.tracer.FinishSpan(collectionID, collectionGuaranteeToProposal)
}

// StartTransactionToCollectionGuarantee starts a span to trace the duration of a transaction
// from being created to being included as part of a collection guarantee
func (c *Collector) StartTransactionToCollectionGuarantee(txID flow.Identifier) {
	c.tracer.StartSpan(txID, collectionTransactionToCollectionGuarantee)
}

// FinishTransactionToCollectionGuarantee finishes a span to trace the duration of a transaction
// from being created to being included as part of a collection guarantee
func (c *Collector) FinishTransactionToCollectionGuarantee(txID flow.Identifier) {
	c.tracer.FinishSpan(txID, collectionTransactionToCollectionGuarantee)
}
