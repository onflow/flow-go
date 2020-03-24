package trace

import (
	"github.com/opentracing/opentracing-go"

	"github.com/dapperlabs/flow-go/model/flow"
)

// StartCollectionSpan starts a collection span, and returns the span for chaining
func StartCollectionSpan(tracer Tracer, collection *flow.LightCollection) opentracing.Span {
	followsFrom := make([]opentracing.StartSpanOption, 0, len(collection.Transactions))
	for _, txID := range collection.Transactions {
		if txSpan, exists := tracer.GetSpan(txID); exists {
			followsFrom = append(followsFrom, opentracing.FollowsFrom(txSpan.Context()))
		}
	}

	return tracer.StartSpan(collection.ID(), "collectionGuaranteeProposal", followsFrom...).
		SetTag("collection_id", collection.ID().String()).
		SetTag("collection_txs", collection.Transactions)
}
