// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"fmt"

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

type MutableState interface {
	State
	// Extend introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows
	// us to execute fork-aware queries against ambiguous protocol state, while
	// still checking that the given block is a valid extension of the protocol
	// state. Depending on implementation it might be a lighter version that checks only
	// block header.
	Extend(candidate *flow.Block) error

	// Finalize finalizes the block with the given hash.
	// At this level, we can only finalize one block at a time. This implies
	// that the parent of the pending block that is to be finalized has
	// to be the last finalized block.
	// It modifies the persistent immutable protocol state accordingly and
	// forwards the pointer to the latest finalized state.
	Finalize(blockID flow.Identifier) error

	// MarkValid marks the block header with the given block hash as valid.
	// At this level, we can only mark one block at a time as valid. This
	// implies that the parent of the block to be marked as valid
	// has to be already valid.
	// It modifies the persistent immutable protocol state accordingly.
	MarkValid(blockID flow.Identifier) error
}

// IsNodeStakedAtBlockID returns true if the `identifier` is staked at `blockID` according to `state`.
func IsNodeStakedAtBlockID(state State, blockID flow.Identifier, identifier flow.Identifier) (bool, error) {
	identity, err := state.AtBlockID(blockID).Identity(identifier)
	if err != nil {
		return false, fmt.Errorf("could not retrieve identity for identifier %v at block id snapshot %v: %w)", identifier, blockID, err)
	}

	staked := identity.Stake > 0
	return staked, nil
}

// StakedIdentity returns staked identity of the input identifier is staked at the given block identifier according to the input state.
func StakedIdentity(state State, blockID flow.Identifier, identifier flow.Identifier) (flow.Identity, error) {
	identity, err := state.AtBlockID(blockID).Identity(identifier)
	if err != nil {
		return flow.Identity{}, fmt.Errorf("could not retrieve identity for identifier %v at block id snapshot %v: %w)", identifier, blockID, err)
	}

	return identity, nil
}
