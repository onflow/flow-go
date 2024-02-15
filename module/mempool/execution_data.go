package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// ExecutionData represents a concurrency-safe memory pool for BlockExecutionData.
type ExecutionData interface {

	// Has checks whether the block execution data for the given block ID is currently in
	// the memory pool.
	Has(flow.Identifier) bool

	// Add adds a block execution data to the mempool, keyed by block ID.
	// It returns false if the execution data was already in the mempool.
	Add(*execution_data.BlockExecutionDataEntity) bool

	// Remove removes block execution data from mempool by block ID.
	// It returns true if the execution data was known and removed.
	Remove(flow.Identifier) bool

	// ByID returns the block execution data for the given block ID from the mempool.
	// It returns false if the execution data was not found in the mempool.
	ByID(flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool)

	// Size return the current size of the memory pool.
	Size() uint

	// All retrieves all execution data that are currently in the memory pool
	// as a slice.
	All() []*execution_data.BlockExecutionDataEntity

	// Clear removes all execution data from the mempool.
	Clear()
}
