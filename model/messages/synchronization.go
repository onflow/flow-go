package messages

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type RequestWrapper struct {
	RequestID      uint64
	RequestPayload interface{}
}

type ResponseWrapper struct {
	RequestID       uint64
	ResponsePayload interface{}
}

// SyncRequest is part of the synchronization protocol and represents a node on
// the network requesting the latest finalized block from the recipient.
type SyncRequest struct{}

// SyncResponse is part of the synchronization protocol and represents the reply
// to a synchronization request that contains the latest finalized block of the
// responding node.
type SyncResponse struct {
	Height  uint64
	BlockID flow.Identifier
}

// RangeRequest is part of the synchronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized blocks by a range of block heights, including from and to
// heights.
type RangeRequest struct {
	FromHeight uint64
	ToHeight   uint64
}

// BatchRequest is part of the sychronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized or unfinalized blocks by a list of block IDs.
type BatchRequest struct {
	BlockIDs []flow.Identifier
}

// BlockResponse is part of the synchronization protocol and represents the
// reply to any active synchronization attempts. It contains a list of blocks
// that should correspond to the request.
type BlockResponse struct {
	Blocks []*flow.Block
}

// ClusterBlockResponse is the same thing as BlockResponse, but for cluster
// consensus.
type ClusterBlockResponse struct {
	Blocks []*cluster.Block
}
