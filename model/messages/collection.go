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

func (up UntrustedClusterBlockPayload) ToInternal() *cluster.Payload {
	payload := &cluster.Payload{
		ReferenceBlockID: up.ReferenceBlockID,
	}
	for _, tx := range up.Collection {
		tx := tx
		payload.Collection.Transactions = append(payload.Collection.Transactions, &tx)
	}
	return payload
}

// UntrustedClusterBlock is a duplicate of cluster.Block used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// cluster.Block type.
// Deprecated: Please update cluster.Payload.Collection to use []flow.TransactionBody,
// then replace instances of this type with cluster.Block
type UntrustedClusterBlock = GenericUntrustedBlock[*cluster.Payload]

// UntrustedClusterBlockFromInternal converts the internal cluster.Block representation
// to the representation used in untrusted messages.
func UntrustedClusterBlockFromInternal(clusterBlock *cluster.Block) UntrustedClusterBlock {
	payload := UntrustedClusterBlockPayload{
		ReferenceBlockID: clusterBlock.Payload.ReferenceBlockID,
		Collection:       make([]flow.TransactionBody, 0, clusterBlock.Payload.Collection.Len()),
	}
	for _, tx := range clusterBlock.Payload.Collection.Transactions {
		payload.Collection = append(payload.Collection, *tx)
	}

	return UntrustedClusterBlock{
		Header:  *clusterBlock.Header,
		Payload: payload,
	}
}

// ClusterBlockProposal is a proposal for a block in collection node cluster
// consensus. The header contains information about consensus state and the
// payload contains the proposed collection (may be empty).
type ClusterBlockProposal = GenericBlockProposal[*cluster.Payload]

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
