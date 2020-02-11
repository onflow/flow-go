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
}
