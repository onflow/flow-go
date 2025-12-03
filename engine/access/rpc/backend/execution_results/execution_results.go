package execution_results

import (
	"context"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// ExecutionResults provides access to execution results stored in the database.
type ExecutionResults struct {
	executionResults storage.ExecutionResults
	seals            storage.Seals
}

// NewExecutionResults creates a new ExecutionResults instance.
func NewExecutionResults(executionResults storage.ExecutionResults) *ExecutionResults {
	return &ExecutionResults{
		executionResults: executionResults,
	}
}

// GetExecutionResultForBlockID gets an execution result by its block ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError]: No execution result with the given block ID was found.
func (e *ExecutionResults) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	// Query seal by blockID
	seal, err := e.seals.FinalizedSealForBlock(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("seal", err)
	}

	// Query result by seal.ResultID
	result, err := e.executionResults.ByID(seal.ResultID)
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
//   - [access.DataNotFoundError]: No execution result with the given ID was found.
func (e *ExecutionResults) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := e.executionResults.ByID(id)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("execution result", err)
	}

	return result, nil
}
