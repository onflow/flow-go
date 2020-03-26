package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Identities represents the simple storage for identities.
type Identities interface {

	// Store inserts the collection guarantee.
	Store(identity *flow.Identity) error

	// ByNodeID retrieves an identity by node ID.
	ByNodeID(nodeID flow.Identifier) (*flow.Identity, error)
}
