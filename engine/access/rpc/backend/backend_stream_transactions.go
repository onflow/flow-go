package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/module/counters"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// backendSubscribeTransactions handles transaction subscriptions.
type backendSubscribeTransactions struct {
	txLocalDataProvider TransactionsLocalDataProvider
	executionResults    storage.ExecutionResults
	log                 zerolog.Logger
	broadcaster         *engine.Broadcaster
	sendTimeout         time.Duration
	responseLimit       float64
	sendBufferSize      int

	getStartHeight   subscription.GetStartHeightFunc
	getHighestHeight subscription.GetHighestHeight
}

// TransactionSubscriptionMetadata holds data representing the status state for each transaction subscription.
type TransactionSubscriptionMetadata struct {
	txID               flow.Identifier
	txReferenceBlockID flow.Identifier
	messageIndex       counters.StrictMonotonousCounter
	blockWithTx        *flow.Header
	txExecuted         bool
}

// SubscribeTransactionStatuses subscribes to transaction status changes starting from the transaction reference block ID.
// Expected errors:
// - storage.ErrNotFound if a block referenced by the transaction does not exist.
// - irrecoverable.Exception if there is an internal error related to state inconsistency.
func (b *backendSubscribeTransactions) SubscribeTransactionStatuses(ctx context.Context, tx *flow.TransactionBody) subscription.Subscription {
	nextHeight, err := b.getStartHeight(ctx, tx.ReferenceBlockID, 0)
	if err != nil {
		return subscription.NewFailedSubscription(err, "could not get start height")
	}

	txInfo := TransactionSubscriptionMetadata{
		txID:               tx.ID(),
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

// getTransactionStatusResponse creates a function for handling transaction status subscriptions based on new block and
// previous status state metadata.
func (b *backendSubscribeTransactions) getTransactionStatusResponse(txInfo *TransactionSubscriptionMetadata) func(context.Context, uint64) (interface{}, error) {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		highestHeight, err := b.getHighestHeight(flow.BlockStatusFinalized)
		if err != nil {
			return nil, fmt.Errorf("could not get highest height for block %d: %w", height, err)
		}

		// Fail early if no notification has been received for the given block height.
		// Note: It's possible for the data to exist in the data store before the notification is
		// received. This ensures a consistent view is available to all streams.
		if height > highestHeight {
			return nil, fmt.Errorf("block %d is not available yet: %w", height, storage.ErrNotFound)
		}

		if txInfo.blockWithTx == nil {
			// Check if block contains transaction.
			txInfo.blockWithTx, err = b.searchForTransactionBlock(height, txInfo)
			if err != nil {
				return nil, err
			}
		}

		// Find the transaction status.
		var txStatus flow.TransactionStatus
		if txInfo.blockWithTx == nil {
			txStatus, err = b.txLocalDataProvider.DeriveUnknownTransactionStatus(txInfo.txReferenceBlockID)
		} else {
			blockID := txInfo.blockWithTx.ID()

			if !txInfo.txExecuted {
				// Check if transaction was executed.
				txInfo.txExecuted, err = b.searchForExecutionResult(blockID)
				if err != nil {
					return nil, err
				}
			}

			txStatus, err = b.txLocalDataProvider.DeriveTransactionStatus(blockID, txInfo.blockWithTx.Height, txInfo.txExecuted)
		}
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		messageIndex := txInfo.messageIndex.Value()
		if ok := txInfo.messageIndex.Set(messageIndex + 1); !ok {
			return nil, status.Errorf(codes.Internal, "the message index has already been incremented to %d", txInfo.messageIndex.Value())
		}

		return &convert.TransactionSubscribeInfo{
			ID:           txInfo.txID,
			Status:       txStatus,
			MessageIndex: messageIndex,
		}, nil
	}
}

// searchForTransactionBlock searches for the block containing the specified transaction.
// It retrieves the block at the given height and checks if the transaction is included in that block.
// Expected errors:
// - codes.Internal if an internal error occurs while retrieving or processing the block.
func (b *backendSubscribeTransactions) searchForTransactionBlock(
	height uint64,
	txInfo *TransactionSubscriptionMetadata,
) (*flow.Header, error) {
	block, err := b.txLocalDataProvider.blocks.ByHeight(height)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get block %d: %v", height, err)
	}

	collectionID, err := b.txLocalDataProvider.LookupCollectionIDInBlock(block, txInfo.txID)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, status.Errorf(codes.Internal, "could not find transaction in block: %v", err)
		}
	}

	if collectionID != flow.ZeroID {
		return block.Header, nil
	}

	return nil, nil
}

// searchForExecutionResult searches for the execution result of a block. It retrieves the execution result for the specified block ID.
// Expected errors:
// - codes.Internal if an internal error occurs while retrieving execution result.
func (b *backendSubscribeTransactions) searchForExecutionResult(
	blockID flow.Identifier,
) (bool, error) {
	result, err := b.executionResults.ByBlockID(blockID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return false, rpc.ConvertError(err, fmt.Sprintf("failed to get execution result for block %s", blockID), codes.Internal)
		}
	}

	if result != nil {
		return true, nil
	}

	return false, nil
}
