package backend

import (
	"context"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type backendExecutionResults struct {
	executionResults storage.ExecutionResults
	seals            storage.Seals
}

// GetExecutionResultForBlockID gets an execution result by its block ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected sentinel errors providing details to clients about failed requests:
//   - access.DataNotFoundError - No execution result with the given block ID was found
func (b *backendExecutionResults) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	// Query seal by blockID
	seal, err := b.seals.FinalizedSealForBlock(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("seal", err)
	}

	// Query result by seal.ResultID
	result, err := b.executionResults.ByID(seal.ResultID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("execution result", err)
	}

	return result, nil
}

// GetExecutionResultByID gets an execution result by its ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected sentinel errors providing details to clients about failed requests:
//   - access.DataNotFoundError - No execution result with the given ID was found
func (b *backendExecutionResults) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := b.executionResults.ByID(id)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("execution result", err)
	}

	return result, nil
}
