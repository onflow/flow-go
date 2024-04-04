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

// backendSubscribeTransactions handles transaction subscriptions.
type backendSubscribeTransactions struct {
	txLocalDataProvider *TransactionsLocalDataProvider
	backendTransactions *backendTransactions
	executionResults    storage.ExecutionResults
	log                 zerolog.Logger

	subscriptionHandler *subscription.SubscriptionHandler
	blockTracker        subscription.BlockTracker
}

// TransactionSubscriptionMetadata holds data representing the status state for each transaction subscription.
type TransactionSubscriptionMetadata struct {
	*access.TransactionResult
	txReferenceBlockID flow.Identifier
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
		TransactionResult: &access.TransactionResult{
			TransactionID: tx.ID(),
			BlockID:       flow.ZeroID,
			Status:        flow.TransactionStatusUnknown,
		},
		txReferenceBlockID: tx.ReferenceBlockID,
		blockWithTx:        nil,
	}

	return b.subscriptionHandler.Subscribe(ctx, nextHeight, b.getTransactionStatusResponse(&txInfo))
}

// getTransactionStatusResponse returns a callback function that produces transaction status
// subscription responses based on new blocks.
func (b *backendSubscribeTransactions) getTransactionStatusResponse(txInfo *TransactionSubscriptionMetadata) func(context.Context, uint64) (interface{}, error) {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		err := b.validateBlockHeight(height)
		if err != nil {
			return nil, err
		}

		// If the transaction status already reported the final status, return with no data available
		if txInfo.Status == flow.TransactionStatusSealed || txInfo.Status == flow.TransactionStatusExpired {
			return nil, fmt.Errorf("transaction final status %s was already reported: %w", txInfo.Status.String(), subscription.ErrEndOfData)
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
		if txInfo.blockWithTx == nil {
			txInfo.Status, err = b.txLocalDataProvider.DeriveUnknownTransactionStatus(txInfo.txReferenceBlockID)
		} else {
			//If transaction result was not found, get transaction status to check if it different from last status
			if txInfo.Status == prevTxStatus {
				txInfo.Status, err = b.txLocalDataProvider.DeriveTransactionStatus(txInfo.BlockID, txInfo.blockWithTx.Height, txInfo.txExecuted)
			}
		}
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		// The same transaction status should not be reported, so return here with no response
		if prevTxStatus == txInfo.Status {
			return nil, nil
		}

		return b.generateResultsWithMissingStatuses(txInfo, prevTxStatus)
	}
}

// generateResultsWithMissingStatuses checks if the current result differs from the previous result by more than one step.
// If yes, it generates results for the missing transaction statuses. This is done because the subscription should send
// responses for each of the statuses in the transaction lifecycle, and the message should be sent in the order of transaction statuses.
// Possible orders of transaction statuses:
// 1. pending(1) -> finalized(2) -> executed(3) -> sealed(4)
// 2. pending(1) -> expired(5)
func (b *backendSubscribeTransactions) generateResultsWithMissingStatuses(
	txInfo *TransactionSubscriptionMetadata,
	prevTxStatus flow.TransactionStatus,
) ([]*access.TransactionResult, error) {

	// If the status is expired, which is the last status, return its result.
	if txInfo.Status == flow.TransactionStatusExpired {
		return []*access.TransactionResult{
			txInfo.TransactionResult,
		}, nil
	}

	var results []*access.TransactionResult

	// If the difference between statuses' values is more than one step, fill in the missing results.
	if (txInfo.Status - prevTxStatus) > 1 {
		for missingStatus := prevTxStatus + 1; missingStatus < txInfo.Status; missingStatus++ {
			var missingTxResult access.TransactionResult
			switch missingStatus {
			case flow.TransactionStatusPending:
				missingTxResult = access.TransactionResult{
					Status:        missingStatus,
					TransactionID: txInfo.TransactionID,
				}
			case flow.TransactionStatusFinalized:
				missingTxResult = access.TransactionResult{
					Status:        missingStatus,
					TransactionID: txInfo.TransactionID,
					BlockID:       txInfo.BlockID,
					BlockHeight:   txInfo.BlockHeight,
					CollectionID:  txInfo.CollectionID,
				}
			case flow.TransactionStatusExecuted:
				missingTxResult = *txInfo.TransactionResult
				missingTxResult.Status = missingStatus
			default:
				return nil, fmt.Errorf("unexpected missing transaction status")
			}
			results = append(results, &missingTxResult)
		}
	}

	results = append(results, txInfo.TransactionResult)
	return results, nil
}

func (b *backendSubscribeTransactions) validateBlockHeight(height uint64) error {
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
	txInfo *TransactionSubscriptionMetadata,
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
	txInfo *TransactionSubscriptionMetadata,
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
		entities.EventEncodingVersion_CCF_V0,
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
