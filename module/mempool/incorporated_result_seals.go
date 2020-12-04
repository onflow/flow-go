// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// IncorporatedResultSeals represents a concurrency safe memory pool for
// incorporated result seals
type IncorporatedResultSeals interface {
	// Add adds an IncorporatedResultSeal to the mempool
	Add(irSeal *flow.IncorporatedResultSeal) (bool, error)

	// All returns all the IncorporatedResultSeals in the mempool
	All() []*flow.IncorporatedResultSeal

	// ByID returns an IncorporatedResultSeal by ID
	ByID(flow.Identifier) (*flow.IncorporatedResultSeal, bool)

	// RegisterEjectionCallbacks adds the provided OnEjection callbacks
	RegisterEjectionCallbacks(callbacks ...OnEjection)

	// Limit returns the size limit of the mempool
	Limit() uint

	// Rem removes an IncorporatedResultSeal from the mempool
	Rem(incorporatedResultID flow.Identifier) bool

	// Size returns the number of items in the mempool
	Size() uint

	// Clear removes all entities from the pool.
	Clear()
}
