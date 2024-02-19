package backend

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

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

func (t *TransactionResultsIndex) Initialize(indexReporter state_synchronization.IndexReporter) error {
	if t.reporter.CompareAndSwap(nil, &indexReporter) {
		return nil
	}
	return fmt.Errorf("index reporter already initialized")
}

func (t *TransactionResultsIndex) ByBlockID(blockID flow.Identifier, height uint64) ([]flow.LightTransactionResult, error) {
	if err := t.checkDataAvailable(height); err != nil {
		return nil, err
	}

	return t.results.ByBlockID(blockID)
}

func (t *TransactionResultsIndex) GetResultsByBlockIDTransactionID(blockID flow.Identifier, height uint64, txID flow.Identifier) (*flow.LightTransactionResult, error) {
	if err := t.checkDataAvailable(height); err != nil {
		return nil, err
	}

	return t.results.ByBlockIDTransactionID(blockID, txID)
}

func (t *TransactionResultsIndex) GetResultsByBlockIDTransactionIndex(blockID flow.Identifier, height uint64, index uint32) (*flow.LightTransactionResult, error) {
	if err := t.checkDataAvailable(height); err != nil {
		return nil, err
	}

	return t.results.ByBlockIDTransactionIndex(blockID, index)
}

// LowestIndexedHeight returns the lowest height indexed by the execution state indexer.
// Expected errors:
// - indexer.ErrIndexNotInitialized: if the EventsIndex has not been initialized
func (t *TransactionResultsIndex) LowestIndexedHeight() (uint64, error) {
	reporter, err := t.getReporter()
	if err != nil {
		return 0, err
	}

	return reporter.LowestIndexedHeight()
}

// HighestIndexedHeight returns the highest height indexed by the execution state indexer.
// Expected errors:
// - indexer.ErrIndexNotInitialized: if the EventsIndex has not been initialized
func (t *TransactionResultsIndex) HighestIndexedHeight() (uint64, error) {
	reporter, err := t.getReporter()
	if err != nil {
		return 0, err
	}

	return reporter.HighestIndexedHeight()
}

func (t *TransactionResultsIndex) checkDataAvailable(height uint64) error {
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
