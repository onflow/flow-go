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

// ToInternal converts the untrusted ClusterBlockVote into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (c *ClusterBlockVote) ToInternal() (any, error) {
	// TODO(malleability, #7702) implement with validation checks
	return c, nil
}

// ClusterTimeoutObject is part of the collection cluster protocol and represents a collection node
// timing out in given round. Contains a sequential number for deduplication purposes.
type ClusterTimeoutObject TimeoutObject

// ToInternal converts the untrusted ClusterTimeoutObject into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (c *ClusterTimeoutObject) ToInternal() (any, error) {
	// TODO(malleability, #7704) implement with validation checks
	return c, nil
}
