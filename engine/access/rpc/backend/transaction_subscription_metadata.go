package backend

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// transactionSubscriptionMetadata represents metadata for managing the state of a transaction subscription.
type transactionSubscriptionMetadata struct {
	*access.TransactionResult
	txReferenceBlockID   flow.Identifier
	blockWithTx          *flow.Header
	eventEncodingVersion entities.EventEncodingVersion

	backendTransactions *backendTransactions
}

// newTransactionSubscriptionMetadata initializes a metadata object with the current state of a transaction.
//
// This metadata object is used for monitoring the transaction's progress and maintaining its state.
//
// Parameters:
//   - ctx: Context to manage the operation lifecycle.
//   - txID: The unique identifier of the transaction.
//   - referenceBlockID: The ID of the transaction's reference block.
//   - backendTransactions: The reference to the backend transactions module.
//   - startHeight: The height of the block to start monitoring from.
//   - requiredEventEncodingVersion: The required version of event encoding.
//
// Returns:
//   - *transactionSubscriptionMetadata: Metadata containing the transaction's current state.
//   - error: An error, if the metadata could not be initialized.
func newTransactionSubscriptionMetadata(
	ctx context.Context,
	backendTransactions *backendTransactions,
	txID flow.Identifier,
	txReferenceBlockID flow.Identifier,
	startHeight uint64,
	eventEncodingVersion entities.EventEncodingVersion,
) (*transactionSubscriptionMetadata, error) {
	txMetadata := &transactionSubscriptionMetadata{
		backendTransactions:  backendTransactions,
		TransactionResult:    &access.TransactionResult{TransactionID: txID},
		txReferenceBlockID:   txReferenceBlockID,
		eventEncodingVersion: eventEncodingVersion,
	}

	if err := txMetadata.initTransactionReferenceBlockID(); err != nil {
		return nil, err
	}

	if err := txMetadata.initBlockInfoFromBlocksRange(startHeight); err != nil {
		return nil, err
	}

	if err := txMetadata.initTransactionResult(ctx); err != nil {
		return nil, err
	}

	return txMetadata, nil
}

// initTransactionReferenceBlockID initializes the reference block ID for the transaction.
// If the reference block ID is not set, it fetches the transaction details and updates the reference block ID.
//
// Returns:
//   - error: An error, if the reference block ID cannot be determined.
func (tm *transactionSubscriptionMetadata) initTransactionReferenceBlockID() error {
	// Get referenceBlockID if it is not set
	if tm.txReferenceBlockID == flow.ZeroID {
		tx, err := tm.backendTransactions.transactions.ByID(tm.TransactionID)
		if err != nil {
			return err
		}
		tm.txReferenceBlockID = tx.ReferenceBlockID
	}

	return nil
}

// initBlockInfoFromBlocksRange initializes block information for the transaction by searching through a range of blocks.
//
// This function iterates over blocks from the transaction's reference block height to a specified end height,
// searching for the block containing the transaction.
//
// Parameters:
//   - end: The ending block height to stop the search.
//
// Returns:
//   - error: An error, if the block information cannot be initialized.
func (tm *transactionSubscriptionMetadata) initBlockInfoFromBlocksRange(end uint64) error {
	refBlock, err := tm.backendTransactions.blocks.ByID(tm.txReferenceBlockID)
	if err != nil {
		return err
	}

	for height := refBlock.Header.Height; height <= end; height++ {
		if err := tm.checkAndFillTransactionBlockData(height); err != nil {
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, ErrTransactionNotInBlock) {
				continue
			}
			return err
		}
	}

	return nil
}

// initTransactionResult initializes the transaction's result.
//
// This function attempts to retrieve the transaction's result from storage or derive new status if the result is unavailable.
//
// Parameters:
//   - ctx: Context to manage the operation lifecycle.
//
// Returns:
//   - error: An error, if the transaction result cannot be initialized.
func (tm *transactionSubscriptionMetadata) initTransactionResult(ctx context.Context) error {
	if err := tm.checkAndFillCompleteTransactionResult(ctx); err != nil {
		return err
	}

	// It is possible to receive a new transaction status while searching for the transaction result or do not find the
	// transaction result at all, that is why transaction status must be filled after searching the transaction result
	if err := tm.fillTransactionStatus(ctx); err != nil {
		return err
	}
	return nil
}

// fillTransactionStatus updates the transaction's status based on its execution result.
//
// This function checks the transaction's execution status and updates its status accordingly. It handles
// cases where the block containing the transaction is available or unavailable.
//
// Parameters:
//   - ctx: Context to manage the operation lifecycle.
//
// Returns:
//   - error: An error, if the transaction status cannot be derived.
func (tm *transactionSubscriptionMetadata) fillTransactionStatus(ctx context.Context) error {
	var err error

	// Check, if transaction executed and transaction result already available
	if tm.blockWithTx == nil {
		tm.Status, err = tm.backendTransactions.DeriveUnknownTransactionStatus(tm.txReferenceBlockID)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return rpc.ConvertStorageError(err)
		}
	} else {
		// When a block with the transaction is available, it is possible to receive a new transaction status while
		// searching for the transaction result. Otherwise, it remains unchanged. So, if the old and new transaction
		// statuses are the same, the current transaction status should be retrieved.
		tm.Status, err = tm.backendTransactions.DeriveTransactionStatus(tm.blockWithTx.Height, tm.IsExecuted())
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return rpc.ConvertStorageError(err)
		}
	}

	return nil
}

// checkAndFillTransactionBlockData searches for the block containing the specified transaction and fill transaction block data.
//
// This function retrieves the block at the given height and checks if the transaction is included
// in that block. If the transaction is found, it updates the block metadata in the subscription.
//
// Parameters:
//   - height: The block height to search for the transaction.
//
// Expected errors:
// - ErrTransactionNotInBlock when unable to retrieve the collection
// - codes.Internal when other errors occur during block or collection lookup
func (tm *transactionSubscriptionMetadata) checkAndFillTransactionBlockData(height uint64) error {
	if tm.blockWithTx != nil {
		return nil
	}

	block, err := tm.backendTransactions.blocks.ByHeight(height)
	if err != nil {
		return err
	}

	collectionID, err := tm.backendTransactions.LookupCollectionIDInBlock(block, tm.TransactionID)
	if err != nil {
		return err
	}

	if collectionID != flow.ZeroID {
		tm.blockWithTx = block.Header
		tm.BlockID = block.ID()
		tm.BlockHeight = height
		tm.CollectionID = collectionID
	}

	return nil
}

// checkAndFillCompleteTransactionResult retrieves the transaction result from storage or an execution node.
//
// This function first tries to fetch the transaction result from local storage. If unavailable,
// it attempts to fetch the result from the execution node to ensure availability.
//
// Parameters:
//   - ctx: Context to manage the operation lifecycle.
//
// Returns:
//   - *access.TransactionResult: The transaction result, if found.
//
// No errors expected during normal operations.
func (tm *transactionSubscriptionMetadata) checkAndFillCompleteTransactionResult(ctx context.Context) error {
	// If there is no transaction block found, it is impossible to search for transaction result
	if tm.blockWithTx == nil || tm.IsExecuted() {
		return nil
	}

	// Trying to get transaction result from local storage
	txResult, err := tm.backendTransactions.GetTransactionResultFromStorage(ctx, tm.blockWithTx, tm.TransactionID, tm.eventEncodingVersion)
	if err != nil {
		// If any error occurs with local storage - request transaction result from EN
		txResult, err = tm.backendTransactions.GetTransactionResultFromExecutionNode(ctx, tm.blockWithTx, tm.TransactionID, tm.eventEncodingVersion)

		if err != nil {
			// if either the execution node reported no results
			if status.Code(err) == codes.NotFound {
				// No result yet, indicate that it has not been executed
				return nil
			}

			return err
		}
	}

	// If transaction result was found, fully replace it in metadata. New transaction status already included in result.
	if txResult != nil {
		tm.TransactionResult = txResult
	}

	return nil
}
