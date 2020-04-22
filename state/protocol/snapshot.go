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

	// Seal return the highest seal at the selected snapshot.
	Seal() (flow.Seal, error)

	// Cluster selects the given cluster from the node selection. You have to
	// manually filter the identities to the desired set of nodes before
	// clustering them.
	Clusters() (*flow.ClusterList, error)

	// Head returns the latest block at the selected point of the protocol state
	// history. It can represent either a finalized or ambiguous block,
	// depending on our selection criteria. Either way, it's the block on which
	// we should build the next block in the context of the selected state.
	Head() (*flow.Header, error)

	// Unfinalized returns the unfinalized block IDs that connect to the finalized
	// block.
	Unfinalized() ([]flow.Identifier, error)
}
