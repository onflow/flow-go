package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// ClusterProposal is a signed cluster block proposal in collection node cluster consensus.
type ClusterProposal cluster.UntrustedProposal

// ToInternal returns the internal type representation for ClusterProposal.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal cluster.Proposal.
func (p *ClusterProposal) ToInternal() (any, error) {
	internal, err := cluster.NewProposal(cluster.UntrustedProposal(*p))
	if err != nil {
		return nil, fmt.Errorf("could not convert %T to internal type: %w", p, err)
	}
	return internal, nil
}

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
type ClusterBlockVote flow.BlockVote

// ToInternal converts the untrusted ClusterBlockVote into its trusted internal
// representation.
func (c *ClusterBlockVote) ToInternal() (any, error) {
	if c.BlockID == flow.ZeroID {
		return nil, fmt.Errorf("BlockID must not be empty")
	}

	if len(c.SigData) == 0 {
		return nil, fmt.Errorf("SigData must not be empty")
	}

	return (*flow.BlockVote)(c), nil
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

// CollectionGuarantee is a message representation of an CollectionGuarantee, which is used
// to announce collections to consensus nodes.
type CollectionGuarantee flow.UntrustedCollectionGuarantee

// ToInternal returns the internal type representation for CollectionGuarantee.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.CollectionGuarantee.
func (c *CollectionGuarantee) ToInternal() (any, error) {
	internal, err := flow.NewCollectionGuarantee(flow.UntrustedCollectionGuarantee(*c))
	if err != nil {
		return nil, fmt.Errorf("could not construct guarantee: %w", err)
	}
	return internal, nil
}
