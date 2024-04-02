package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// backendSubscribeTransactions handles transaction subscriptions.
type backendSubscribeTransactions struct {
	txLocalDataProvider *TransactionsLocalDataProvider
	executionResults    storage.ExecutionResults
	log                 zerolog.Logger
	broadcaster         *engine.Broadcaster
	sendTimeout         time.Duration
	responseLimit       float64
	sendBufferSize      int

	blockTracker subscription.BlockTracker
}

// TransactionSubscriptionMetadata holds data representing the status state for each transaction subscription.
type TransactionSubscriptionMetadata struct {
	txResult           *access.TransactionResult
	txReferenceBlockID flow.Identifier
	messageIndex       counters.StrictMonotonousCounter
	blockWithTx        *flow.Header
	txExecuted         bool
}

// SubscribeTransactionStatuses subscribes to transaction status changes starting from the transaction reference block ID.
// If invalid tx parameters will be supplied SubscribeTransactionStatuses will return a failed subscription.
func (b *backendSubscribeTransactions) SubscribeTransactionStatuses(ctx context.Context, tx *flow.TransactionBody) subscription.Subscription {
	nextHeight, err := b.blockTracker.GetStartHeightFromBlockID(tx.ReferenceBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	txInfo := TransactionSubscriptionMetadata{
		txResult: &access.TransactionResult{
			TransactionID: tx.ID(),
			BlockID:       flow.ZeroID,
			Status:        flow.TransactionStatusUnknown,
		},
		txReferenceBlockID: tx.ReferenceBlockID,
		messageIndex:       counters.NewMonotonousCounter(0),
		blockWithTx:        nil,
	}

	sub := subscription.NewHeightBasedSubscription(
		b.sendBufferSize,
		nextHeight,
		b.getTransactionStatusResponse(&txInfo),
	)

	go subscription.NewStreamer(b.log, b.broadcaster, b.sendTimeout, b.responseLimit, sub).Stream(ctx)

	return sub
}

// getTransactionStatusResponse returns a callback function that produces transaction status
// subscription responses based on new blocks.
func (b *backendSubscribeTransactions) getTransactionStatusResponse(txInfo *TransactionSubscriptionMetadata) func(context.Context, uint64) (interface{}, error) {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		// Get the highest available finalized block height
		highestHeight, err := b.blockTracker.GetHighestHeight(flow.BlockStatusFinalized)
		if err != nil {
			return nil, fmt.Errorf("could not get highest height for block %d: %w", height, err)
		}

		// Fail early if no block finalized notification has been received for the given height.
		// Note: It's possible that the block is locally finalized before the notification is
		// received. This ensures a consistent view is available to all streams.
		if height > highestHeight {
			return nil, fmt.Errorf("block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
		}

		// If the transaction status already reported the final status, return with no data available
		if txInfo.txResult.Status == flow.TransactionStatusSealed || txInfo.txResult.Status == flow.TransactionStatusExpired {
			return nil, fmt.Errorf("transaction final status %s was already reported: %w", txInfo.txResult.Status.String(), subscription.ErrEndOfData)
		}

		// If on this step transaction block not available, search for it.
		if txInfo.blockWithTx == nil {
			// Search for transaction`s block information.
			txInfo.blockWithTx,
				txInfo.txResult.BlockID,
				txInfo.txResult.BlockHeight,
				txInfo.txResult.CollectionID,
				err = b.searchForTransactionBlockInfo(height, txInfo)

			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return nil, fmt.Errorf("could not find block %d in storage: %w", height, subscription.ErrBlockNotReady)
				}

				if !errors.Is(err, ErrTransactionNotInBlock) {
					return nil, status.Errorf(codes.Internal, "could not get block %d: %v", height, err)
				}
			}
		}

		// Find the transaction status.
		var txStatus flow.TransactionStatus
		var txResult *access.TransactionResult

		// If block with transaction was not found, get transaction status to check if it different from last status
		if txInfo.blockWithTx == nil {
			txStatus, err = b.txLocalDataProvider.DeriveUnknownTransactionStatus(txInfo.txReferenceBlockID)
		} else {
			// Check, if transaction executed and transaction result already available
			if !txInfo.txExecuted {
				txResult, err = b.searchForTransactionResult(ctx, txInfo.txResult.BlockID, txInfo.txResult.TransactionID)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "failed to get execution result for block %s: %v", txInfo.txResult.BlockID, err)
				}
				//Fill in execution status for future usages
				txInfo.txExecuted = txResult != nil
			}

			// If transaction result was found, fully replace it in metadata. New transaction status already included in result.
			if txResult != nil {
				txInfo.txResult = txResult
			} else {
				//If transaction result was not found or already filed in, get transaction status to check if it different from last status
				txStatus, err = b.txLocalDataProvider.DeriveTransactionStatus(txInfo.txResult.BlockID, txInfo.blockWithTx.Height, txInfo.txExecuted)
			}
		}
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		// The same transaction status should not be reported, so return here with no response
		if txInfo.txResult.Status == txStatus {
			return nil, nil
		}

		// If the current transaction status different from the last available status, assign new status to result
		if txResult == nil {
			txInfo.txResult.Status = txStatus
		}

		messageIndex := txInfo.messageIndex.Value()
		if ok := txInfo.messageIndex.Set(messageIndex + 1); !ok {
			return nil, status.Errorf(codes.Internal, "the message index has already been incremented to %d", txInfo.messageIndex.Value())
		}

		return &access.TransactionSubscribeInfo{
			Result:       txInfo.txResult,
			MessageIndex: messageIndex,
		}, nil
	}
}

// searchForTransactionBlockInfo searches for the block containing the specified transaction.
// It retrieves the block at the given height and checks if the transaction is included in that block.
// Expected errors:
// - subscription.ErrBlockNotReady when unable to retrieve the block or collection ID
// - codes.Internal when other errors occur during block or collection lookup
func (b *backendSubscribeTransactions) searchForTransactionBlockInfo(
	height uint64,
	txInfo *TransactionSubscriptionMetadata,
) (*flow.Header, flow.Identifier, uint64, flow.Identifier, error) {
	block, err := b.txLocalDataProvider.blocks.ByHeight(height)
	if err != nil {
		return nil, flow.ZeroID, 0, flow.ZeroID, fmt.Errorf("error looking up block: %w", err)
	}

	collectionID, err := b.txLocalDataProvider.LookupCollectionIDInBlock(block, txInfo.txResult.TransactionID)
	if err != nil {
		return nil, flow.ZeroID, 0, flow.ZeroID, fmt.Errorf("error looking up transaction in block: %w", err)
	}

	if collectionID != flow.ZeroID {
		return block.Header, block.ID(), height, collectionID, nil
	}

	return nil, flow.ZeroID, 0, flow.ZeroID, nil
}

// searchForTransactionResult searches for the execution result of a block. It retrieves the execution result for the specified block ID.
// Expected errors:
// - codes.Internal if an internal error occurs while retrieving execution result.
func (b *backendSubscribeTransactions) searchForTransactionResult(
	ctx context.Context,
	blockID flow.Identifier,
	txID flow.Identifier,
) (*access.TransactionResult, error) {
	_, err := b.executionResults.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get execution result for block %s: %w", blockID, err)
	}

	block, err := b.txLocalDataProvider.blocks.ByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("error looking up block: %w", err)
	}

	txResult, err := b.txLocalDataProvider.GetTransactionResultFromStorage(ctx, block, txID, entities.EventEncodingVersion_CCF_V0)
	if err != nil {
		// if either the storage or execution node reported no results or there were not enough execution results
		if status.Code(err) == codes.NotFound {
			// No result yet, indicate that it has not been executed
			return nil, nil
		}
		// Other Error trying to retrieve the result, return with err
		return nil, err
	}

	return txResult, nil
}
