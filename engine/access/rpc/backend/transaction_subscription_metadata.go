package backend

import (
	"context"
	"errors"

	"github.com/onflow/flow-go/engine/access/subscription"

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

// transactionSubscriptionMetadata manages the state of a transaction subscription.
//
// This struct contains metadata for tracking a transaction's progress, including
// references to relevant blocks, collections, and transaction results.
type transactionSubscriptionMetadata struct {
	blocks               storage.Blocks
	collections          storage.Collections
	txResult             *access.TransactionResult
	txReferenceBlockID   flow.Identifier
	blockWithTx          *flow.Header
	eventEncodingVersion entities.EventEncodingVersion

	backendTransactions *backendTransactions
}

// newTransactionSubscriptionMetadata initializes a new metadata object for a transaction subscription.
//
// This function constructs a transaction metadata object used for tracking the transaction's progress
// and maintaining its state throughout execution.
//
// Parameters:
//   - ctx: Context for managing the lifecycle of the operation.
//   - backendTransactions: A reference to the backend transaction manager.
//   - txID: The unique identifier of the transaction.
//   - txReferenceBlockID: The ID of the transaction’s reference block.
//   - eventEncodingVersion: The required version of event encoding.
//
// Returns:
//   - *transactionSubscriptionMetadata: The initialized transaction metadata object.
//
// No errors expected during normal operations.
func newTransactionSubscriptionMetadata(
	ctx context.Context,
	backendTransactions *backendTransactions,
	txID flow.Identifier,
	txReferenceBlockID flow.Identifier,
	eventEncodingVersion entities.EventEncodingVersion,
) (*transactionSubscriptionMetadata, error) {
	txMetadata := &transactionSubscriptionMetadata{
		backendTransactions:  backendTransactions,
		txResult:             &access.TransactionResult{TransactionID: txID},
		eventEncodingVersion: eventEncodingVersion,
		blocks:               backendTransactions.blocks,
		collections:          backendTransactions.collections,
	}

	if err := txMetadata.initTransactionReferenceBlockID(txReferenceBlockID); err != nil {
		return nil, err
	}

	if err := txMetadata.initBlockInfo(); err != nil {
		return nil, err
	}

	if err := txMetadata.initTransactionResult(ctx); err != nil {
		return nil, err
	}

	return txMetadata, nil
}

// initTransactionReferenceBlockID sets the reference block ID for the transaction.
//
// If the reference block ID is unset, it attempts to retrieve it from storage.
//
// Parameters:
//   - txReferenceBlockID: The reference block ID of the transaction.
//
// No errors expected during normal operations.
func (tm *transactionSubscriptionMetadata) initTransactionReferenceBlockID(txReferenceBlockID flow.Identifier) error {
	// Get referenceBlockID if it is not set
	if txReferenceBlockID == flow.ZeroID {
		tx, err := tm.backendTransactions.transactions.ByID(tm.txResult.TransactionID)
		if err != nil {
			return err
		}
		txReferenceBlockID = tx.ReferenceBlockID
	}

	tm.txReferenceBlockID = txReferenceBlockID

	return nil
}

// initBlockInfo determines the block that contains the transaction and updates metadata accordingly.
//
// This function searches the transaction’s collection and its corresponding block, updating
// relevant fields in the metadata object.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if the collection or block cannot be retrieved.
func (tm *transactionSubscriptionMetadata) initBlockInfo() error {
	collection, err := tm.collections.LightByTransactionID(tm.txResult.TransactionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}

		return err
	}

	tm.txResult.CollectionID = collection.ID()

	block, err := tm.blocks.ByCollectionID(tm.txResult.CollectionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}

		return err
	}

	tm.blockWithTx = block.Header
	tm.txResult.BlockID = block.ID()
	tm.txResult.BlockHeight = block.Header.Height

	return nil
}

// initTransactionResult initializes the transaction result.
//
// Parameters:
//   - ctx: Context for managing the operation lifecycle.
//
// No errors expected during normal operations.
func (tm *transactionSubscriptionMetadata) initTransactionResult(ctx context.Context) error {
	if err := tm.refreshTransactionResult(ctx); err != nil {
		return err
	}

	// It is possible to receive a new transaction status while searching for the transaction result or do not find the
	// transaction result at all, that is why transaction status must be filled after searching the transaction result
	if err := tm.refreshStatus(ctx); err != nil {
		return err
	}
	return nil
}

// Refresh updates the transaction subscription metadata to reflect the latest state.
//
// Parameters:
//   - ctx: Context for managing the operation lifecycle.
//   - height: The block height used for searching transaction data.
//
// Expected errors during normal operation:
//   - `ErrBlockNotReady` if the block at the given height is not found.
func (tm *transactionSubscriptionMetadata) Refresh(ctx context.Context, height uint64) error {
	if err := tm.refreshCollection(height); err != nil {
		return err
	}

	if err := tm.refreshBlock(); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return subscription.ErrBlockNotReady
		}
	}

	if err := tm.refreshTransactionResult(ctx); err != nil {
		return err
	}

	if err := tm.refreshStatus(ctx); err != nil {
		return err
	}

	return nil
}

// refreshStatus updates the transaction's status based on its execution result.
//
// Parameters:
//   - ctx: Context for managing the operation lifecycle.
//
// No errors expected during normal operations.
func (tm *transactionSubscriptionMetadata) refreshStatus(ctx context.Context) error {
	var err error

	// Check, if transaction executed and transaction result already available
	if tm.blockWithTx == nil {
		tm.txResult.Status, err = tm.backendTransactions.DeriveUnknownTransactionStatus(tm.txReferenceBlockID)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return rpc.ConvertStorageError(err)
		}
		return nil
	}

	// When a block with the transaction is available, it is possible to receive a new transaction status while
	// searching for the transaction result. Otherwise, it remains unchanged. So, if the old and new transaction
	// statuses are the same, the current transaction status should be retrieved.
	tm.txResult.Status, err = tm.backendTransactions.DeriveTransactionStatus(tm.blockWithTx.Height, tm.txResult.IsExecuted())
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return rpc.ConvertStorageError(err)
	}
	return nil
}

// refreshBlock updates the block metadata if the transaction has been included in a block.
//
// No errors expected during normal operations.
func (tm *transactionSubscriptionMetadata) refreshBlock() error {
	if tm.txResult.CollectionID == flow.ZeroID || tm.blockWithTx != nil {
		return nil
	}

	block, err := tm.blocks.ByCollectionID(tm.txResult.CollectionID)
	if err != nil {
		return err
	}

	tm.blockWithTx = block.Header
	tm.txResult.BlockID = block.ID()
	tm.txResult.BlockHeight = block.Header.Height

	return nil
}

// refreshCollection updates the collection metadata if the transaction is included in a block.
//
// Parameters:
//   - height: The block height at which the transaction is expected.
//
// Expected errors during normal operation:
//   - `ErrTransactionNotInBlock` if the transaction is not found in the block.
func (tm *transactionSubscriptionMetadata) refreshCollection(height uint64) error {
	if tm.txResult.CollectionID != flow.ZeroID {
		return nil
	}

	block, err := tm.blocks.ByHeight(height)
	if err != nil {
		return err
	}

	collectionID, err := tm.backendTransactions.LookupCollectionIDInBlock(block, tm.txResult.TransactionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return subscription.ErrBlockNotReady
		}

		if !errors.Is(err, ErrTransactionNotInBlock) {
			return err
		}
	}

	if collectionID != flow.ZeroID {
		tm.txResult.CollectionID = collectionID
	}

	return nil
}

// refreshTransactionResult attempts to retrieve the transaction result from storage or an execution node.
//
// Parameters:
//   - ctx: Context for managing the operation lifecycle.
//
// Expected errors during normal operation:
//   - `codes.NotFound` if the transaction result is unavailable.
func (tm *transactionSubscriptionMetadata) refreshTransactionResult(ctx context.Context) error {
	// skip check if we already have the result, or if we don't know which block it is in yet
	if tm.blockWithTx == nil || tm.txResult.IsExecuted() {
		return nil
	}

	// Trying to get transaction result from local storage
	txResult, err := tm.backendTransactions.GetTransactionResultFromStorage(ctx, tm.blockWithTx, tm.txResult.TransactionID, tm.eventEncodingVersion)
	if err != nil {
		// If any error occurs with local storage - request transaction result from EN
		txResult, err = tm.backendTransactions.GetTransactionResultFromExecutionNode(ctx, tm.blockWithTx, tm.txResult.TransactionID, tm.eventEncodingVersion)

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
		tm.txResult = txResult
	}

	return nil
}
