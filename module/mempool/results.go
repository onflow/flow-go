package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Results represents a concurrency-safe memory pool for execution results.
type Results interface {

	// Has will check if the given result is in the memory pool.
	Has(resultID flow.Identifier) bool

	// Add will add the given execution result to the memory pool. It will return
	// false if it was already in the mempool.
	Add(result *flow.ExecutionResult) bool

	// Rem will attempt to remove the result from the memory pool.
	Rem(resultID flow.Identifier) bool

	// ByID retrieve the execution result with the given ID from the memory pool.
	// It will return false if it was not found in the mempool.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will return a list of all approvals in the memory pool.
	All() []*flow.ExecutionResult

	// DropForBlock will remove all results for the given block.
	DropForBlock(blockID flow.Identifier) []flow.Identifier
}
