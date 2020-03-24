package messages

import (
	"github.com/dapperlabs/flow-go/model/flow"
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

// BatchRequest is part of the sychronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized or unfinalized blocks by a list of block IDs.
type BatchRequest struct {
	Nonce    uint64
	BlockIDs []flow.Identifier
}

// BlockResponse is part of the synchronization protocol and represents the
// reply to any active synchronization attempts. It contains a list of blocks
// that should correspond to the request.
type BlockResponse struct {
	Nonce  uint64
	Blocks []*flow.Block
}
