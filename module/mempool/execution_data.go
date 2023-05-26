package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// ExecutionData represents a concurrency-safe memory pool for BlockExecutionData.
type ExecutionData interface {

	// Has checks whether the execution data with the given hash is currently in
	// the memory pool.
	Has(id flow.Identifier) bool

	// Add adds the given execution data to the memory pool.
	// It returns false if the execution data was already in the mempool.
	Add(ed *execution_data.BlockExecutionDataEntity) bool

	// Remove removes the given execution data from the memory pool.
	// It returns true if the execution data was known and removed.
	Remove(id flow.Identifier) bool

	// ByID retrieves the execution data with the given ID from the memory pool.
	// It returns false if the execution data was not found in the mempool.
	ByID(txID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool)

	// Size return the current size of the memory pool.
	Size() uint

	// All retrieves all execution data that are currently in the memory pool
	// as a slice.
	All() []*execution_data.BlockExecutionDataEntity

	// Clear removes all execution data from the mempool.
	Clear()
}
