package index

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// TransactionResultsIndex implements a wrapper around `storage.LightTransactionResult` ensuring that needed data has been synced and is available to the client.
// Note: read detail how `Reporter` is working
type TransactionResultsIndex struct {
	*Reporter
	results storage.LightTransactionResults
}

func NewTransactionResultsIndex(reporter *Reporter, results storage.LightTransactionResults) *TransactionResultsIndex {
	return &TransactionResultsIndex{
		Reporter: reporter,
		results:  results,
	}
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
