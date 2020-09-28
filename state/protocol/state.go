// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// State allows, in addition to ReadOnlyState,  mutating the protocol state in a consistent manner.
type State interface {
	ReadOnlyState

	// Mutate will create a mutator for the persistent protocol state. It allows
	// us to extend the protocol state in a consistent manner that conserves the
	// integrity, validity and functionality of the database.
	Mutate() Mutator
}

// ReadOnlyState represents the full protocol state of the local node. It allows us to
// obtain snapshots of the state at any point of the protocol state history.
type ReadOnlyState interface {

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
