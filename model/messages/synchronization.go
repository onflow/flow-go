package messages

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// SyncRequest is part of the synchronization protocol and represents a node on
// the network sharing the height of its latest finalized block and requesting
// the same information from the recipient.
type SyncRequest struct {
	Nonce  uint64
	Height uint64
}

// SyncResponse is part of the synchronization protocol and represents the reply
// to a synchronization request that contains the latest finalized block height
// of the responding node.
type SyncResponse struct {
	Nonce  uint64
	Height uint64
}

// RangeRequest is part of the synchronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized blocks by a range of block heights, including from and to
// heights.
type RangeRequest struct {
	Nonce      uint64
	FromHeight uint64
	ToHeight   uint64
}

// RangeResponse is part of the synchronization protocol and represents the reply to
// a range request. It contains a list of blocks that should correspond to the request.
type RangeResponse struct {
	Nonce  uint64
	Blocks []*flow.Block
}

// BatchRequest is part of the sychronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized or unfinalized blocks by a list of block IDs.
type BatchRequest struct {
	Nonce    uint64
	BlockIDs []flow.Identifier
}

// BatchResponse is part of the synchronization protocol and represents the reply to
// a batch request. It contains a list of blocks that should correspond to the request.
type BatchResponse struct {
	Nonce  uint64
	Blocks []*flow.Block
}

// ClusterRangeResponse is the same thing as RangeResponse, but for cluster
// consensus.
type ClusterRangeResponse struct {
	Nonce  uint64
	Blocks []*cluster.Block
}

// ClusterBatchResponse is the same thing as BatchResponse, but for cluster
// consensus.
type ClusterBatchResponse struct {
	Nonce  uint64
	Blocks []*cluster.Block
}
