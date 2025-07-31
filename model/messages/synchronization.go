package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// SyncRequest is part of the synchronization protocol and represents a node on
// the network sharing the height of its latest finalized block and requesting
// the same information from the recipient.
// All SyncRequest messages are validated before being processed. If validation fails, then a misbehavior report is created.
// See synchronization.validateSyncRequestForALSP for more details.
//
type SyncRequest struct {
	Nonce  uint64
	Height uint64
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedSyncRequest SyncRequest

//
func NewSyncRequest(untrusted UntrustedSyncRequest) (*SyncRequest, error) {
	return &SyncRequest{
		Nonce:  untrusted.Nonce,
		Height: untrusted.Height,
	}, nil
}

// SyncResponse is part of the synchronization protocol and represents the reply
// to a synchronization request that contains the latest finalized block height
// of the responding node.
//
type SyncResponse struct {
	Nonce  uint64
	Height uint64
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedSyncResponse SyncResponse

//
func NewSyncResponse(untrusted UntrustedSyncResponse) (*SyncResponse, error) {
	return &SyncResponse{
		Nonce:  untrusted.Nonce,
		Height: untrusted.Height,
	}, nil
}

// RangeRequest is part of the synchronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized blocks by a range of block heights, including from and to
// heights.
// All RangeRequest messages are validated before being processed. If validation fails, then a misbehavior report is created.
// See synchronization.validateRangeRequestForALSP for more details.
//
type RangeRequest struct {
	Nonce      uint64
	FromHeight uint64
	ToHeight   uint64
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedRangeRequest RangeRequest

//
func NewRangeRequest(untrusted UntrustedRangeRequest) (*RangeRequest, error) {
	if untrusted.FromHeight > untrusted.ToHeight {
		return nil, fmt.Errorf("from height (%d) must not be greater than to height (%d)", untrusted.FromHeight, untrusted.ToHeight)
	}
	return &RangeRequest{
		Nonce:      untrusted.Nonce,
		FromHeight: untrusted.FromHeight,
		ToHeight:   untrusted.ToHeight,
	}, nil
}

// BatchRequest is part of the synchronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized or unfinalized blocks by a list of block IDs.
// All BatchRequest messages are validated before being processed. If validation fails, then a misbehavior report is created.
// See synchronization.validateBatchRequestForALSP for more details.
//
type BatchRequest struct {
	Nonce    uint64
	BlockIDs []flow.Identifier
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedBatchRequest BatchRequest

//
func NewBatchRequest(untrusted UntrustedBatchRequest) (*BatchRequest, error) {
	if len(untrusted.BlockIDs) == 0 {
		return nil, fmt.Errorf("block IDs must not be empty")
	}
	for i, blockID := range untrusted.BlockIDs {
		if blockID == flow.ZeroID {
			return nil, fmt.Errorf("block ID at index %d must not be zero", i)
		}
	}
	return &BatchRequest{
		Nonce:    untrusted.Nonce,
		BlockIDs: untrusted.BlockIDs,
	}, nil
}

// BlockResponse is part of the synchronization protocol and represents the
// reply to any active synchronization attempts. It contains a list of blocks
// that should correspond to the request.
//
type BlockResponse struct {
	Nonce  uint64
	Blocks []flow.Block
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedBlockResponse BlockResponse

//
func NewBlockResponse(untrusted UntrustedBlockResponse) (*BlockResponse, error) {
	for i, block := range untrusted.Blocks {
		if block.Header == nil {
			return nil, fmt.Errorf("block %d header must not be nil", i)
		}
		if block.Payload == nil {
			return nil, fmt.Errorf("block %d payload must not be nil", i)
		}
	}
	return &BlockResponse{
		Nonce:  untrusted.Nonce,
		Blocks: untrusted.Blocks,
	}, nil
}

func (br *BlockResponse) BlocksInternal() []*flow.Block {
	internal := make([]*flow.Block, len(br.Blocks))
	for i, block := range br.Blocks {
		block := block
		internal[i] = &block
	}
	return internal
}

// ClusterBlockResponse is the same thing as BlockResponse, but for cluster
// consensus.
//
type ClusterBlockResponse struct {
	Nonce  uint64
	Blocks []cluster.Block
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedClusterBlockResponse ClusterBlockResponse

//
func NewClusterBlockResponse(untrusted UntrustedClusterBlockResponse) (*ClusterBlockResponse, error) {
	for i, block := range untrusted.Blocks {
		if block.Header == nil {
			return nil, fmt.Errorf("block %d header must not be nil", i)
		}
		if block.Payload == nil {
			return nil, fmt.Errorf("block %d payload must not be nil", i)
		}
	}
	return &ClusterBlockResponse{
		Nonce:  untrusted.Nonce,
		Blocks: untrusted.Blocks,
	}, nil
}

func (br *ClusterBlockResponse) BlocksInternal() []*cluster.Block {
	internal := make([]*cluster.Block, len(br.Blocks))
	for i, block := range br.Blocks {
		block := block
		internal[i] = &block
	}
	return internal
}
