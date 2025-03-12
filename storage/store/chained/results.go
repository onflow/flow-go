package chained

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type ChainedExecutionResults struct {
	first  storage.ExecutionResultsReader
	second storage.ExecutionResultsReader
}

var _ storage.ExecutionResultsReader = (*ChainedExecutionResults)(nil)

// NewResults returns a new ChainedExecutionResults results store, which will handle reads. which only implements
// read operations
// for reads, it first query the first database, then the second database, this is useful when migrating
// data from badger to pebble
func NewExecutionResults(first storage.ExecutionResultsReader, second storage.ExecutionResultsReader) *ChainedExecutionResults {
	return &ChainedExecutionResults{
		first:  first,
		second: second,
	}
}

func (c *ChainedExecutionResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := c.first.ByID(resultID)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return c.second.ByID(resultID)
	}

	return nil, err
}

func (c *ChainedExecutionResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := c.first.ByBlockID(blockID)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return c.second.ByBlockID(blockID)
	}

	return nil, err
}

func (c *ChainedExecutionResults) ByIDTx(resultID flow.Identifier) func(*transaction.Tx) (*flow.ExecutionResult, error) {
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
