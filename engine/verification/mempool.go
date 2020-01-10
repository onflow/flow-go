package verification

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Mempool is an interface of cache for storing ExecutionReceipts and ExecutionResults
type Mempool interface {
	// Put represents the process of storing an ExecutionReceipt in the mempool
	// Put returns true if the input ExecutionReceipt has not yet been stored in the
	// mempool, otherwise, it returns false as a sign of duplicate detection
	Put(receipt *flow.ExecutionReceipt) bool

	// GetExecutionResult represents the process of retrieving an ExecutionResult from the mempool
	// If the ExecutionResult exists in the mempool, GetExecutionResult should return the item and a nil error
	// Otherwise, it should return an error indicating that the ExecutionResult does not exist in the mempool
	GetExecutionResult(resultID flow.Identifier) (*flow.ExecutionResult, error)

	// GetExecutionReceipts represents the process of fetching all the ExecutionReceipts that
	// are associated with a certain ExecutionResult. On receiving the hash of a ExecutionResult, it
	// returns the hash table of all the ExecutionReceipts associated with that ExecutionResult. The keys in the
	// returned hash table is the hash of ExecutionReceipts and values are the ExecutionReceipts themselves.
	// It returns an error in case there is no ExecutionReceipt corresponding to the input hash in the mempool
	GetExecutionReceipts(resultID flow.Identifier) (map[flow.Identifier]*flow.ExecutionReceipt, error)

	// ReceiptsCount receives hash of an execution result
	// and returns the number of execution receipts that are stored in the mempool
	ReceiptsCount(resultID flow.Identifier) uint

	// ResultNum returns number of ExecutionResults that are stored in the mempool
	ResultsNum() uint
}
