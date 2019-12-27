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
	results map[string]*flow.ExecutionResult
	// receipts is a key-value store of the ExecutionReceipts
	// receipts[h1] returns the hash set of all the ExecutionReceipts with their
	// hash value of the ExecutionResult as h1
	// todo a proto execution receipt for avoiding several copies of the same execution results
	receipts map[string]map[string]*flow.ExecutionReceipt
}

// New creates, initializes and returns a new ERMempool
func New() *ERMempool {
	return &ERMempool{
		results:  make(map[string]*flow.ExecutionResult),
		receipts: make(map[string]map[string]*flow.ExecutionReceipt),
	}

}

// ResultNum returns number of ExecutionResults that are stored in the ERMempool
func (m *ERMempool) ResultsNum() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.results)
}

// ReceiptsCount receives hash of an execution result and returns the number of
// execution receipts that are stored in the Mempool
func (m *ERMempool) ReceiptsCount(fingerprint flow.Fingerprint) int {
	m.RLock()
	defer m.RUnlock()
	return len(m.receipts[fingerprint.Hex()])
}

// GetExecutionResult represents the process of retrieving an ExecutionResult from the Mempool
// If the ExecutionResult exists in the ERMempool, GetExecutionResult should return the item and a nil error
// Otherwise, it should return an error indicating that the ExecutionResult does not exist in the Mempool
func (m *ERMempool) GetExecutionResult(fingerprint flow.Fingerprint) (*flow.ExecutionResult, error) {
	m.RLock()
	defer m.RUnlock()
	hashStr := fingerprint.Hex()
	result, ok := m.results[hashStr]
	if !ok {
		return nil, errors.Errorf("no Execution Result exists for this hash: %s", hashStr)
	}
	return result, nil
}

// GetExecutionReceipts represents the process of fetching all the ExecutionReceipts that
// are associated with a certain ExecutionResult. On receiving the hash of a ExecutionResult, it
// returns the hash table of all the ExecutionReceipts associated with that ExecutionResult. The keys in the
// returned hash table is the hash of ExecutionReceipts and values are the ExecutionReceipts themselves.
// It returns an error in case there is no ExecutionReceipt corresponding to the input hash in the Mempool
func (m *ERMempool) GetExecutionReceipts(fingerprint flow.Fingerprint) (map[string]*flow.ExecutionReceipt, error) {
	m.RLock()
	defer m.RUnlock()
	hashStr := fingerprint.Hex()
	ers, ok := m.receipts[hashStr]
	if !ok {
		return nil, errors.Errorf("no Execution Receipts exists for this hash: %s", hashStr)
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
	resultHash := receipt.ExecutionResult.Fingerprint().Hex()

	// storing the execution result of the receipt in the results map
	// in case it does not exist
	if _, ok := m.results[resultHash]; !ok {
		m.results[resultHash] = &receipt.ExecutionResult
	}

	//todo unloading the ExecutionResult part of the receipt for memory efficiency

	// allocating memory for the map of execution receipts corresponding to this block
	if _, ok := m.receipts[resultHash]; !ok {
		m.receipts[resultHash] = make(map[string]*flow.ExecutionReceipt)
	}

	receiptHash := receipt.Hash().Hex()
	// checking for duplication
	if _, ok := m.receipts[resultHash][receiptHash]; !ok {
		// storing this execution receipt to the set of execution receipts
		m.receipts[resultHash][receiptHash] = receipt
		// invoking the handler on the new execution receipt
		return true
	}
	return false
}
