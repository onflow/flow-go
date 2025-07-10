package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
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

// UntrustedClusterBlockPayload is a duplicate of cluster.Payload used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// cluster.Payload type.
// Deprecated: Please update cluster.Payload.Collection to use []flow.TransactionBody,
// then replace instances of this type with cluster.Payload
type UntrustedClusterBlockPayload struct {
	Collection       []flow.TransactionBody
	ReferenceBlockID flow.Identifier
}

// UntrustedClusterBlock is a duplicate of cluster.Block used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// cluster.Block type.
// Deprecated: Please update cluster.Payload.Collection to use []flow.TransactionBody,
// then replace instances of this type with cluster.Block
type UntrustedClusterBlock struct {
	Header  flow.Header
	Payload UntrustedClusterBlockPayload
}

// ToInternal returns the internal representation of the type.
func (ub *UntrustedClusterBlock) ToInternal() *cluster.Block {
	block := &cluster.Block{
		Header: &ub.Header,
		Payload: &cluster.Payload{
			ReferenceBlockID: ub.Payload.ReferenceBlockID,
		},
	}
	for _, tx := range ub.Payload.Collection {
		tx := tx
		block.Payload.Collection.Transactions = append(block.Payload.Collection.Transactions, &tx)
	}
	return block
}

// UntrustedClusterBlockFromInternal converts the internal cluster.Block representation
// to the representation used in untrusted messages.
func UntrustedClusterBlockFromInternal(clusterBlock *cluster.Block) UntrustedClusterBlock {
	block := UntrustedClusterBlock{
		Header: *clusterBlock.Header,
		Payload: UntrustedClusterBlockPayload{
			ReferenceBlockID: clusterBlock.Payload.ReferenceBlockID,
			Collection:       make([]flow.TransactionBody, 0, clusterBlock.Payload.Collection.Len()),
		},
	}
	for _, tx := range clusterBlock.Payload.Collection.Transactions {
		block.Payload.Collection = append(block.Payload.Collection, *tx)
	}
	return block
}

// ClusterBlockProposal is a proposal for a block in collection node cluster
// consensus. The header contains information about consensus state and the
// payload contains the proposed collection (may be empty).
//
//structwrite:immutable
type ClusterBlockProposal struct {
	Block *cluster.Block
}

// UntrustedClusterBlockProposal is an untrusted input-only representation of a ClusterBlockProposal,
// used for construction.
//
// An instance of UntrustedClusterBlockProposal should be validated and converted into
// a trusted ClusterBlockProposal using NewClusterBlockProposal constructor.
type UntrustedClusterBlockProposal ClusterBlockProposal

// NewClusterBlockProposal creates a new instance of ClusterBlockProposal.
//
// Parameters:
//   - untrusted: untrusted ClusterBlockProposal to be validated
//
// Returns:
//   - *ClusterBlockProposal: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewClusterBlockProposal(untrusted UntrustedClusterBlockProposal) (*ClusterBlockProposal, error) {
	// TODO: add validation logic
	if untrusted.Block == nil {
		return nil, fmt.Errorf("block must not be nil")
	}
	return &ClusterBlockProposal{Block: untrusted.Block}, nil
}

func NewClusterBlockProposalFromInternal(internal *cluster.Block) *ClusterBlockProposal {
	return &ClusterBlockProposal{Block: internal}
}

// ClusterBlockVote is a vote for a proposed block in collection node cluster
// consensus; effectively a vote for a particular collection.
//
//structwrite:immutable
type ClusterBlockVote BlockVote

// UntrustedClusterBlockVote is an untrusted input-only representation of a ClusterBlockVote,
// used for construction.
//
// An instance of UntrustedClusterBlockVote should be validated and converted into
// a trusted ClusterBlockVote using NewClusterBlockVote constructor.
type UntrustedClusterBlockVote ClusterBlockVote

// NewClusterBlockVote creates a new instance of ClusterBlockVote.
//
// Parameters:
//   - untrusted: untrusted ClusterBlockVote to be validated
//
// Returns:
//   - *ClusterBlockVote: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewClusterBlockVote(untrusted UntrustedClusterBlockVote) (*ClusterBlockVote, error) {
	// TODO: add validation logic
	vote, err := NewBlockVote(UntrustedBlockVote(untrusted))
	if err != nil {
		return nil, err
	}
	cv := ClusterBlockVote(*vote)
	return &cv, nil
}

// ClusterTimeoutObject is part of the collection cluster protocol and represents a collection node
// timing out in given round. Contains a sequential number for deduplication purposes.
//
//structwrite:immutable
type ClusterTimeoutObject TimeoutObject

// UntrustedClusterTimeoutObject is an untrusted input-only representation of a ClusterTimeoutObject,
// used for construction.
//
// An instance of UntrustedClusterTimeoutObject should be validated and converted into
// a trusted ClusterTimeoutObject using NewClusterTimeoutObject constructor.
type UntrustedClusterTimeoutObject ClusterTimeoutObject

// NewClusterTimeoutObject creates a new instance of ClusterTimeoutObject.
//
// Parameters:
//   - untrusted: untrusted ClusterTimeoutObject to be validated
//
// Returns:
//   - *ClusterTimeoutObject: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewClusterTimeoutObject(untrusted UntrustedClusterTimeoutObject) (*ClusterTimeoutObject, error) {
	// TODO: add validation logic
	to, err := NewTimeoutObject(UntrustedTimeoutObject(untrusted))
	if err != nil {
		return nil, err
	}
	cto := ClusterTimeoutObject(*to)
	return &cto, nil
}
