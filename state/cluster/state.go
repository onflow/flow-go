package cluster

import (
	cluster "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// State represents the chain state for collection node cluster consensus. It
// tracks which blocks are finalized and indexes blocks by number and ID. The
// purpose of cluster consensus is to agree on collections of transactions, so
// each block within the cluster state corresponds to a proposed collection.
//
// NOTE: This is modelled after, and is a simpler version of, protocol.State.
type State interface {

	// Params returns constant information about the cluster state.
	Params() Params

	// Final returns the snapshot of the cluster state at the latest finalized
	// block. The returned snapshot is therefore immutable over time.
	Final() Snapshot

	// AtBlockID returns the snapshot of the persistent cluster at the given
	// block ID. It is available for any block that was introduced into the
	// the cluster state, and can thus represent an ambiguous state that was or
	// will never be finalized.
	AtBlockID(blockID flow.Identifier) Snapshot
}

// MutableState allows extending the cluster state in a consistent manner that preserves
// integrity, validity, and functionality of the database. It enforces a number of invariants on the
// input data to ensure internal bookkeeping mechanisms remain functional and valid.
type MutableState interface {
	State
	// Extend introduces the given block into the cluster state as a pending
	// without modifying the current finalized state.
	Extend(candidate *cluster.Block) error
}
