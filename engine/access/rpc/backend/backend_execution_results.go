package backend

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type backendExecutionResults struct {
	executionResults storage.ExecutionResults
}

func (b *backendExecutionResults) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	er, err := b.executionResults.ByBlockID(blockID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	return er, nil
}

// GetExecutionResultByID gets an execution result by its ID.
func (b *backendExecutionResults) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := b.executionResults.ByID(id)
	if err != nil {
		return nil, convertStorageError(err)
	}

	return result, nil
}
