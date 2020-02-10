package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// RegisterSets represents persistent storage for register sets.
type RegisterSets interface {

	// Store will store a register set in the persistent storage.
	Store(blockID flow.Identifier, set *flow.RegisterSet) error

	// ByBlockID will retrieve a register set by block ID from persistent storage.
	ByBlockID(blockID flow.Identifier) (*flow.RegisterSet, error)
}
