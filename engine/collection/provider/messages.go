package provider

import "github.com/dapperlabs/flow-go/model/flow"

// CollectionRequest request all transactions from a collection with given fingerprint
type CollectionRequest struct {
	Fingerprint flow.Fingerprint
}

// CollectionResponse is a response to previously sent CollectionRequest which contains transactions payload
type CollectionResponse struct {
	Fingerprint  flow.Fingerprint
	Transactions []flow.TransactionBody
}
