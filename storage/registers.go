package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Registers represent persistent storage for execution data registers.
type Registers interface {

	// Store insert register value indexed by ID. If update paths exist the payloads are overwritten.
	Store(ID flow.RegisterID, value *flow.RegisterEntry) error

	// ByID returns register by the ID.
	ByID(ID flow.RegisterID) (*flow.RegisterEntry, error)

	// TODO(sideninja) maybe we should add batch insert and retrieve methods to optimize operations.
}
