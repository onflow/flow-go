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

// UntrustedClusterBlockPayload is a duplicate of cluster.Payload used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// cluster.Payload type.
// Deprecated: Please update cluster.Payload.Collection to use flow.Collection,
// then replace instances of this type with cluster.Payload
type UntrustedClusterBlockPayload struct {
	Collection       flow.Collection
	ReferenceBlockID flow.Identifier
}

// Hash returns the hash of the payload.
func (p UntrustedClusterBlockPayload) Hash() flow.Identifier {
	return flow.MakeID(p)
}

// UntrustedClusterBlock is a duplicate of cluster.Block used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// cluster.Block type.
// Deprecated: Please update cluster.Payload.Collection to use []flow.TransactionBody,
// then replace instances of this type with cluster.Block
type UntrustedClusterBlock struct {
	Header  flow.HeaderFields
	Payload UntrustedClusterBlockPayload
}

// ToHeader return flow.Header data for UntrustedClusterBlock.
func (ub *UntrustedClusterBlock) ToHeader() *flow.Header {
	return &flow.Header{
		ChainID:            ub.Header.ChainID,
		ParentID:           ub.Header.ParentID,
		Height:             ub.Header.Height,
		Timestamp:          ub.Header.Timestamp,
		View:               ub.Header.View,
		ParentView:         ub.Header.ParentView,
		ParentVoterIndices: ub.Header.ParentVoterIndices,
		ParentVoterSigData: ub.Header.ParentVoterSigData,
		ProposerID:         ub.Header.ProposerID,
		LastViewTC:         ub.Header.LastViewTC,
		PayloadHash:        ub.Payload.Hash(),
	}
}

// ToInternal returns the internal representation of the type.
func (ub *UntrustedClusterBlock) ToInternal() *cluster.Block {
	return cluster.NewBlock(
		ub.Header,
		cluster.Payload{
			ReferenceBlockID: ub.Payload.ReferenceBlockID,
			Collection:       ub.Payload.Collection,
		},
	)
}

// UntrustedClusterBlockFromInternal converts the internal cluster.Block representation
// to the representation used in untrusted messages.
func UntrustedClusterBlockFromInternal(clusterBlock *cluster.Block) UntrustedClusterBlock {
	return UntrustedClusterBlock{
		Header: *clusterBlock.Header,
		Payload: UntrustedClusterBlockPayload{
			ReferenceBlockID: clusterBlock.Payload.ReferenceBlockID,
			Collection:       clusterBlock.Payload.Collection,
		},
	}
}

// ClusterBlockProposal is a proposal for a block in collection node cluster
// consensus. The header contains information about consensus state and the
// payload contains the proposed collection (may be empty).
type ClusterBlockProposal struct {
	Block           UntrustedClusterBlock
	ProposerSigData []byte
}

func NewClusterBlockProposal(internal *cluster.Block, proposerSig []byte) *ClusterBlockProposal {
	return &ClusterBlockProposal{
		Block:           UntrustedClusterBlockFromInternal(internal),
		ProposerSigData: proposerSig,
	}
}

func (cbp *ClusterBlockProposal) ToInternal() *cluster.BlockProposal {
	return &cluster.BlockProposal{
		Block:           cbp.Block.ToInternal(),
		ProposerSigData: cbp.ProposerSigData,
	}
}

func ClusterBlockProposalFrom(proposal *cluster.BlockProposal) *ClusterBlockProposal {
	return &ClusterBlockProposal{
		Block:           UntrustedClusterBlockFromInternal(proposal.Block),
		ProposerSigData: proposal.ProposerSigData,
	}
}

// ClusterBlockVote is a vote for a proposed block in collection node cluster
// consensus; effectively a vote for a particular collection.
type ClusterBlockVote BlockVote

// ClusterTimeoutObject is part of the collection cluster protocol and represents a collection node
// timing out in given round. Contains a sequential number for deduplication purposes.
type ClusterTimeoutObject TimeoutObject
