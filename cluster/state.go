package cluster

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

// State represents the chain state for collection node cluster consensus. It
// tracks which blocks are finalized and indexes blocks by number and ID.
//
// NOTE: This is modelled after, and is a simpler version of, protocol.State.
type State interface {

	// Final returns the snapshot of the cluster state at the latest finalized
	// block. The returned snapshot is therefore immutable over time.
	Final() Snapshot

	// AtNumber returns the snapshot of the persistent cluster state at the
	// given block number. It is only available for finalized blocks and the
	// returned snapshot is therefore immutable over.
	AtNumber(number uint64) Snapshot

	// AtBlockID returns the snapshot of the persistent cluster at the given
	// block ID. It is available for any block that was introduced into the
	// the cluster state, and can thus represent an ambiguous state that was or
	// will never be finalized.
	AtBlockID(blockID flow.Identifier) Snapshot

	// Mutate will create a mutator for the persistent cluster state. It allows
	// extending the cluster state in a consistent manner that preserves
	// integrity, validity, and functionality of the database.
	Mutate() Mutator
}

// Snapshot represents an immutable snapshot at a specific point in the cluster
// state history.
type Snapshot interface {

	// Collection returns the collection generated in this step of the cluster
	// state history.
	Collection() *flow.Collection
}

// Mutator represents an interface to modify the persistent cluster state in a
// way that conserves its integrity. It enforces a number of invariants on the
// input data to ensure internal bookkeeping mechanisms remain functional and
// valid.
type Mutator interface {

	// Bootstrap initializes the persistent cluster state with a genesis block.
	// The genesis block must have number 0, a parent hash of 32 zero bytes,
	// and an empty collection as payload.
	Bootstrap(genesis *cluster.Block) error

	// Extend introduces the block with the given ID into the persistent
	// cluster state without modifying the current finalized state.
	Extend(blockID flow.Identifier)
}
