package storage

import (
	"github.com/pkg/errors"
	"sync"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/execution"
)

// ERMempool represents an in-memory storage of the execution receipts
type ERMempool struct {
	// mutex is used to secure the ERMempool against concurrent access
	sync.RWMutex
	// results is a key-value mempool of the ExecutionResults
	results map[string]*execution.ExecutionResult
	// receipts is a key-value mempool of the ExecutionReceipts
	// todo current implementation does not deduplicate execution results
	// todo a proto execution receipt for avoiding several copies of the same execution results
	receipts map[string][]execution.ExecutionReceipt
}

// New creates, initializes and returns a new ERMempool
func New() *ERMempool {
	return &ERMempool{
		results:  make(map[string]*execution.ExecutionResult),
		receipts: make(map[string][]execution.ExecutionReceipt),
	}

}

// ResultNum returns number of ExecutionResults that are stored in the ERMempool
func (m *ERMempool) ResultsNum() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.results)
}

// ReceiptsCount receives hash of an execution result and returns the number of
// execution receipts that are stored in the ERMempool
func (m *ERMempool) ReceiptsCount(hash crypto.Hash) int {
	m.RLock()
	defer m.RUnlock()
	return len(m.receipts[hash.Hex()])
}

// GetExecutionResult represents the process of retrieving an ExecutionResult from the ERMempool
// If the ExecutionResult exists in the ERMempool, GetExecutionResult should return the item and a nil error
// Otherwise, it should return an error indicating that the ExecutionResult does not exist in the ERMempool
func (m *ERMempool) GetExecutionResult(hash crypto.Hash) (*execution.ExecutionResult, error) {
	m.RLock()
	defer m.RUnlock()
	hashStr := hash.Hex()
	result, ok := m.results[hashStr]
	if !ok {
		return nil, errors.Errorf("no Execution Result exists for this hash: %s", hashStr)
	}
	return result, nil
}

// GetExecutionReceipts represents the process of fetching all the ExecutionReceipts that
// are associated with a certain ExecutionResult. On receiving the hash of a ExecutionResult, it
// returns the slice of all the ExecutionReceipts associated with that ExecutionResult.
// It returns an error in case there is no ExecutionReceipt corresponding to the input hash in the ERMempool
func (m *ERMempool) GetExecutionReceipts(hash crypto.Hash) ([]execution.ExecutionReceipt, error) {
	m.RLock()
	defer m.RUnlock()
	hashStr := hash.Hex()
	ers, ok := m.receipts[hashStr]
	if !ok {
		return nil, errors.Errorf("no Execution Receipts exists for this hash: %s", hashStr)
	}
	return ers, nil
}

// Put represents the process of storing an ExecutionReceipt in the ERMempool
// It also makes sure that a single instance of the ExecutionReceipt's result part is
// available in the result ERMempool
func (m *ERMempool) Put(receipt *execution.ExecutionReceipt) {
	m.Lock()
	defer m.Unlock()

	// taking hash of execution result of the receipt
	hash := receipt.ExecutionResult.Hash()
	// todo refactor hash.Hex to hash.String
	hashStr := hash.Hex()

	// storing the execution result of the receipt in the results map
	// in case it does not exist
	if _, ok := m.results[hashStr]; !ok {
		m.results[hashStr] = &receipt.ExecutionResult
	}

	//todo unloading the ExecutionResult part of the receipt for memory efficiency

	// allocating memory for the slice of execution receipts corresponding to this block
	if _, ok := m.receipts[hashStr]; !ok {
		m.receipts[hashStr] = make([]execution.ExecutionReceipt, 0)
	}

	// appending this execution receipt to the set of execution receipts of the same block
	m.receipts[hashStr] = append(m.receipts[hashStr], *receipt)
}
