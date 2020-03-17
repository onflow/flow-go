package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// RegisterDeltas represents persistent storage for register deltas.
type RegisterDeltas interface {

	// Store will store a register delta in the persistent storage.
	Store(blockID flow.Identifier, delta *flow.RegisterDelta) error

	// ByBlockID will retrieve a register delta by block ID from persistent storage.
	ByBlockID(blockID flow.Identifier) (*flow.RegisterDelta, error)
}
