package backend

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// sendTransaction defines a function type for sending a transaction.
type sendTransaction func(ctx context.Context, tx *flow.TransactionBody) error

// backendSubscribeTransactions handles transaction subscriptions.
type backendSubscribeTransactions struct {
	backendTransactions *backendTransactions
	executionResults    storage.ExecutionResults
	log                 zerolog.Logger

	subscriptionHandler *subscription.SubscriptionHandler
	blockTracker        subscription.BlockTracker
	sendTransaction     sendTransaction
}

// SendAndSubscribeTransactionStatuses sends a transaction and subscribes to its status updates.
// It starts monitoring the status from the transaction's reference block ID.
// If the transaction cannot be sent or an error occurs during subscription creation, a failed subscription is returned.
func (b *backendSubscribeTransactions) SendAndSubscribeTransactionStatuses(
	ctx context.Context,
	tx *flow.TransactionBody,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	if err := b.sendTransaction(ctx, tx); err != nil {
		b.log.Err(err).Str("tx_id", tx.ID().String()).Msg("failed to send transaction")
		return subscription.NewFailedSubscription(err, "failed to send transaction")
	}

	return b.createSubscription(ctx, tx.ID(), tx.ReferenceBlockID, tx.ReferenceBlockID, requiredEventEncodingVersion)
}

// SubscribeTransactionStatuses subscribes to the status updates of a transaction.
// If the block cannot be retrieved or an error occurs during subscription creation, a failed subscription is returned.
func (b *backendSubscribeTransactions) SubscribeTransactionStatuses(
	ctx context.Context,
	txID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	header, err := b.backendTransactions.state.Sealed().Head()
	if err != nil {
		// throw the exception as the node must have the current sealed block in storage
		irrecoverable.Throw(ctx, err)
		return subscription.NewFailedSubscription(err, "failed to subscribe to transaction")
	}

	return b.createSubscription(ctx, txID, header.ID(), flow.ZeroID, requiredEventEncodingVersion)
}

// createSubscription initializes a subscription for monitoring a transaction's status.
// If the start height cannot be determined, a failed subscription is returned.
func (b *backendSubscribeTransactions) createSubscription(
	ctx context.Context,
	txID flow.Identifier,
	startBlockID flow.Identifier,
	referenceBlockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	// Get height to start subscription from
	startHeight, err := b.blockTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		b.log.Err(err).Str("block_id", startBlockID.String()).Msg("failed to get start height")
		return subscription.NewFailedSubscription(err, "failed to get start height")
	}

	txInfo, err := b.createTransactionInfoWithCurrentState(ctx, txID, referenceBlockID, startHeight, requiredEventEncodingVersion)
	if err != nil {
		b.log.Err(err).Str("txID", txID.String()).Msg("failed to get current transaction state")
		return subscription.NewFailedSubscription(err, "failed to get tx reference block ID")
	}

	return b.subscriptionHandler.Subscribe(ctx, startHeight, b.getTransactionStatusResponse(txInfo))
}

// createTransactionInfoWithCurrentState retrieves the current state of a transaction result and fills the transaction subscription metadata.
func (b *backendSubscribeTransactions) createTransactionInfoWithCurrentState(
	ctx context.Context,
	txID flow.Identifier,
	referenceBlockID flow.Identifier,
	startHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*transactionSubscriptionMetadata, error) {
	txInfo := newTransactionSubscriptionMetadata(b.backendTransactions, txID, referenceBlockID, requiredEventEncodingVersion)

	if err := txInfo.initTransactionReferenceBlockID(); err != nil {
		return nil, err
	}

	if err := txInfo.initBlockInfoFromBlocksRange(startHeight); err != nil {
		return nil, err
	}

	if err := txInfo.initTransactionResult(ctx); err != nil {
		return nil, err
	}

	return txInfo, nil
}

// getTransactionStatusResponse returns a callback function that produces transaction status
// subscription responses based on new blocks.
func (b *backendSubscribeTransactions) getTransactionStatusResponse(
	txInfo *transactionSubscriptionMetadata,
) func(context.Context, uint64) (interface{}, error) {
	triggerMissingStatusesOnce := atomic.NewBool(false)

	return func(ctx context.Context, height uint64) (interface{}, error) {
		err := b.checkBlockReady(height)
		if err != nil {
			return nil, err
		}

		if triggerMissingStatusesOnce.CompareAndSwap(false, true) {
			return b.generateResultsWithMissingStatuses(txInfo.TransactionResult, flow.TransactionStatusUnknown)
		}

		if txInfo.IsFinal() {
			return nil, fmt.Errorf("transaction final status %s already reported: %w", txInfo.Status.String(), subscription.ErrEndOfData)
		}

		// If on this step transaction block not available, search for it.
		if txInfo.blockWithTx == nil {
			// Search for transaction`s block information.
			err = txInfo.searchForTransactionInBlock(height)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return nil, fmt.Errorf("could not find block %d in storage: %w", height, subscription.ErrBlockNotReady)
				}

				if !errors.Is(err, ErrTransactionNotInBlock) {
					return nil, status.Errorf(codes.Internal, "could not get block %d: %v", height, err)
				}
			}
		}

		// Get old status here, as it could be replaced by status from founded tx result
		prevTxStatus := txInfo.Status

		if txInfo.blockWithTx != nil && !txInfo.IsExecuted() {
			txResult, err := txInfo.searchForTransactionResult(ctx)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get execution result for block %s: %v", txInfo.BlockID, err)
			}

			// If transaction result was found, fully replace it in metadata. New transaction status already included in result.
			if txResult != nil {
				txInfo.TransactionResult = txResult
			}
		}

		err = txInfo.deriveTransactionResult(ctx)
		if err != nil {
			return nil, err
		}

		// If the old and new transaction statuses are still the same, the status change should not be reported, so
		// return here with no response.
		if prevTxStatus == txInfo.Status {
			return nil, nil
		}

		return b.generateResultsWithMissingStatuses(txInfo.TransactionResult, prevTxStatus)
	}
}

// checkBlockReady checks if the given block height is valid and available based on the expected block status.
// Expected errors during normal operation:
// - subscription.ErrBlockNotReady: block for the given block height is not available.
func (b *backendSubscribeTransactions) checkBlockReady(height uint64) error {
	// Get the highest available finalized block height
	highestHeight, err := b.blockTracker.GetHighestHeight(flow.BlockStatusFinalized)
	if err != nil {
		return fmt.Errorf("could not get highest height for block %d: %w", height, err)
	}

	// Fail early if no block finalized notification has been received for the given height.
	// Note: It's possible that the block is locally finalized before the notification is
	// received. This ensures a consistent view is available to all streams.
	if height > highestHeight {
		return fmt.Errorf("block %d is not available yet: %w", height, subscription.ErrBlockNotReady)
	}

	return nil
}

// generateResultsWithMissingStatuses checks if the current result differs from the previous result by more than one step.
// If yes, it generates results for the missing transaction statuses. This is done because the subscription should send
// responses for each of the statuses in the transaction lifecycle, and the message should be sent in the order of transaction statuses.
// Possible orders of transaction statuses:
// 1. pending(1) -> finalized(2) -> executed(3) -> sealed(4)
// 2. pending(1) -> expired(5)
// No errors expected during normal operations.
func (b *backendSubscribeTransactions) generateResultsWithMissingStatuses(
	txResult *access.TransactionResult,
	prevTxStatus flow.TransactionStatus,
) ([]*access.TransactionResult, error) {
	// If the previous status is pending and the new status is expired, which is the last status, return its result.
	// If the previous status is anything other than pending, return an error, as this transition is unexpected.
	if txResult.Status == flow.TransactionStatusExpired {
		if prevTxStatus == flow.TransactionStatusPending {
			return []*access.TransactionResult{
				txResult,
			}, nil
		} else {
			return nil, fmt.Errorf("unexpected transition from %s to %s transaction status", prevTxStatus.String(), txResult.Status.String())
		}
	}

	var results []*access.TransactionResult

	// If the difference between statuses' values is more than one step, fill in the missing results.
	if (txResult.Status - prevTxStatus) > 1 {
		for missingStatus := prevTxStatus + 1; missingStatus < txResult.Status; missingStatus++ {
			switch missingStatus {
			case flow.TransactionStatusPending:
				results = append(results, &access.TransactionResult{
					Status:        missingStatus,
					TransactionID: txResult.TransactionID,
				})
			case flow.TransactionStatusFinalized:
				results = append(results, &access.TransactionResult{
					Status:        missingStatus,
					TransactionID: txResult.TransactionID,
					BlockID:       txResult.BlockID,
					BlockHeight:   txResult.BlockHeight,
					CollectionID:  txResult.CollectionID,
				})
			case flow.TransactionStatusExecuted:
				missingTxResult := *txResult
				missingTxResult.Status = missingStatus
				results = append(results, &missingTxResult)
			default:
				return nil, fmt.Errorf("unexpected missing transaction status")
			}
		}
	}

	results = append(results, txResult)
	return results, nil
}
