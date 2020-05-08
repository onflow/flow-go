package cluster

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Snapshot represents an immutable snapshot at a specific point in the cluster
// state history.
type Snapshot interface {

	// Collection returns the collection generated in this step of the cluster
	// state history.
	Collection() (*flow.Collection, error)

	// Head returns the latest block at the selected point of the cluster state
	// history. If the snapshot was selected by block ID, returns the header
	// with that block ID. If the snapshot was selected as final, returns the
	// latest finalized block.
	Head() (*flow.Header, error)

	// Pending returns all children IDs for the snapshot head, which thus were
	// potential extensions of the protocol state at this snapshot. The result
	// is ordered such that parents are included before their children.
	Pending() ([]flow.Identifier, error)
}
