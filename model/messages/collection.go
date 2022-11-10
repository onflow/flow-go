package messages

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// SubmitCollectionGuarantee is a request to submit the given collection
// guarantee to consensus nodes. Only valid as a node-local message.
type SubmitCollectionGuarantee struct {
	Guarantee flow.CollectionGuarantee
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

type UntrustedClusterBlockPayload struct {
	Collection       []flow.TransactionBody
	ReferenceBlockID flow.Identifier
}

type UntrustedClusterBlock struct {
	Header  flow.Header
	Payload UntrustedClusterBlockPayload
}

func (ub UntrustedClusterBlock) ToInternal() *cluster.Block {
	block := &cluster.Block{
		Header: &ub.Header,
		Payload: &cluster.Payload{
			Collection: flow.Collection{
				Transactions: make([]*flow.TransactionBody, 0, len(ub.Payload.Collection)),
			},
			ReferenceBlockID: ub.Payload.ReferenceBlockID,
		},
	}
	for _, tx := range ub.Payload.Collection {
		block.Payload.Collection.Transactions = append(block.Payload.Collection.Transactions, &tx)
	}
	return block
}

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
type ClusterBlockProposal struct {
	Block UntrustedClusterBlock
}

func NewClusterBlockProposal(internal *cluster.Block) *ClusterBlockProposal {
	return &ClusterBlockProposal{
		Block: UntrustedClusterBlockFromInternal(internal),
	}
}

// ClusterBlockVote is a vote for a proposed block in collection node cluster
// consensus; effectively a vote for a particular collection.
type ClusterBlockVote struct {
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}
