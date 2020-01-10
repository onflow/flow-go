package storage

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
)

// ERMempool represents an in-memory storage of the execution receipts
// It implements the verification's Mempool interface
type ERMempool struct {
	// mutex is used to secure the Mempool against concurrent access
	sync.RWMutex
	// results is a key-value store of the ExecutionResults
	results map[flow.Identifier]*flow.ExecutionResult
	// receipts is a key-value store of the ExecutionReceipts
	// receipts[h1] returns the hash set of all the ExecutionReceipts with their
	// hash value of the ExecutionResult as h1
	// todo a proto execution receipt for avoiding several copies of the same execution results
	receipts map[flow.Identifier]map[flow.Identifier]*flow.ExecutionReceipt
}

// New creates, initializes and returns a new ERMempool
func New() *ERMempool {
	return &ERMempool{
		results:  make(map[flow.Identifier]*flow.ExecutionResult),
		receipts: make(map[flow.Identifier]map[flow.Identifier]*flow.ExecutionReceipt),
	}

}

// ResultNum returns number of ExecutionResults that are stored in the ERMempool
func (m *ERMempool) ResultsNum() uint {
	m.RLock()
	defer m.RUnlock()
	return uint(len(m.results))
}

// ReceiptsCount receives hash of an execution result and returns the number of
// execution receipts that are stored in the Mempool
func (m *ERMempool) ReceiptsCount(resultID flow.Identifier) uint {
	m.RLock()
	defer m.RUnlock()
	return uint(len(m.receipts[resultID]))
}

// GetExecutionResult represents the process of retrieving an ExecutionResult from the Mempool
// If the ExecutionResult exists in the ERMempool, GetExecutionResult should return the item and a nil error
// Otherwise, it should return an error indicating that the ExecutionResult does not exist in the Mempool
func (m *ERMempool) GetExecutionResult(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	m.RLock()
	defer m.RUnlock()
	result, ok := m.results[resultID]
	if !ok {
		return nil, errors.Errorf("no Execution Result exists for this hash: %x", resultID)
	}
	return result, nil
}

// GetExecutionReceipts represents the process of fetching all the ExecutionReceipts that
// are associated with a certain ExecutionResult. On receiving the hash of a ExecutionResult, it
// returns the hash table of all the ExecutionReceipts associated with that ExecutionResult. The keys in the
// returned hash table is the hash of ExecutionReceipts and values are the ExecutionReceipts themselves.
// It returns an error in case there is no ExecutionReceipt corresponding to the input hash in the Mempool
func (m *ERMempool) GetExecutionReceipts(resultID flow.Identifier) (map[flow.Identifier]*flow.ExecutionReceipt, error) {
	m.RLock()
	defer m.RUnlock()
	ers, ok := m.receipts[resultID]
	if !ok {
		return nil, errors.Errorf("no Execution Receipts exists for this hash: %x", resultID)
	}
	return ers, nil
}

// Put represents the process of storing an ExecutionReceipt in the Mempool
// It also makes sure that a single instance of the ExecutionReceipt's result part is
// available in the result part of the ERMempool
func (m *ERMempool) Put(receipt *flow.ExecutionReceipt) bool {
	m.Lock()
	defer m.Unlock()

	// todo refactor hash.Hex to hash.String
	// taking hash of execution result of the receipt
	resultID := receipt.ExecutionResult.ID()

	// storing the execution result of the receipt in the results map
	// in case it does not exist
	if _, ok := m.results[resultID]; !ok {
		m.results[resultID] = &receipt.ExecutionResult
	}

	//todo unloading the ExecutionResult part of the receipt for memory efficiency

	// allocating memory for the map of execution receipts corresponding to this block
	if _, ok := m.receipts[resultID]; !ok {
		m.receipts[resultID] = make(map[flow.Identifier]*flow.ExecutionReceipt)
	}

	receiptID := receipt.ID()
	// checking for duplication
	if _, ok := m.receipts[resultID][receiptID]; !ok {
		// storing this execution receipt to the set of execution receipts
		m.receipts[resultID][receiptID] = receipt
		// invoking the handler on the new execution receipt
		return true
	}
	return false
}
