package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// CollectionRequest request all transactions from a collection with the given
// fingerprint.
type CollectionRequest struct {
	ID    flow.Identifier
	Nonce uint64 // so that we aren't deduplicated by the network layer
}

// CollectionResponse is a response to a request for a collection.
type CollectionResponse struct {
	Collection flow.Collection
	Nonce      uint64 // so that we aren't deduplicated by the network layer
}

// ClusterBlockVote is a vote for a proposed block in collection node cluster
// consensus; effectively a vote for a particular collection.
type ClusterBlockVote BlockVote

var _ UntrustedMessage = (*ClusterBlockVote)(nil)

func (c ClusterBlockVote) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return c, nil
}

// ClusterTimeoutObject is part of the collection cluster protocol and represents a collection node
// timing out in given round. Contains a sequential number for deduplication purposes.
type ClusterTimeoutObject TimeoutObject

var _ UntrustedMessage = (*ClusterTimeoutObject)(nil)

func (c *ClusterTimeoutObject) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return c, nil
}

// CollectionGuarantee is the wire form. It’s a defined type so we can add methods here.
type CollectionGuarantee flow.UntrustedCollectionGuarantee

var _ UntrustedMessage = (*CollectionGuarantee)(nil)

func (cg *CollectionGuarantee) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return cg, nil
}

type TransactionBody flow.UntrustedTransactionBody

var _ UntrustedMessage = (*TransactionBody)(nil)

func (tb *TransactionBody) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return tb, nil
}

type Transaction flow.UntrustedTransaction

var _ UntrustedMessage = (*Transaction)(nil)

func (t *Transaction) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return t, nil
}
