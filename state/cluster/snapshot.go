package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

// Snapshot pertains to a specific fork of the collector cluster consensus. Specifically,
// it references one block denoted as the `Head`. This Snapshot type is for collector
// clusters, so we are referencing a cluster block, aka collection, here.
type Snapshot interface {

	// Collection returns the collection designated as the reference for this
	// snapshot. Technically, this is a portion of the payload of a cluster block.
	//
	// Expected error returns during normal operations:
	//  - If the snapshot is for an unknown collection [state.ErrUnknownSnapshotReference]
	Collection() (*flow.Collection, error)

	// Head returns the header of the collection that is designated as the reference for
	// this snapshot. Technically, this is the header of a [cluster.Block]
	//
	// Expected error returns during normal operations:
	//  - If the snapshot is for an unknown collection [state.ErrUnknownSnapshotReference]
	Head() (*flow.Header, error)

	// Pending returns the IDs of all collections descending from the snapshot's head collection.
	// The result is ordered such that parents are included before their children. While only valid
	// descendants will be returned, note that the descendants may not be finalized yet.
	//
	// CAUTION: the list of descendants returned is constructed for each call via database reads,
	// and may be expensive to compute, especially if the reference collection is older.
	//
	// Expected error returns during normal operations:
	//  - If the snapshot is for an unknown collection [state.ErrUnknownSnapshotReference]
	Pending() ([]flow.Identifier, error)
}
