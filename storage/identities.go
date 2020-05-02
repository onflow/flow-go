package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Identities represents the simple storage for identities.
type Identities interface {

	// ByBlockID will retrieve all identities from the storage by block
	ByBlockID(blockID flow.Identifier) (flow.IdentityList, error)
}
