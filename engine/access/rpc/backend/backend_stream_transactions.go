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
	eventEncodingVersion entities.EventEncodingVersion
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

	return b.createSubscription(ctx, tx.ID(), tx.ReferenceBlockID, tx.ReferenceBlockID, requiredEventEncodingVersion, true)
}

// SubscribeTransactionStatuses subscribes to the status updates of a transaction.
// If the block cannot be retrieved or an error occurs during subscription creation, a failed subscription is returned.
func (b *backendSubscribeTransactions) SubscribeTransactionStatuses(
	ctx context.Context,
	txID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) subscription.Subscription {
	header, err := b.txLocalDataProvider.state.Sealed().Head()
	if err != nil {
		// throw the exception as the node must have the current sealed block in storage
		irrecoverable.Throw(ctx, err)
	}

	return b.createSubscription(ctx, txID, header.ID(), flow.ZeroID, requiredEventEncodingVersion, false)
}

// createSubscription initializes a subscription for monitoring a transaction's status.
// If the start height cannot be determined, a failed subscription is returned.
func (b *backendSubscribeTransactions) createSubscription(
	ctx context.Context,
	txID flow.Identifier,
	startBlockID flow.Identifier,
	referenceBlockID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	shouldTriggerPending bool,
) subscription.Subscription {
	// Get height to start subscription from
	startHeight, err := b.blockTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		b.log.Debug().Err(err).Str("block_id", startBlockID.String()).Msg("failed to get start height")
		return subscription.NewFailedSubscription(err, "failed to get start height")
	}

	txInfo, err := b.fillCurrentTransactionResultState(ctx, txID, referenceBlockID, startHeight, requiredEventEncodingVersion)
	if err != nil {
		b.log.Debug().Err(err).Str("tID", startBlockID.String()).Msg("failed to get current transaction state")
		return subscription.NewFailedSubscription(err, "failed to get tx reference block ID")
	}

	return b.subscriptionHandler.Subscribe(ctx, startHeight, b.getTransactionStatusResponse(txInfo, shouldTriggerPending, txInfo.IsFinal()))
}

// fillCurrentTransactionResultState retrieves the current state of a transaction result and fills the transaction subscription metadata.
// TODO: Add more comments in code
func (b *backendSubscribeTransactions) fillCurrentTransactionResultState(
	ctx context.Context,
	txID flow.Identifier,
	referenceBlockID flow.Identifier,
	startHeight uint64,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*transactionSubscriptionMetadata, error) {
	// Get referenceBlockID if it is not set
	if referenceBlockID == flow.ZeroID {
		tx, err := b.backendTransactions.transactions.ByID(txID)
		if err != nil {
			return nil, err
		}
		referenceBlockID = tx.ReferenceBlockID
	}

	refBlock, err := b.txLocalDataProvider.blocks.ByID(referenceBlockID)
	if err != nil {
		return nil, err
	}

	var blockWithTx *flow.Header
	var blockId flow.Identifier
	var blockHeight uint64
	var collectionID flow.Identifier

	for height := refBlock.Header.Height; height <= startHeight; height++ {
		blockWithTx, blockId, blockHeight, collectionID, err = b.searchForTransactionBlock(height, txID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) ||
				errors.Is(err, ErrTransactionNotInBlock) {
				continue
			}

			return nil, err
		}

		if blockWithTx != nil {
			break
		}
	}

	var txResult *access.TransactionResult

	if blockWithTx == nil {
		txResult = &access.TransactionResult{
			TransactionID: txID,
		}

		txResult.Status, err = b.txLocalDataProvider.DeriveUnknownTransactionStatus(referenceBlockID)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, err
		}
	} else {
		txResult, err = b.searchForTransactionResult(ctx, txID, blockWithTx, requiredEventEncodingVersion)
		if err != nil {
			return nil, err
		}

		if txResult == nil {
			// If we've gotten here, but the block has not yet been executed, report it as only been finalized
			txResult = &access.TransactionResult{
				TransactionID: txID,
				BlockID:       blockId,
				BlockHeight:   blockHeight,
				Status:        flow.TransactionStatusFinalized,
				CollectionID:  collectionID,
			}
		}
	}

	return &transactionSubscriptionMetadata{
		TransactionResult:    txResult,
		txReferenceBlockID:   referenceBlockID,
		blockWithTx:          blockWithTx,
		eventEncodingVersion: requiredEventEncodingVersion,
	}, nil
}

// getTransactionStatusResponse returns a callback function that produces transaction status
// subscription responses based on new blocks.
func (b *backendSubscribeTransactions) getTransactionStatusResponse(
	txInfo *transactionSubscriptionMetadata,
	shouldTriggerPending bool,
	shouldTriggerFinalized bool,
) func(context.Context, uint64) (interface{}, error) {
	triggerPendingOnce := atomic.NewBool(false)
	triggerFinalizedOnce := atomic.NewBool(false)

	return func(ctx context.Context, height uint64) (interface{}, error) {
		err := b.checkBlockReady(height)
		if err != nil {
			return nil, err
		}

		// TODO: If possible, this should be simplified, as we use the same pattern as for Pending
		if shouldTriggerFinalized && triggerFinalizedOnce.CompareAndSwap(false, true) {
			return b.generateResultsWithMissingStatuses(txInfo.TransactionResult, flow.TransactionStatusUnknown)
		}

		if shouldTriggerPending && triggerPendingOnce.CompareAndSwap(false, true) {
			// The status of the first pending transaction should be returned immediately, as the transaction has already been sent.
			// This should occur only once for each subscription.
			txInfo.Status = flow.TransactionStatusPending
			return b.generateResultsWithMissingStatuses(txInfo.TransactionResult, flow.TransactionStatusUnknown)
		}

		if txInfo.IsFinal() {
			return nil, fmt.Errorf("transaction final status %s already reported: %w", txInfo.Status.String(), subscription.ErrEndOfData)
		}

		// If on this step transaction block not available, search for it.
		if txInfo.blockWithTx == nil {
			// Search for transaction`s block information.
			txInfo.blockWithTx,
				txInfo.BlockID,
				txInfo.BlockHeight,
				txInfo.CollectionID,
				err = b.searchForTransactionBlock(height, txInfo.TransactionID)

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
			txResult, err := b.searchForTransactionResult(ctx, txInfo.TransactionID, txInfo.blockWithTx, txInfo.eventEncodingVersion)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get execution result for block %s: %v", txInfo.BlockID, err)
			}

			// If transaction result was found, fully replace it in metadata. New transaction status already included in result.
			if txResult != nil {
				txInfo.TransactionResult = txResult
			}
		}

		// Check, if transaction executed and transaction result already available
		if txInfo.blockWithTx == nil {
			txInfo.Status, err = b.txLocalDataProvider.DeriveUnknownTransactionStatus(txInfo.txReferenceBlockID)
			if err != nil {
				if !errors.Is(err, state.ErrUnknownSnapshotReference) {
					irrecoverable.Throw(ctx, err)
				}
				return nil, rpc.ConvertStorageError(err)
			}
		} else {
			// When a block with the transaction is available, it is possible to receive a new transaction status while
			// searching for the transaction result. Otherwise, it remains unchanged. So, if the old and new transaction
			// statuses are the same, the current transaction status should be retrieved.
			txInfo.Status, err = b.txLocalDataProvider.DeriveTransactionStatus(txInfo.blockWithTx.Height, txInfo.IsExecuted())
			if err != nil {
				if !errors.Is(err, state.ErrUnknownSnapshotReference) {
					irrecoverable.Throw(ctx, err)
				}
				return nil, rpc.ConvertStorageError(err)
			}
		}

		// If the old and new transaction statuses are still the same, the status change should not be reported, so
		// return here with no response.
		if prevTxStatus == txInfo.Status {
			return nil, nil
		}

		return b.generateResultsWithMissingStatuses(txInfo.TransactionResult, prevTxStatus)
	}
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

// searchForTransactionBlock searches for the block containing the specified transaction.
// It retrieves the block at the given height and checks if the transaction is included in that block.
// Expected errors:
// - ErrTransactionNotInBlock when unable to retrieve the collection
// - codes.Internal when other errors occur during block or collection lookup
func (b *backendSubscribeTransactions) searchForTransactionBlock(
	height uint64,
	txID flow.Identifier,
) (*flow.Header, flow.Identifier, uint64, flow.Identifier, error) {
	block, err := b.txLocalDataProvider.blocks.ByHeight(height)
	if err != nil {
		return nil, flow.ZeroID, 0, flow.ZeroID, fmt.Errorf("error looking up block: %w", err)
	}

	collectionID, err := b.txLocalDataProvider.LookupCollectionIDInBlock(block, txID)
	if err != nil {
		return nil, flow.ZeroID, 0, flow.ZeroID, fmt.Errorf("error looking up transaction in block: %w", err)
	}

	if collectionID != flow.ZeroID {
		return block.Header, block.ID(), height, collectionID, nil
	}

	return nil, flow.ZeroID, 0, flow.ZeroID, nil
}

// searchForTransactionResult searches for the transaction result of a block. It retrieves the transaction result from
// storage and, in case of failure, attempts to fetch the transaction result directly from the execution node.
// This is necessary to ensure data availability despite sync storage latency.
//
// No errors expected during normal operations.
func (b *backendSubscribeTransactions) searchForTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockWithTx *flow.Header,
	eventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	txResult, err := b.backendTransactions.GetTransactionResultFromStorage(ctx, blockWithTx, txID, eventEncodingVersion)
	if err != nil {
		// If any error occurs with local storage - request transaction result from EN
		txResult, err = b.backendTransactions.GetTransactionResultFromExecutionNode(
			ctx,
			blockWithTx,
			txID,
			eventEncodingVersion,
		)

		if err != nil {
			// if either the execution node reported no results
			if status.Code(err) == codes.NotFound {
				// No result yet, indicate that it has not been executed
				return nil, nil
			}
			// Other Error trying to retrieve the result, return with err
			return nil, err
		}
	}

	return txResult, nil
}
