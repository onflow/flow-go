package collection

import (
	"github.com/onflow/flow-go/model/flow"
)

// GuaranteedCollectionPublisher defines the interface to send collection guarantees
// from a collection node to consensus nodes. Collection guarantees are broadcast on a best-effort basis,
// and it is acceptable to discard some guarantees (especially those that are out of date).
// Implementation is non-blocking and concurrency safe.
type GuaranteedCollectionPublisher interface {
	// SubmitCollectionGuarantee adds a guarantee to an internal queue
	// to be published to consensus nodes.
	SubmitCollectionGuarantee(guarantee *flow.CollectionGuarantee)
}
