// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// State represents the full protocol state of the local node. It allows us to
// obtain snapshots of the state at any point of the protocol state history.
type State interface {

	// Params gives access to a number of stable parameters of the protocol state.
	Params() Params

	// Final returns the snapshot of the persistent protocol state at the latest
	// finalized block, and the returned snapshot is therefore immutable over
	// time.
	Final() Snapshot

	// Sealed returns the snapshot of the persistent protocol state at the
	// latest sealed block, and the returned snapshot is therefore immutable
	// over time.
	Sealed() Snapshot

	// AtHeight returns the snapshot of the persistent protocol state at the
	// given block number. It is only available for finalized blocks and the
	// returned snapshot is therefore immutable over time.
	AtHeight(height uint64) Snapshot

	// AtBlockID returns the snapshot of the persistent protocol state at the
	// given block ID. It is available for any block that was introduced into
	// the protocol state, and can thus represent an ambiguous state that was or
	// will never be finalized.
	AtBlockID(blockID flow.Identifier) Snapshot
}

type FollowerState interface {
	State
	// ExtendCertified introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows
	// us to execute fork-aware queries against ambiguous protocol state, while
	// still checking that the given block is a valid extension of the protocol state.
	// Caller must pass a QC for candidate block to prove that candidate block
	// has been certified, and it's safe to add it to the block state.
	// QC cannot be nil and must certify candidate block (candidate.View == qc.View && candidate.BlockID == qc.BlockID)
	// Expected errors during normal operations:
	//  * state.OutdatedExtensionError if the candidate block is outdated (e.g. orphaned)
	//  * state.InvalidExtensionError if the candidate block is invalid
	ExtendCertified(ctx context.Context, candidate *flow.Block, qc *flow.QuorumCertificate) error

	// Finalize finalizes the block with the given hash.
	// At this level, we can only finalize one block at a time. This implies
	// that the parent of the pending block that is to be finalized has
	// to be the last finalized block.
	// It modifies the persistent immutable protocol state accordingly and
	// forwards the pointer to the latest finalized state.
	// TODO error docs
	Finalize(ctx context.Context, blockID flow.Identifier) error
}

type ParticipantState interface {
	FollowerState
	// Extend introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows
	// us to execute fork-aware queries against ambiguous protocol state, while
	// still checking that the given block is a valid extension of the protocol state.
	// Depending on implementation it might be a lighter version that checks only block header.
	// The candidate block must have passed HotStuff validation before being passed to Extend.
	// Expected errors during normal operations:
	//  * state.OutdatedExtensionError if the candidate block is outdated (e.g. orphaned)
	//  * state.InvalidExtensionError if the candidate block is invalid
	Extend(ctx context.Context, candidate *flow.Block) error
}
