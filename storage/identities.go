package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Identities represents the simple storage for identities.
type Identities interface {

	// Store will store the given identity.
	Store(identity *flow.Identity) error

	// ByNodeID will retrieve the identity for a node.
	ByNodeID(nodeID flow.Identifier) (*flow.Identity, error)
}
