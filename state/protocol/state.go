// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
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

	// Bootstrap initializes the persistent protocol state with the given block,
	// execution state and block seal. In order to successfully bootstrap, the
	// execution result needs to refer to the provided block and the block seal
	// needs to refer to the provided block and execution result. The identities
	// in the block payload will be used as the initial set of staked node
	// identities.
	Bootstrap(root *flow.Block, result *flow.ExecutionResult, seal *flow.Seal) error
}
