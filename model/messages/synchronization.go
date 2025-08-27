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
type SyncRequest flow.SyncRequest

// ToInternal returns the internal type representation for SyncRequest.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.SyncRequest.
func (s *SyncRequest) ToInternal() (any, error) {
	return (*flow.SyncRequest)(s), nil
}

// SyncResponse is part of the synchronization protocol and represents the reply
// to a synchronization request that contains the latest finalized block height
// of the responding node.
type SyncResponse flow.SyncResponse

// ToInternal returns the internal type representation for SyncResponse.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.SyncResponse.
func (s *SyncResponse) ToInternal() (any, error) {
	return (*flow.SyncResponse)(s), nil
}

// RangeRequest is part of the synchronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized blocks by a range of block heights, including from and to
// heights.
// All RangeRequest messages are validated before being processed. If validation fails, then a misbehavior report is created.
// See synchronization.validateRangeRequestForALSP for more details.
type RangeRequest struct {
	Nonce      uint64
	FromHeight uint64
	ToHeight   uint64
}

// ToInternal converts the untrusted RangeRequest into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (r *RangeRequest) ToInternal() (any, error) {
	// TODO(malleability, #7707) implement with validation checks
	return r, nil
}

// BatchRequest is part of the synchronization protocol and represents an active
// (pulling) attempt to synchronize with the consensus state of the network. It
// requests finalized or unfinalized blocks by a list of block IDs.
// All BatchRequest messages are validated before being processed. If validation fails, then a misbehavior report is created.
// See synchronization.validateBatchRequestForALSP for more details.
type BatchRequest flow.BatchRequest

// ToInternal returns the internal type representation for BatchRequest.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.BatchRequest.
func (b *BatchRequest) ToInternal() (any, error) {
	return (*flow.BatchRequest)(b), nil
}

// BlockResponse is part of the synchronization protocol and represents the
// reply to any active synchronization attempts. It contains a list of blocks
// that should correspond to the request.
type BlockResponse struct {
	Nonce  uint64
	Blocks []flow.UntrustedProposal
}

// ToInternal converts the untrusted BlockResponse into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (br *BlockResponse) ToInternal() (any, error) {
	// TODO(malleability, #7709) implement with validation checks
	return br, nil
}

// BlocksInternal converts all untrusted block proposals in the BlockResponse
// into trusted flow.Proposal instances.
//
// All errors indicate that the input message could not be converted to a valid proposal.
func (br *BlockResponse) BlocksInternal() ([]*flow.Proposal, error) {
	internal := make([]*flow.Proposal, len(br.Blocks))
	for i, untrusted := range br.Blocks {
		proposal, err := flow.NewProposal(untrusted)
		if err != nil {
			return nil, fmt.Errorf("could not build proposal: %w", err)
		}
		internal[i] = proposal
	}
	return internal, nil
}

// ClusterBlockResponse is the same thing as BlockResponse, but for cluster
// consensus.
type ClusterBlockResponse struct {
	Nonce  uint64
	Blocks []cluster.UntrustedProposal
}

// ToInternal converts the untrusted ClusterBlockResponse into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (br *ClusterBlockResponse) ToInternal() (any, error) {
	// TODO(malleability, #7703) implement with validation checks
	return br, nil
}

func (br *ClusterBlockResponse) BlocksInternal() ([]*cluster.Proposal, error) {
	internal := make([]*cluster.Proposal, len(br.Blocks))
	for i, untrusted := range br.Blocks {
		proposal, err := cluster.NewProposal(untrusted)
		if err != nil {
			return nil, fmt.Errorf("could not build proposal: %w", err)
		}
		internal[i] = proposal
	}
	return internal, nil
}
