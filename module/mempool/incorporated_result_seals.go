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

	// Limit returns the size limit of the mempool
	Limit() uint

	// Rem removes an IncorporatedResultSeal from the mempool
	Rem(incorporatedResultID flow.Identifier) bool

	// Size returns the number of items in the mempool
	Size() uint

	// Clear removes all entities from the pool.
	Clear()

	// PruneUpToHeight remove all seals for blocks whose height is strictly
	// smaller that height. Note: seals for blocks at height are retained.
	// After pruning, seals below for blocks below the given height are dropped.
	//
	// Monotonicity Requirement:
	// The pruned height cannot decrease, as we cannot recover already pruned elements.
	// If `height` is smaller than the previous value, the previous value is kept
	// and the sentinel empool.NewDecreasingPruningHeightError is returned.
	PruneUpToHeight(height uint64) error
}
