package messages

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// CollectionRequest request all transactions from a collection with the given
// fingerprint.
type CollectionRequest struct {
	Fingerprint flow.Fingerprint
}

// CollectionResponse is a response to previously sent CollectionRequest which
// contains transactions payload.
type CollectionResponse struct {
	Fingerprint  flow.Fingerprint
	Transactions []flow.TransactionBody
}

// SubmitCollectionGuarantee is a request to submit the given collection
// guarantee to consensus nodes. Only valid as a node-local message.
type SubmitCollectionGuarantee struct {
	Guarantee flow.CollectionGuarantee
}
