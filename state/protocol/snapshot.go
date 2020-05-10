// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Snapshot represents an immutable snapshot at a specific point of the
// protocol state history. It allows us to read the parameters at the selected
// point in a deterministic manner.
type Snapshot interface {

	// Identities returns a list of identities at the selected point of
	// the protocol state history. It allows us to provide optional upfront
	// filters which can be used by the implementation to speed up database
	// lookups.
	Identities(selector flow.IdentityFilter) (flow.IdentityList, error)

	// Identity attempts to retrieve the node with the given identifier at the
	// selected point of the protocol state history. It will error if it doesn't exist.
	Identity(nodeID flow.Identifier) (*flow.Identity, error)

	// Commit return the sealed execution state commitment at this block.
	Commit() (flow.StateCommitment, error)

	// Cluster selects the given cluster from the node selection. You have to
	// manually filter the identities to the desired set of nodes before
	// clustering them.
	Clusters() (*flow.ClusterList, error)

	// Head returns the latest block at the selected point of the protocol state
	// history. It can represent either a finalized or ambiguous block,
	// depending on our selection criteria. Either way, it's the block on which
	// we should build the next block in the context of the selected state.
	Head() (*flow.Header, error)

	// Pending returns all children IDs for the snapshot head, which thus were
	// potential extensions of the protocol state at this snapshot. The result
	// is ordered such that parents are included before their children.
	Pending() ([]flow.Identifier, error)

	Contains(blockID flow.Identifier) (bool, error)
}
