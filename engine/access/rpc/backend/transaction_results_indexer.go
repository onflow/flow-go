package backend

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

// TransactionResultsIndex When the index is initially bootstrapped, the indexer needs to load an execution state
// checkpoint from disk and index all the data. This process can take more than 1 hour on some systems. Consequently,
// the Initialize pattern is implemented to enable the Access API to start up and serve queries before the index is
// fully ready. During the initialization phase, all calls to retrieve data from this struct should return
// indexer.ErrIndexNotInitialized. The caller is responsible for handling this error appropriately for the method.
type TransactionResultsIndex struct {
	results  storage.LightTransactionResults
	reporter *atomic.Pointer[state_synchronization.IndexReporter]
}

var _ state_synchronization.IndexReporter = (*TransactionResultsIndex)(nil)

func NewTransactionResultsIndex(results storage.LightTransactionResults) *TransactionResultsIndex {
	return &TransactionResultsIndex{
		results:  results,
		reporter: atomic.NewPointer[state_synchronization.IndexReporter](nil),
	}
}

// Initialize replaces nil value with actual reporter instance
// Expected errors:
// - If the reporter was already initialized, return error
func (t *TransactionResultsIndex) Initialize(indexReporter state_synchronization.IndexReporter) error {
	if t.reporter.CompareAndSwap(nil, &indexReporter) {
		return nil
	}
	return fmt.Errorf("index reporter already initialized")
}

// ByBlockID checks data availability and returns all transaction results for a block
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the `TransactionResultsIndex` has not been initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
func (t *TransactionResultsIndex) ByBlockID(blockID flow.Identifier, height uint64) ([]flow.LightTransactionResult, error) {
	if err := t.checkDataAvailability(height); err != nil {
		return nil, err
	}

	return t.results.ByBlockID(blockID)
}

// ByBlockIDTransactionID checks data availability and return the transaction result for the given block ID and transaction ID
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the `TransactionResultsIndex` has not been initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
func (t *TransactionResultsIndex) ByBlockIDTransactionID(blockID flow.Identifier, height uint64, txID flow.Identifier) (*flow.LightTransactionResult, error) {
	if err := t.checkDataAvailability(height); err != nil {
		return nil, err
	}

	return t.results.ByBlockIDTransactionID(blockID, txID)
}

// ByBlockIDTransactionIndex checks data availability and return the transaction result for the given blockID and transaction index
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the `TransactionResultsIndex` has not been initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - codes.NotFound when result cannot be provided by storage due to the absence of data.
func (t *TransactionResultsIndex) ByBlockIDTransactionIndex(blockID flow.Identifier, height uint64, index uint32) (*flow.LightTransactionResult, error) {
	if err := t.checkDataAvailability(height); err != nil {
		return nil, err
	}

	return t.results.ByBlockIDTransactionIndex(blockID, index)
}

// LowestIndexedHeight returns the lowest height indexed by the execution state indexer.
// Expected errors:
// - indexer.ErrIndexNotInitialized if the `TransactionResultsIndex` has not been initialized
func (t *TransactionResultsIndex) LowestIndexedHeight() (uint64, error) {
	reporter, err := t.getReporter()
	if err != nil {
		return 0, err
	}

	return reporter.LowestIndexedHeight()
}

// HighestIndexedHeight returns the highest height indexed by the execution state indexer.
// Expected errors:
// - indexer.ErrIndexNotInitialized if the `TransactionResultsIndex` has not been initialized
func (t *TransactionResultsIndex) HighestIndexedHeight() (uint64, error) {
	reporter, err := t.getReporter()
	if err != nil {
		return 0, err
	}

	return reporter.HighestIndexedHeight()
}

func (t *TransactionResultsIndex) checkDataAvailability(height uint64) error {
	reporter, err := t.getReporter()
	if err != nil {
		return err
	}

	highestHeight, err := reporter.HighestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get highest indexed height: %w", err)
	}
	if height > highestHeight {
		return fmt.Errorf("%w: block not indexed yet", storage.ErrHeightNotIndexed)
	}

	lowestHeight, err := reporter.LowestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get lowest indexed height: %w", err)
	}
	if height < lowestHeight {
		return fmt.Errorf("%w: block is before lowest indexed height", storage.ErrHeightNotIndexed)
	}

	return nil
}

func (t *TransactionResultsIndex) getReporter() (state_synchronization.IndexReporter, error) {
	reporter := t.reporter.Load()
	if reporter == nil {
		return nil, indexer.ErrIndexNotInitialized
	}
	return *reporter, nil
}
