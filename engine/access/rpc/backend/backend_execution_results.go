package backend

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type backendExecutionResults struct {
	executionResults storage.ExecutionResults
	seals            storage.Seals
	receipts         storage.ExecutionReceipts
}

func (b *backendExecutionResults) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	// Query seal by blockID
	seal, err := b.seals.FinalizedSealForBlock(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	// Query result by seal.ResultID
	result, err := b.executionResults.ByID(seal.ResultID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	return result, nil
}

// GetExecutionResultByID gets an execution result by its ID.
func (b *backendExecutionResults) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	result, err := b.executionResults.ByID(id)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	return result, nil
}

// GetExecutionReceiptsByBlockID retrieves all known execution receipts for the given block.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if no receipts are indexed for the given block ID.
func (b *backendExecutionResults) GetExecutionReceiptsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*flow.ExecutionReceipt, error) {
	// the block/receipt index is populated by the Access ingestion engine when receiving receipts
	// directly from execution nodes, and by the follower engine when persisting blocks.
	receipts, err := b.receipts.ByBlockID(blockID)
	if err != nil {
		// ByBlockID does not return an error if no receipts are found
		err = fmt.Errorf("failed to get execution receipts by block ID: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	if len(receipts) == 0 {
		return nil, status.Errorf(codes.NotFound, "no receipts found for block")
	}

	return receipts, nil
}

// GetExecutionReceiptsByResultID retrieves all known execution receipts that commit to the given
// execution result ID. It resolves the associated block ID from the result, then retrieves all
// receipts for that block, filtering to those matching the requested result.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if the execution result or its block's receipts are not found.
func (b *backendExecutionResults) GetExecutionReceiptsByResultID(ctx context.Context, resultID flow.Identifier) ([]*flow.ExecutionReceipt, error) {
	result, err := b.executionResults.ByID(resultID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			err = fmt.Errorf("failed to get execution result: %w", err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
		return nil, status.Errorf(codes.NotFound, "could not find execution result: %v", err)
	}

	// there is no receipt/result index, so we have to lookup the block and filter by result ID
	allReceipts, err := b.receipts.ByBlockID(result.BlockID)
	if err != nil {
		// ByBlockID does not return an error if no receipts are found for the given block.
		err = fmt.Errorf("failed to get execution receipts by result ID: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	var receipts []*flow.ExecutionReceipt
	for _, receipt := range allReceipts {
		if receipt.ExecutionResult.ID() == resultID {
			receipts = append(receipts, receipt)
		}
	}

	if len(receipts) == 0 {
		return nil, status.Errorf(codes.NotFound, "no receipts found for result")
	}

	return receipts, nil
}
