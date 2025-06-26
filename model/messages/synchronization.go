package messages

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// SyncRequest is part of the synchronization protocol and represents a node on
// the network sharing the height of its latest finalized block and requesting
// the same information from the recipient.
// All SyncRequest messages are validated before being processed. If validation fails, then a misbehavior report is created.
// See synchronization.validateSyncRequestForALSP for more details.
//
//structwrite:immutable
type SyncRequest struct {
	Nonce  uint64
	Height uint64
}

// UntrustedSyncRequest is an untrusted input-only representation of a SyncRequest,
// used for construction.
//
// An instance of UntrustedSyncRequest should be validated and converted into
// a trusted SyncRequest using NewSyncRequest constructor.
type UntrustedSyncRequest SyncRequest

// NewSyncRequest creates a new instance of SyncRequest.
//
// Parameters:
//   - untrusted: untrusted SyncRequest to be validated
//
// Returns:
//   - *SyncRequest: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewSyncRequest(untrusted UntrustedSyncRequest) (*SyncRequest, error) {
	// TODO: add validation logic
	return &SyncRequest{Nonce: untrusted.Nonce, Height: untrusted.Height}, nil
}

// SyncResponse is part of the synchronization protocol and represents the reply
// to a synchronization request that contains the latest finalized block height
// of the responding node.
//
//structwrite:immutable
type SyncResponse struct {
	Nonce  uint64
	Height uint64
}

// UntrustedSyncResponse is an untrusted input-only representation of a SyncResponse,
// used for construction.
//
// An instance of UntrustedSyncResponse should be validated and converted into
// a trusted SyncResponse using NewSyncResponse constructor.
type UntrustedSyncResponse SyncResponse

// NewSyncResponse creates a new instance of SyncResponse.
//
// Parameters:
//   - untrusted: untrusted SyncResponse to be validated
//
// Returns:
//   - *SyncResponse: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewSyncResponse(untrusted UntrustedSyncResponse) (*SyncResponse, error) {
	// TODO: add validation logic
	return &SyncResponse{Nonce: untrusted.Nonce, Height: untrusted.Height}, nil
}

// RangeRequest is part of the synchronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized blocks by a range of block heights, including from and to
// heights.
// All RangeRequest messages are validated before being processed. If validation fails, then a misbehavior report is created.
// See synchronization.validateRangeRequestForALSP for more details.
//
//structwrite:immutable
type RangeRequest struct {
	Nonce      uint64
	FromHeight uint64
	ToHeight   uint64
}

// UntrustedRangeRequest is an untrusted input-only representation of a RangeRequest,
// used for construction.
//
// An instance of UntrustedRangeRequest should be validated and converted into
// a trusted RangeRequest using NewRangeRequest constructor.
type UntrustedRangeRequest RangeRequest

// NewRangeRequest creates a new instance of RangeRequest.
//
// Parameters:
//   - untrusted: untrusted RangeRequest to be validated
//
// Returns:
//   - *RangeRequest: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewRangeRequest(untrusted UntrustedRangeRequest) (*RangeRequest, error) {
	// TODO: add validation logic
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
//structwrite:immutable
type BatchRequest struct {
	Nonce    uint64
	BlockIDs []flow.Identifier
}

// UntrustedBatchRequest is an untrusted input-only representation of a BatchRequest,
// used for construction.
//
// An instance of UntrustedBatchRequest should be validated and converted into
// a trusted BatchRequest using NewBatchRequest constructor.
type UntrustedBatchRequest BatchRequest

// NewBatchRequest creates a new instance of BatchRequest.
//
// Parameters:
//   - untrusted: untrusted BatchRequest to be validated
//
// Returns:
//   - *BatchRequest: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewBatchRequest(untrusted UntrustedBatchRequest) (*BatchRequest, error) {
	// TODO: add validation logic
	return &BatchRequest{Nonce: untrusted.Nonce, BlockIDs: untrusted.BlockIDs}, nil
}

// BlockResponse is part of the synchronization protocol and represents the
// reply to any active synchronization attempts. It contains a list of blocks
// that should correspond to the request.
//
//structwrite:immutable
type BlockResponse struct {
	Nonce  uint64
	Blocks []*flow.Block
}

// UntrustedBlockResponse is an untrusted input-only representation of a BlockResponse,
// used for construction.
//
// An instance of UntrustedBlockResponse should be validated and converted into
// a trusted BlockResponse using NewBlockResponse constructor.
type UntrustedBlockResponse BlockResponse

// NewBlockResponse creates a new instance of BlockResponse.
//
// Parameters:
//   - untrusted: untrusted BlockResponse to be validated
//
// Returns:
//   - *BlockResponse: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewBlockResponse(untrusted UntrustedBlockResponse) (*BlockResponse, error) {
	// TODO: add validation logic
	return &BlockResponse{Nonce: untrusted.Nonce, Blocks: untrusted.Blocks}, nil
}

func (br *BlockResponse) BlocksInternal() []*flow.Block {
	return br.Blocks
}

// ClusterBlockResponse is the same thing as BlockResponse, but for cluster
// consensus.
//
//structwrite:immutable
type ClusterBlockResponse struct {
	Nonce  uint64
	Blocks []*cluster.Block
}

// UntrustedClusterBlockResponse is an untrusted input-only representation of a ClusterBlockResponse,
// used for construction.
//
// An instance of UntrustedClusterBlockResponse should be validated and converted into
// a trusted ClusterBlockResponse using NewClusterBlockResponse constructor.
type UntrustedClusterBlockResponse ClusterBlockResponse

// NewClusterBlockResponse creates a new instance of ClusterBlockResponse.
//
// Parameters:
//   - untrusted: untrusted ClusterBlockResponse to be validated
//
// Returns:
//   - *ClusterBlockResponse: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewClusterBlockResponse(untrusted UntrustedClusterBlockResponse) (*ClusterBlockResponse, error) {
	// TODO: add validation logic
	return &ClusterBlockResponse{Nonce: untrusted.Nonce, Blocks: untrusted.Blocks}, nil
}

func (br *ClusterBlockResponse) BlocksInternal() []*cluster.Block {
	return br.Blocks
}
