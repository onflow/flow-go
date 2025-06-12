package messages

import (
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

// UntrustedClusterBlock represents untrusted cluster block models received over the network.
// This type exists only to explicitly differentiate between trusted and untrusted instances of a cluster block.
// This differentiation is currently largely unused, but eventually untrusted models should use
// a different type (like this one), until such time as they are fully validated.
type UntrustedClusterBlock cluster.Block

// UntrustedClusterProposal represents untrusted signed proposed block in collection node cluster consensus.
// This type exists only to explicitly differentiate between trusted and untrusted instances of a cluster block proposal.
// This differentiation is currently largely unused, but eventually untrusted models should use
// a different type (like this one), until such time as they are fully validated.
type UntrustedClusterProposal cluster.BlockProposal

func NewUntrustedClusterProposal(internal cluster.Block, proposerSig []byte) *UntrustedClusterProposal {
	return &UntrustedClusterProposal{
		Block:           internal,
		ProposerSigData: proposerSig,
	}
}

// DeclareTrusted converts the UntrustedClusterProposal to a trusted internal cluster.BlockProposal.
// CAUTION: Prior to using this function, ensure that the untrusted proposal has been fully validated.
// TODO(malleability immutable, #7277): This conversion should eventually be accompanied by a full validation of the untrusted input.
func (cbp *UntrustedClusterProposal) DeclareTrusted() *cluster.BlockProposal {
	return &cluster.BlockProposal{
		Block:           cluster.NewBlock(cbp.Block.Header, cbp.Block.Payload),
		ProposerSigData: cbp.ProposerSigData,
	}
}

func UntrustedClusterProposalFromInternal(proposal *cluster.BlockProposal) *UntrustedClusterProposal {
	p := UntrustedClusterProposal(*proposal)
	return &p
}

// ClusterBlockVote is a vote for a proposed block in collection node cluster
// consensus; effectively a vote for a particular collection.
type ClusterBlockVote BlockVote

// ClusterTimeoutObject is part of the collection cluster protocol and represents a collection node
// timing out in given round. Contains a sequential number for deduplication purposes.
type ClusterTimeoutObject TimeoutObject
