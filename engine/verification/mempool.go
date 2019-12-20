package verification

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/execution"
)

// Mempool is an interface of cache for storing ExecutionReceipts and ExecutionResults
type Mempool interface {
	// Put represents the process of storing an ExecutionReceipt in the Mempool
	Put(receipt *execution.ExecutionReceipt)

	// GetExecutionResult represents the process of retrieving an ExecutionResult from the Mempool
	// If the ExecutionResult exists in the mempool, GetExecutionResult should return the item and a nil error
	// Otherwise, it should return an error indicating that the ExecutionResult does not exist in the Mempool
	GetExecutionResult(hash crypto.Hash) (*execution.ExecutionResult, error)


	// GetExecutionReceipts represents the process of fetching all the ExecutionReceipts that
	// are associated with a certain ExecutionResult. On receiving the hash of a ExecutionResult, it
	// returns the slice of all the ExecutionReceipts associated with that ExecutionResult.
	// It returns an error in case there is no ExecutionReceipt corresponding to the input hash in the Mempool
	GetExecutionReceipts(hash crypto.Hash) ([]execution.ExecutionReceipt, error)

	// ReceiptsCount receives hash of an execution result
	// and returns the number of execution receipts that are stored in the Mempool
	ReceiptsCount(hash crypto.Hash) int

	// ResultNum returns number of ExecutionResults that are stored in the Mempool
	ResultsNum() int
}
