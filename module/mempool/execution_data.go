package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// ExecutionData represents a concurrency-safe memory pool for BlockExecutionData.
type ExecutionData interface {

	// Has checks whether the transaction with the given hash is currently in
	// the memory pool.
	Has(id flow.Identifier) bool

	// Add will add the given transaction body to the memory pool. It will
	// return false if it was already in the mempool.
	Add(ed *execution_data.BlockExecutionDataEntity) bool

	// Remove will remove the given transaction from the memory pool; it will
	// will return true if the transaction was known and removed.
	Remove(id flow.Identifier) bool

	// ByID retrieve the transaction with the given ID from the memory
	// pool. It will return false if it was not found in the mempool.
	ByID(txID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all transactions that are currently in the memory pool
	// as a slice.
	All() []*execution_data.BlockExecutionDataEntity

	// Clear removes all transactions from the mempool.
	Clear()
}
