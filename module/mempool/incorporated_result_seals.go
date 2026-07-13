package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// IncorporatedResultSeals represents a concurrency safe memory pool for
// incorporated result seals.
type IncorporatedResultSeals interface {
	// Add adds an IncorporatedResultSeal to the mempool. The method returns true if the seal was added to the mempool,
	// and false if it was a duplicate (dropped). The seal is considered a duplicate if and only if a seal for the same
	// IncorporatedResult ID is already present in the mempool.
	Add(irSeal *flow.IncorporatedResultSeal) (bool, error)

	// All returns all the IncorporatedResultSeals in the mempool.
	All() []*flow.IncorporatedResultSeal

	// Get returns an IncorporatedResultSeal by IncorporatedResult ID.
	Get(flow.Identifier) (*flow.IncorporatedResultSeal, bool)

	// Limit returns the size limit of the mempool.
	Limit() uint

	// Remove removes an IncorporatedResultSeal from the mempool.
	Remove(incorporatedResultID flow.Identifier) bool

	// Size returns the number of items in the mempool.
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
	// and the sentinel mempool.BelowPrunedThresholdError is returned.
	PruneUpToHeight(height uint64) error
}
