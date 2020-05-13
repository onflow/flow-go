package messages

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
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

// ClusterBlockProposal is a proposal for a block in collection node cluster
// consensus. The header contains information about consensus state and the
// payload contains the proposed collection (may be empty).
type ClusterBlockProposal struct {
	Header  *flow.Header
	Payload *cluster.Payload
}

// ClusterBlockVote is a vote for a proposed block in collection node cluster
// consensus; effectively a vote for a particular collection.
type ClusterBlockVote struct {
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}

// ClusterBlockRequest is a request for a block in collection node cluster
// consensus; effectively a request for a particular collection collection and
// the associated consensus information.
type ClusterBlockRequest struct {
	BlockID flow.Identifier
	Nonce   uint64
}

// ClusterBlockResponse  is the response to a collection request. It contains
// the block proposing the collection.
type ClusterBlockResponse struct {
	Proposal *ClusterBlockProposal
	Nonce    uint64
}
