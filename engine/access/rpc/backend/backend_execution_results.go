package backend

import (
	"context"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type backendExecutionResults struct {
	executionResults storage.ExecutionResults
}

func (b *backendExecutionResults) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := b.executionResults.ByBlockID(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("execution result", err)
	}

	return result, nil
}

// GetExecutionResultByID gets an execution result by its ID.
func (b *backendExecutionResults) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := b.executionResults.ByID(id)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("execution result", err)
	}

	return result, nil
}
