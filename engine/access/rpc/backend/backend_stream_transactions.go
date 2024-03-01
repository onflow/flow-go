package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"

	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"

	"github.com/onflow/flow-go/engine/common/rpc/convert"

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

// SendAndSubscribeTransactionStatuses subscribes to transaction status changes starting from the transaction reference block ID.
// Expected errors:
//   - subscription.NewFailedSubscription if any error returned by `b.getStartHeight` function
func (b *backendSubscribeTransactions) SendAndSubscribeTransactionStatuses(ctx context.Context, tx *flow.TransactionBody) subscription.Subscription {
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
			err := b.searchForTransactionBlock(height, txInfo)
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
				err := b.searchForTransactionResult(blockID, txInfo)
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

// searchForTransactionBlock searches for the block containing transaction information at the given height.
// It updates the TransactionSubscriptionMetadata with the found block if the transaction is present.
func (b *backendSubscribeTransactions) searchForTransactionBlock(
	height uint64,
	txInfo *TransactionSubscriptionMetadata,
) error {
	block, err := b.txLocalDataProvider.blocks.ByHeight(height)
	if err != nil {
		return status.Errorf(codes.Internal, "could not get block by collection ID: %v", err)
	}

	collectionID, err := b.txLocalDataProvider.LookupCollectionIDInBlock(block, txInfo.txID)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return status.Errorf(codes.Internal, "could not find transaction in block: %v", err)
		}
	}

	if collectionID != flow.ZeroID {
		txInfo.blockWithTx = block.Header
	}

	return nil
}

// searchForTransactionResult searches for the transaction result by block ID and transaction ID.
// It updates the TransactionSubscriptionMetadata with the execution status of the transaction.
func (b *backendSubscribeTransactions) searchForTransactionResult(
	blockID flow.Identifier,
	txInfo *TransactionSubscriptionMetadata,
) error {
	result, err := b.txLocalDataProvider.txResultsIndex.ByBlockIDTransactionID(blockID, txInfo.blockWithTx.Height, txInfo.txID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) &&
			!errors.Is(err, storage.ErrHeightNotIndexed) &&
			!errors.Is(err, indexer.ErrIndexNotInitialized) {
			return rpc.ConvertError(err, fmt.Sprintf("failed to get transaction result for block %s", blockID), codes.Internal)
		}
	}

	if result != nil {
		txInfo.txExecuted = true
	}

	return nil
}
