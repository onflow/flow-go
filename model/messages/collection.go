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

// ToHeader converts the untrusted block into a compact [flow.Header] representation,
// where the payload is compressed to a hash reference.
func (ub *UntrustedClusterBlock) ToHeader() *flow.Header {
	return (*cluster.Block)(ub).ToHeader()
}

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

// ToInternal converts the UntrustedClusterProposal to a trusted internal cluster.BlockProposal.
// TODO: Validate the untrusted input before converting to trusted internal representation.
func (cbp *UntrustedClusterProposal) ToInternal() *cluster.BlockProposal {
	return &cluster.BlockProposal{
		Block:           cbp.Block,
		ProposerSigData: cbp.ProposerSigData,
	}
}

func UntrustedClusterProposalFromInternal(proposal *cluster.BlockProposal) *UntrustedClusterProposal {
	return &UntrustedClusterProposal{
		Block:           proposal.Block,
		ProposerSigData: proposal.ProposerSigData,
	}
}

// ClusterBlockVote is a vote for a proposed block in collection node cluster
// consensus; effectively a vote for a particular collection.
type ClusterBlockVote BlockVote

// ClusterTimeoutObject is part of the collection cluster protocol and represents a collection node
// timing out in given round. Contains a sequential number for deduplication purposes.
type ClusterTimeoutObject TimeoutObject
