package chained

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type ChainedTransactionResults struct {
	first  storage.TransactionResultsReader
	second storage.TransactionResultsReader
}

var _ storage.TransactionResultsReader = (*ChainedTransactionResults)(nil)

// NewTransactionResults returns a new ChainedTransactionResults transaction results store, which only implements
// read operations
// it first queries the first database, then the second database. this is useful when migrating
// data from badger to pebble
func NewTransactionResults(first storage.TransactionResultsReader, second storage.TransactionResultsReader) *ChainedTransactionResults {
	return &ChainedTransactionResults{
		first:  first,
		second: second,
	}
}

// ByBlockIDTransactionID returns the runtime transaction result for the given block ID and transaction ID
// It returns storage.ErrNotFound if the result is not found
// any other errors are exceptions
func (c *ChainedTransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResult, error) {
	result, err := c.first.ByBlockIDTransactionID(blockID, transactionID)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return c.second.ByBlockIDTransactionID(blockID, transactionID)
	}

	return nil, err
}

// ByBlockIDTransactionIndex returns the runtime transaction result for the given block ID and transaction index
// It returns storage.ErrNotFound if the result is not found
// any other errors are exceptions
func (c *ChainedTransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResult, error) {
	result, err := c.first.ByBlockIDTransactionIndex(blockID, txIndex)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return c.second.ByBlockIDTransactionIndex(blockID, txIndex)
	}

	return nil, err
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
// It returns storage.ErrNotFound if the result is not found
// any other errors are exceptions
func (c *ChainedTransactionResults) ByBlockID(id flow.Identifier) ([]flow.TransactionResult, error) {
	results, err := c.first.ByBlockID(id)
	if err != nil {
		return nil, err
	}

	if len(results) > 0 {
		return results, nil
	}

	return c.second.ByBlockID(id)
}
