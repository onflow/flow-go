package chained

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type ChainedResults struct {
	first  storage.ExecutionResultsReader
	second storage.ExecutionResultsReader
}

var _ storage.ExecutionResultsReader = (*ChainedResults)(nil)

// NewResults returns a new ChainedResults results store, which will handle reads. Any writes query
// will return err
// for reads, it first query first database, then second database, this is useful when migrating
// data from badger to pebble
func NewExecutionResults(first storage.ExecutionResultsReader, second storage.ExecutionResultsReader) *ChainedResults {
	return &ChainedResults{
		first:  first,
		second: second,
	}
}

func (c *ChainedResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := c.first.ByID(resultID)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return c.second.ByID(resultID)
	}

	return nil, err
}

func (c *ChainedResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := c.first.ByBlockID(blockID)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return c.second.ByBlockID(blockID)
	}

	return nil, err
}

func (c *ChainedResults) ByIDTx(resultID flow.Identifier) func(*transaction.Tx) (*flow.ExecutionResult, error) {
	return func(tx *transaction.Tx) (*flow.ExecutionResult, error) {
		result, err := c.first.ByIDTx(resultID)(tx)
		if err == nil {
			return result, nil
		}

		if errors.Is(err, storage.ErrNotFound) {
			return c.second.ByIDTx(resultID)(tx)
		}

		return nil, err
	}
}
