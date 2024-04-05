package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/module/counters"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// backendSubscribeTransactions handles transaction subscriptions.
type backendSubscribeTransactions struct {
	txLocalDataProvider *TransactionsLocalDataProvider
	executionResults    storage.ExecutionResults
	log                 zerolog.Logger

	subscriptionHandler *subscription.SubscriptionHandler
	blockTracker        subscription.BlockTracker
}

// TransactionSubscriptionMetadata holds data representing the status state for each transaction subscription.
type TransactionSubscriptionMetadata struct {
	txID               flow.Identifier
	txReferenceBlockID flow.Identifier
	messageIndex       counters.StrictMonotonousCounter
	blockWithTx        *flow.Header
	blockID            flow.Identifier
	txExecuted         bool
	lastTxStatus       flow.TransactionStatus
}

// SubscribeTransactionStatuses subscribes to transaction status changes starting from the transaction reference block ID.
// If invalid tx parameters will be supplied SubscribeTransactionStatuses will return a failed subscription.
func (b *backendSubscribeTransactions) SubscribeTransactionStatuses(ctx context.Context, tx *flow.TransactionBody) subscription.Subscription {
	nextHeight, err := b.blockTracker.GetStartHeightFromBlockID(tx.ReferenceBlockID)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	txInfo := TransactionSubscriptionMetadata{
		txID:               tx.ID(),
		txReferenceBlockID: tx.ReferenceBlockID,
		messageIndex:       counters.NewMonotonousCounter(0),
		blockWithTx:        nil,
		blockID:            flow.ZeroID,
		lastTxStatus:       flow.TransactionStatusUnknown,
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getTransactionStatusResponse(&txInfo))
}

// getTransactionStatusResponse returns a callback function that produces transaction status
// subscription responses based on new blocks.
func (b *backendSubscribeTransactions) getTransactionStatusResponse(txInfo *TransactionSubscriptionMetadata) func(context.Context, uint64) (interface{}, error) {
	return func(ctx context.Context, height uint64) (interface{}, error) {
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

		if txInfo.lastTxStatus == flow.TransactionStatusSealed || txInfo.lastTxStatus == flow.TransactionStatusExpired {
			return nil, fmt.Errorf("transaction final status %s was already reported: %w", txInfo.lastTxStatus.String(), subscription.ErrEndOfData)
		}

		if txInfo.blockWithTx == nil {
			// Check if block contains transaction.
			txInfo.blockWithTx, txInfo.blockID, err = b.searchForTransactionBlock(height, txInfo)
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
		if txInfo.blockWithTx == nil {
			txStatus, err = b.txLocalDataProvider.DeriveUnknownTransactionStatus(txInfo.txReferenceBlockID)
		} else {
			if !txInfo.txExecuted {
				// Check if transaction was executed.
				txInfo.txExecuted, err = b.searchForExecutionResult(txInfo.blockID)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "failed to get execution result for block %s: %v", txInfo.blockID, err)
				}
			}

			txStatus, err = b.txLocalDataProvider.DeriveTransactionStatus(txInfo.blockID, txInfo.blockWithTx.Height, txInfo.txExecuted)
		}
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		// The same transaction status should not be reported, so return here with no response
		if txInfo.lastTxStatus == txStatus {
			return nil, nil
		}
		txInfo.lastTxStatus = txStatus

		messageIndex := txInfo.messageIndex.Value()
		if ok := txInfo.messageIndex.Set(messageIndex + 1); !ok {
			return nil, status.Errorf(codes.Internal, "the message index has already been incremented to %d", txInfo.messageIndex.Value())
		}

		return &convert.TransactionSubscribeInfo{
			ID:           txInfo.txID,
			Status:       txInfo.lastTxStatus,
			MessageIndex: messageIndex,
		}, nil
	}
}

// searchForTransactionBlock searches for the block containing the specified transaction.
// It retrieves the block at the given height and checks if the transaction is included in that block.
// Expected errors:
// - subscription.ErrBlockNotReady when unable to retrieve the block or collection ID
// - codes.Internal when other errors occur during block or collection lookup
func (b *backendSubscribeTransactions) searchForTransactionBlock(
	height uint64,
	txInfo *TransactionSubscriptionMetadata,
) (*flow.Header, flow.Identifier, error) {
	block, err := b.txLocalDataProvider.blocks.ByHeight(height)
	if err != nil {
		return nil, flow.ZeroID, fmt.Errorf("error looking up block: %w", err)
	}

	collectionID, err := b.txLocalDataProvider.LookupCollectionIDInBlock(block, txInfo.txID)
	if err != nil {
		return nil, flow.ZeroID, fmt.Errorf("error looking up transaction in block: %w", err)
	}

	if collectionID != flow.ZeroID {
		return block.Header, block.ID(), nil
	}

	return nil, flow.ZeroID, nil
}

// searchForExecutionResult searches for the execution result of a block. It retrieves the execution result for the specified block ID.
// Expected errors:
// - codes.Internal if an internal error occurs while retrieving execution result.
func (b *backendSubscribeTransactions) searchForExecutionResult(
	blockID flow.Identifier,
) (bool, error) {
	_, err := b.executionResults.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get execution result for block %s: %w", blockID, err)
	}

	return true, nil
}
