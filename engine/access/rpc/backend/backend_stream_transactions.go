package backend

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const TransactionExpiryForUnknownStatus = flow.DefaultTransactionExpiry + 10

// sendTransaction defines a function type for sending a transaction.
type sendTransaction func(ctx context.Context, tx *flow.TransactionBody) error

// backendSubscribeTransactions manages transaction subscriptions for monitoring transaction statuses.
// It provides functionalities to send transactions, subscribe to transaction status updates,
// and handle subscription lifecycles.
type backendSubscribeTransactions struct {
	log                 zerolog.Logger
	backendTransactions *backendTransactions
	subscriptionHandler *subscription.SubscriptionHandler
	blockTracker        subscription.BlockTracker
	sendTransaction     sendTransaction
}

// SendAndSubscribeTransactionStatuses sends a transaction and subscribes to its status updates.
//
// The subscription begins monitoring from the reference block specified in the transaction itself and
// streams updates until the transaction reaches a final state ([flow.TransactionStatusSealed] or [flow.TransactionStatusExpired]).
// Upon reaching a final state, the subscription automatically terminates.
//
// Parameters:
//   - ctx: The context to manage the transaction sending and subscription lifecycle, including cancellation.
//   - tx: The transaction body to be sent and monitored.
//   - requiredEventEncodingVersion: The version of event encoding required for the subscription.
//
// If the transaction cannot be sent, the subscription will fail and return a failed subscription.
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

// SubscribeTransactionStatuses subscribes to status updates for a given transaction ID.
//
// The subscription starts monitoring from the last sealed block. Updates are streamed
// until the transaction reaches a final state ([flow.TransactionStatusSealed] or [flow.TransactionStatusExpired]).
// The subscription terminates automatically once the final state is reached.
//
// Parameters:
//   - ctx: The context to manage the subscription's lifecycle, including cancellation.
//   - txID: The unique identifier of the transaction to monitor.
//   - requiredEventEncodingVersion: The version of event encoding required for the subscription.
func (b *backendSubscribeTransactions) SubscribeTransactionStatuses(
	ctx context.Context,
	txID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	header, err := b.backendTransactions.state.Sealed().Head()
	if err != nil {
		// throw the exception as the node must have the current sealed block in storage
		irrecoverable.Throw(ctx, fmt.Errorf("c: %w", err))
		return subscription.NewFailedSubscription(err, "failed to subscribe to transaction")
	}

	return b.createSubscription(ctx, txID, header.ID(), flow.ZeroID, requiredEventEncodingVersion)
}

// createSubscription initializes a transaction subscription for monitoring status updates.
//
// The subscription monitors the transaction's progress starting from the specified block ID.
// It streams updates until the transaction reaches a final state or an error occurs.
//
// Parameters:
//   - ctx: Context to manage the subscription lifecycle.
//   - txID: The unique identifier of the transaction to monitor.
//   - startBlockID: The ID of the block to start monitoring from.
//   - referenceBlockID: The ID of the transaction's reference block.
//   - requiredEventEncodingVersion: The required version of event encoding.
//
// Returns:
//   - subscription.Subscription: A subscription for monitoring transaction status updates.
//
// If the start height cannot be determined or current transaction state cannot be determined, a failed subscription is returned.
func (b *backendSubscribeTransactions) createSubscription(
	ctx context.Context,
	txID flow.Identifier,
	startBlockID flow.Identifier,
	referenceBlockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	// Determine the height of the block to start the subscription from.
	startHeight, err := b.blockTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		b.log.Err(err).Str("block_id", startBlockID.String()).Msg("failed to get start height")
		return subscription.NewFailedSubscription(err, "failed to get start height")
	}

	// Retrieve the current state of the transaction.
	txInfo, err := newTransactionSubscriptionMetadata(ctx, b.backendTransactions, txID, referenceBlockID, requiredEventEncodingVersion)
	if err != nil {
		b.log.Err(err).Str("tx_id", txID.String()).Msg("failed to get current transaction state")
		return subscription.NewFailedSubscription(err, "failed to get tx reference block ID")
	}

	return b.subscriptionHandler.Subscribe(ctx, startHeight, b.getTransactionStatusResponse(txInfo, startHeight))
}

// getTransactionStatusResponse returns a callback function that produces transaction status
// subscription responses based on new blocks.
func (b *backendSubscribeTransactions) getTransactionStatusResponse(
	txInfo *transactionSubscriptionMetadata,
	startHeight uint64,
) func(context.Context, uint64) (interface{}, error) {
	triggerMissingStatusesOnce := atomic.NewBool(false)

	return func(ctx context.Context, height uint64) (interface{}, error) {
		err := b.checkBlockReady(height)
		if err != nil {
			return nil, err
		}

		if triggerMissingStatusesOnce.CompareAndSwap(false, true) {
			return b.generateResultsStatuses(txInfo.txResult, flow.TransactionStatusUnknown)
		}

		if txInfo.txResult.IsFinal() {
			return nil, fmt.Errorf("transaction final status %s already reported: %w", txInfo.txResult.Status.String(), subscription.ErrEndOfData)
		}

		heightDiff := height - startHeight
		hasReachedUnknownStatusLimit := txInfo.txResult.Status == flow.TransactionStatusUnknown && heightDiff >= TransactionExpiryForUnknownStatus
		if hasReachedUnknownStatusLimit {
			txInfo.txResult.Status = flow.TransactionStatusExpired
			return b.generateResultsStatuses(txInfo.txResult, flow.TransactionStatusUnknown)
		}

		// Get old status here, as it could be replaced by status from founded tx result
		prevTxStatus := txInfo.txResult.Status

		if err = txInfo.Refresh(ctx, height); err != nil {
			if errors.Is(err, subscription.ErrBlockNotReady) {
				return nil, err
			}

			return nil, status.Errorf(codes.Internal, "failed to refresh transaction information: %v", err)
		}

		return b.generateResultsStatuses(txInfo.txResult, prevTxStatus)
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

// generateResultsStatuses checks if the current result differs from the previous result by more than one step.
// If yes, it generates results for the missing transaction statuses. This is done because the subscription should send
// responses for each of the statuses in the transaction lifecycle, and the message should be sent in the order of transaction statuses.
// Possible orders of transaction statuses:
// 1. pending(1) -> finalized(2) -> executed(3) -> sealed(4)
// 2. pending(1) -> expired(5)
// No errors expected during normal operations.
func (b *backendSubscribeTransactions) generateResultsStatuses(
	txResult *access.TransactionResult,
	prevTxStatus flow.TransactionStatus,
) ([]*access.TransactionResult, error) {
	// If the old and new transaction statuses are still the same, the status change should not be reported, so
	// return here with no response.
	if prevTxStatus == txResult.Status {
		return nil, nil
	}

	// If the previous status is pending or unknown and the new status is expired, which is the last status, return its result.
	// If the previous status is anything other than pending, return an error, as this transition is unexpected.
	if txResult.Status == flow.TransactionStatusExpired {
		if prevTxStatus == flow.TransactionStatusPending || prevTxStatus == flow.TransactionStatusUnknown {
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
