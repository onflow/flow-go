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
// Deprecated: Please replace instances of this type with cluster.Payload
type UntrustedClusterBlockPayload struct {
	Collection       flow.Collection
	ReferenceBlockID flow.Identifier
}

// Hash returns a collision-resistant hash of the UntrustedClusterBlockPayload struct.
func (p UntrustedClusterBlockPayload) Hash() flow.Identifier {
	return flow.MakeID(p)
}

// UntrustedClusterBlock is a duplicate of cluster.Block used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// cluster.Block type.
// Deprecated: Please replace instances of this type with cluster.Block
type UntrustedClusterBlock struct {
	Header  flow.HeaderBody
	Payload UntrustedClusterBlockPayload
}

// ToHeader return flow.Header data for UntrustedClusterBlock.
func (ub *UntrustedClusterBlock) ToHeader() *flow.Header {
	return &flow.Header{
		HeaderBody:  ub.Header,
		PayloadHash: ub.Payload.Hash(),
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
		Header: clusterBlock.Header,
		Payload: UntrustedClusterBlockPayload{
			ReferenceBlockID: clusterBlock.Payload.ReferenceBlockID,
			Collection:       clusterBlock.Payload.Collection,
		},
	}
}

// UntrustedClusterProposal is a proposal for a block in collection node cluster
// consensus. The header contains information about consensus state and the
// payload contains the proposed collection (may be empty).
type UntrustedClusterProposal struct {
	Block           UntrustedClusterBlock
	ProposerSigData []byte
}

func NewUntrustedClusterProposal(internal *cluster.Block, proposerSig []byte) *UntrustedClusterProposal {
	return &UntrustedClusterProposal{
		Block:           UntrustedClusterBlockFromInternal(internal),
		ProposerSigData: proposerSig,
	}
}

func (cbp *UntrustedClusterProposal) ToInternal() *cluster.BlockProposal {
	return &cluster.BlockProposal{
		Block:           cbp.Block.ToInternal(),
		ProposerSigData: cbp.ProposerSigData,
	}
}

func UntrustedClusterProposalFromInternal(proposal *cluster.BlockProposal) *UntrustedClusterProposal {
	return &UntrustedClusterProposal{
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
