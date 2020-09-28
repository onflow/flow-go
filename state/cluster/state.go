package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

// State represents the chain state for collection node cluster consensus. It
// tracks which blocks are finalized and indexes blocks by number and ID. The
// purpose of cluster consensus is to agree on collections of transactions, so
// each block within the cluster state corresponds to a proposed collection.
//
// NOTE: This is modelled after, and is a simpler version of, protocol.State.
type State interface {

	// Final returns the snapshot of the cluster state at the latest finalized
	// block. The returned snapshot is therefore immutable over time.
	Final() Snapshot

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
