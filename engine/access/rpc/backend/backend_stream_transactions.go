package backend

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// sendTransaction defines a function type for sending a transaction.
type sendTransaction func(ctx context.Context, tx *flow.TransactionBody) error

// backendSubscribeTransactions handles transaction subscriptions.
type backendSubscribeTransactions struct {
	txLocalDataProvider *TransactionsLocalDataProvider
	backendTransactions *backendTransactions
	executionResults    storage.ExecutionResults
	log                 zerolog.Logger

	subscriptionHandler *subscription.SubscriptionHandler
	blockTracker        subscription.BlockTracker
	sendTransaction     sendTransaction
}

// transactionSubscriptionMetadata holds data representing the status state for each transaction subscription.
type transactionSubscriptionMetadata struct {
	*access.TransactionResult
	txReferenceBlockID   flow.Identifier
	blockWithTx          *flow.Header
	txExecuted           bool
	eventEncodingVersion entities.EventEncodingVersion
	shouldTriggerPending bool
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
		b.log.Debug().Err(err).Str("tx_id", tx.ID().String()).Msg("failed to send transaction")
		return subscription.NewFailedSubscription(err, "failed to send transaction")
	}

	return b.createSubscription(ctx, tx.ID(), tx.ReferenceBlockID, 0, tx.ReferenceBlockID, requiredEventEncodingVersion, true)
}

// SubscribeTransactionStatusesFromStartHeight subscribes to the status updates of a transaction.
// Monitoring starts from the specified block height.
// If the block height cannot be determined or an error occurs during subscription creation, a failed subscription is returned.
func (b *backendSubscribeTransactions) SubscribeTransactionStatusesFromStartHeight(
	ctx context.Context,
	txID flow.Identifier,
	startHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	return b.createSubscription(ctx, txID, flow.ZeroID, startHeight, flow.ZeroID, requiredEventEncodingVersion, false)
}

// SubscribeTransactionStatusesFromStartBlockID subscribes to the status updates of a transaction.
// Monitoring starts from the specified block ID.
// If the block ID cannot be determined or an error occurs during subscription creation, a failed subscription is returned.
func (b *backendSubscribeTransactions) SubscribeTransactionStatusesFromStartBlockID(
	ctx context.Context,
	txID flow.Identifier,
	startBlockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	return b.createSubscription(ctx, txID, startBlockID, 0, flow.ZeroID, requiredEventEncodingVersion, false)
}

// SubscribeTransactionStatusesFromLatest subscribes to the status updates of a transaction.
// Monitoring starts from the latest block.
// If the block cannot be retrieved or an error occurs during subscription creation, a failed subscription is returned.
func (b *backendSubscribeTransactions) SubscribeTransactionStatusesFromLatest(
	ctx context.Context,
	txID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	header, err := b.txLocalDataProvider.state.Sealed().Head()
	if err != nil {
		b.log.Error().Err(err).Msg("failed to retrieve latest block")
		return subscription.NewFailedSubscription(err, "failed to retrieve latest block")
	}

	return b.createSubscription(ctx, txID, header.ID(), 0, flow.ZeroID, requiredEventEncodingVersion, false)
}

// createSubscription initializes a subscription for monitoring a transaction's status.
// If the start height cannot be determined, a failed subscription is returned.
func (b *backendSubscribeTransactions) createSubscription(
	ctx context.Context,
	txID flow.Identifier,
	startBlockID flow.Identifier,
	startBlockHeight uint64,
	referenceBlockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	shouldTriggerPending bool,
) subscription.Subscription {
	var nextHeight uint64
	var err error

	// Get height to start subscription from
	if startBlockID == flow.ZeroID {
		if nextHeight, err = b.blockTracker.GetStartHeightFromHeight(startBlockHeight); err != nil {
			b.log.Debug().Err(err).Uint64("block_height", startBlockHeight).Msg("failed to get start height")
			return subscription.NewFailedSubscription(err, "failed to get start height")
		}
	} else {
		if nextHeight, err = b.blockTracker.GetStartHeightFromBlockID(startBlockID); err != nil {
			b.log.Debug().Err(err).Str("block_id", startBlockID.String()).Msg("failed to get start height")
			return subscription.NewFailedSubscription(err, "failed to get start height")
		}
	}

	txInfo := transactionSubscriptionMetadata{
		TransactionResult: &access.TransactionResult{
			TransactionID: txID,
			BlockID:       flow.ZeroID,
		},
		txReferenceBlockID:   referenceBlockID,
		blockWithTx:          nil,
		eventEncodingVersion: requiredEventEncodingVersion,
		shouldTriggerPending: shouldTriggerPending,
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getTransactionStatusResponse(&txInfo))
}

// getTransactionStatusResponse returns a callback function that produces transaction status
// subscription responses based on new blocks.
func (b *backendSubscribeTransactions) getTransactionStatusResponse(txInfo *transactionSubscriptionMetadata) func(context.Context, uint64) (interface{}, error) {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		err := b.checkBlockReady(height)
		if err != nil {
			return nil, err
		}

		if txInfo.shouldTriggerPending {
			return b.handlePendingStatus(txInfo)
		}

		if b.isTransactionFinalStatus(txInfo) {
			return nil, fmt.Errorf("transaction final status %s already reported: %w", txInfo.Status.String(), subscription.ErrEndOfData)
		}

		// If on this step transaction block not available, search for it.
		if txInfo.blockWithTx == nil {
			// Search for transaction`s block information.
			txInfo.blockWithTx,
				txInfo.BlockID,
				txInfo.BlockHeight,
				txInfo.CollectionID,
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

		// Get old status here, as it could be replaced by status from founded tx result
		prevTxStatus := txInfo.Status

		// Check, if transaction executed and transaction result already available
		if txInfo.blockWithTx != nil && !txInfo.txExecuted {
			txResult, err := b.searchForTransactionResult(ctx, txInfo)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get execution result for block %s: %v", txInfo.BlockID, err)
			}

			// If transaction result was found, fully replace it in metadata. New transaction status already included in result.
			if txResult != nil {
				txInfo.TransactionResult = txResult
				//Fill in execution status for future usages
				txInfo.txExecuted = true
			}
		}

		// If block with transaction was not found, get transaction status to check if it different from last status
		if txInfo.Status, err = b.getTransactionStatus(ctx, txInfo, prevTxStatus); err != nil {
			return nil, err
		}

		// If the old and new transaction statuses are still the same, the status change should not be reported, so
		// return here with no response.
		if prevTxStatus == txInfo.Status {
			return nil, nil
		}

		return b.generateResultsWithMissingStatuses(txInfo, prevTxStatus)
	}
}

// handlePendingStatus handles the initial pending status for a transaction.
func (b *backendSubscribeTransactions) handlePendingStatus(txInfo *transactionSubscriptionMetadata) (interface{}, error) {
	txInfo.shouldTriggerPending = false
	// The status of the first pending transaction should be returned immediately, as the transaction has already been sent.
	// This should occur only once for each subscription.
	txInfo.Status = flow.TransactionStatusPending
	return b.generateResultsWithMissingStatuses(txInfo, flow.TransactionStatusUnknown)
}

// isTransactionFinalStatus checks if a transaction has reached a final state (Sealed or Expired).
func (b *backendSubscribeTransactions) isTransactionFinalStatus(txInfo *transactionSubscriptionMetadata) bool {
	return txInfo.Status == flow.TransactionStatusSealed || txInfo.Status == flow.TransactionStatusExpired
}

// getTransactionStatus determines the current status of a transaction based on its metadata
// and previous status. It  derives the transaction status by analyzing the transaction's
// execution block, if available, or its reference block.
//
// No errors expected during normal operations.
func (b *backendSubscribeTransactions) getTransactionStatus(ctx context.Context, txInfo *transactionSubscriptionMetadata, prevTxStatus flow.TransactionStatus) (flow.TransactionStatus, error) {
	txStatus := txInfo.Status
	var err error

	if txInfo.blockWithTx == nil {
		txStatus, err = b.txLocalDataProvider.DeriveUnknownTransactionStatus(txInfo.txReferenceBlockID)
	} else if txStatus == prevTxStatus {
		// When a block with the transaction is available, it is possible to receive a new transaction status while
		// searching for the transaction result. Otherwise, it remains unchanged. So, if the old and new transaction
		// statuses are the same, the current transaction status should be retrieved.
		txStatus, err = b.txLocalDataProvider.DeriveTransactionStatus(txInfo.blockWithTx.Height, txInfo.txExecuted)
	}

	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return flow.TransactionStatusUnknown, rpc.ConvertStorageError(err)
	}

	return txStatus, nil
}

// generateResultsWithMissingStatuses checks if the current result differs from the previous result by more than one step.
// If yes, it generates results for the missing transaction statuses. This is done because the subscription should send
// responses for each of the statuses in the transaction lifecycle, and the message should be sent in the order of transaction statuses.
// Possible orders of transaction statuses:
// 1. pending(1) -> finalized(2) -> executed(3) -> sealed(4)
// 2. pending(1) -> expired(5)
// No errors expected during normal operations.
func (b *backendSubscribeTransactions) generateResultsWithMissingStatuses(
	txInfo *transactionSubscriptionMetadata,
	prevTxStatus flow.TransactionStatus,
) ([]*access.TransactionResult, error) {
	// If the previous status is pending and the new status is expired, which is the last status, return its result.
	// If the previous status is anything other than pending, return an error, as this transition is unexpected.
	if txInfo.Status == flow.TransactionStatusExpired {
		if prevTxStatus == flow.TransactionStatusPending {
			return []*access.TransactionResult{
				txInfo.TransactionResult,
			}, nil
		} else {
			return nil, fmt.Errorf("unexpected transition from %s to %s transaction status", prevTxStatus.String(), txInfo.Status.String())
		}
	}

	var results []*access.TransactionResult

	// If the difference between statuses' values is more than one step, fill in the missing results.
	if (txInfo.Status - prevTxStatus) > 1 {
		for missingStatus := prevTxStatus + 1; missingStatus < txInfo.Status; missingStatus++ {
			switch missingStatus {
			case flow.TransactionStatusPending:
				results = append(results, &access.TransactionResult{
					Status:        missingStatus,
					TransactionID: txInfo.TransactionID,
				})
			case flow.TransactionStatusFinalized:
				results = append(results, &access.TransactionResult{
					Status:        missingStatus,
					TransactionID: txInfo.TransactionID,
					BlockID:       txInfo.BlockID,
					BlockHeight:   txInfo.BlockHeight,
					CollectionID:  txInfo.CollectionID,
				})
			case flow.TransactionStatusExecuted:
				missingTxResult := *txInfo.TransactionResult
				missingTxResult.Status = missingStatus
				results = append(results, &missingTxResult)
			default:
				return nil, fmt.Errorf("unexpected missing transaction status")
			}
		}
	}

	results = append(results, txInfo.TransactionResult)
	return results, nil
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

// searchForTransactionBlockInfo searches for the block containing the specified transaction.
// It retrieves the block at the given height and checks if the transaction is included in that block.
// Expected errors:
// - ErrTransactionNotInBlock when unable to retrieve the collection
// - codes.Internal when other errors occur during block or collection lookup
func (b *backendSubscribeTransactions) searchForTransactionBlockInfo(
	height uint64,
	txInfo *transactionSubscriptionMetadata,
) (*flow.Header, flow.Identifier, uint64, flow.Identifier, error) {
	block, err := b.txLocalDataProvider.blocks.ByHeight(height)
	if err != nil {
		return nil, flow.ZeroID, 0, flow.ZeroID, fmt.Errorf("error looking up block: %w", err)
	}

	collectionID, err := b.txLocalDataProvider.LookupCollectionIDInBlock(block, txInfo.TransactionID)
	if err != nil {
		return nil, flow.ZeroID, 0, flow.ZeroID, fmt.Errorf("error looking up transaction in block: %w", err)
	}

	if collectionID != flow.ZeroID {
		return block.Header, block.ID(), height, collectionID, nil
	}

	return nil, flow.ZeroID, 0, flow.ZeroID, nil
}

// searchForTransactionResult searches for the transaction result of a block. It retrieves the execution result for the specified block ID.
// Expected errors:
// - codes.Internal if an internal error occurs while retrieving execution result.
func (b *backendSubscribeTransactions) searchForTransactionResult(
	ctx context.Context,
	txInfo *transactionSubscriptionMetadata,
) (*access.TransactionResult, error) {
	_, err := b.executionResults.ByBlockID(txInfo.BlockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get execution result for block %s: %w", txInfo.BlockID, err)
	}

	txResult, err := b.backendTransactions.GetTransactionResult(
		ctx,
		txInfo.TransactionID,
		txInfo.BlockID,
		txInfo.CollectionID,
		txInfo.eventEncodingVersion,
	)

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
