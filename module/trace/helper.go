package trace

import (
	"github.com/dapperlabs/flow-go/model/flow"
	opentracing "github.com/opentracing/opentracing-go"
)

// StartCollectionGuaranteeSpan starts a collection guarantee span, and returns the span for chaining
func StartCollectionGuaranteeSpan(tracer Tracer, guarantee flow.CollectionGuarantee, transactions []*flow.Transaction) opentracing.Span {
	followsFrom := make([]opentracing.StartSpanOption, 0, len(transactions))
	txIDs := make([]flow.Identifier, 0, len(transactions))
	for _, tx := range transactions {
		if txSpan, exists := tracer.GetSpan(tx.ID()); exists {
			followsFrom = append(followsFrom, opentracing.FollowsFrom(txSpan.Context()))
		}
		txIDs = append(txIDs, tx.ID())
	}
	return tracer.StartSpan(guarantee.ID(), "collectionGuaranteeProposal", followsFrom...).
		SetTag("guarantee_id", guarantee.ID().String()).
		SetTag("collection_txs", txIDs)
}
