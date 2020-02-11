package cluster

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

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
	Extend(blockID flow.Identifier) error
}
