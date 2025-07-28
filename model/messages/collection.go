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

// UntrustedClusterProposal represents untrusted signed proposed block in collection node cluster consensus.
// This type exists only to explicitly differentiate between trusted and untrusted instances of a cluster block proposal.
// This differentiation is currently largely unused, but eventually untrusted models should use
// a different type (like this one), until such time as they are fully validated.
type UntrustedClusterProposal cluster.Proposal

func NewUntrustedClusterProposal(internal cluster.UnsignedBlock, proposerSig []byte) *UntrustedClusterProposal {
	return &UntrustedClusterProposal{
		Block:           internal,
		ProposerSigData: proposerSig,
	}
}

// DeclareStructurallyValid converts the UntrustedClusterProposal to a trusted internal cluster.Proposal.
// CAUTION: Prior to using this function, ensure that the untrusted proposal has been fully validated.
func (cbp *UntrustedClusterProposal) DeclareStructurallyValid() (*cluster.Proposal, error) {
	block, err := cluster.NewUnsignedBlock(
		cluster.UntrustedUnsignedBlock{
			HeaderBody: cbp.Block.HeaderBody,
			Payload:    cbp.Block.Payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not build cluster block: %w", err)
	}

	// validate ProposerSigData
	if len(cbp.ProposerSigData) == 0 {
		return nil, fmt.Errorf("proposer signature must not be empty")
	}
	return &cluster.Proposal{
		Block:           *block,
		ProposerSigData: cbp.ProposerSigData,
	}, nil
}

func UntrustedClusterProposalFromInternal(proposal *cluster.Proposal) *UntrustedClusterProposal {
	p := UntrustedClusterProposal(*proposal)
	return &p
}

// ClusterBlockVote is a vote for a proposed block in collection node cluster
// consensus; effectively a vote for a particular collection.
type ClusterBlockVote BlockVote

// ClusterTimeoutObject is part of the collection cluster protocol and represents a collection node
// timing out in given round. Contains a sequential number for deduplication purposes.
type ClusterTimeoutObject TimeoutObject
