package stream

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	txprovider "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// TransactionMetadata manages the state of a transaction subscription.
//
// This struct contains metadata for tracking a transaction's progress, including
// references to relevant blocks, collections, and transaction results.
type TransactionMetadata struct {
	blocks       storage.Blocks
	collections  storage.Collections
	transactions storage.Transactions

	txResult           *accessmodel.TransactionResult
	txReferenceBlockID flow.Identifier
	blockWithTx        *flow.Header //TODO: what is this???

	eventEncodingVersion entities.EventEncodingVersion

	txProvider         *txprovider.FailoverTransactionProvider
	txStatusDeriver    *txstatus.TxStatusDeriver
	execResultProvider optimistic_sync.ExecutionResultProvider

	execResultInfo *optimistic_sync.ExecutionResultInfo
	criteria       optimistic_sync.Criteria
}

// NewTransactionMetadata initializes a new metadata object for a transaction subscription.
func NewTransactionMetadata(
	blocks storage.Blocks,
	collections storage.Collections,
	transactions storage.Transactions,
	txID flow.Identifier,
	txReferenceBlockID flow.Identifier,
	eventEncodingVersion entities.EventEncodingVersion,
	txProvider *txprovider.FailoverTransactionProvider,
	txStatusDeriver *txstatus.TxStatusDeriver,
	execResultProvider optimistic_sync.ExecutionResultProvider,
) *TransactionMetadata {
	return &TransactionMetadata{
		txResult:             &accessmodel.TransactionResult{TransactionID: txID},
		eventEncodingVersion: eventEncodingVersion,
		blocks:               blocks,
		collections:          collections,
		transactions:         transactions,
		txReferenceBlockID:   txReferenceBlockID,
		txProvider:           txProvider,
		txStatusDeriver:      txStatusDeriver,
		execResultProvider:   execResultProvider,
	}
}

// Refresh updates the transaction subscription metadata to reflect the latest state.
//
// Expected error returns during normal operation:
//   - [ErrBlockNotReady] - if the block at the given height is not found.
//   - codes.Internal - if impossible to get transaction result due to event payload conversion failed
func (t *TransactionMetadata) Refresh(ctx context.Context) error {
	if err := t.refreshCollection(); err != nil {
		return err
	}
	if err := t.refreshBlock(); err != nil {
		return err
	}
	if err := t.refreshTransactionResult(ctx); err != nil {
		return err
	}
	if err := t.refreshStatus(ctx); err != nil {
		return err
	}
	return nil
}

// refreshTransactionReferenceBlockID sets the reference block ID for the transaction.
// If the reference block ID is unset, it attempts to retrieve it from storage.
//
// No errors expected during normal operations.
func (t *TransactionMetadata) refreshTransactionReferenceBlockID() error {
	// Get referenceBlockID if it is not set
	if t.txReferenceBlockID != flow.ZeroID {
		return nil
	}

	tx, err := t.transactions.ByID(t.txResult.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to lookup transaction by transaction ID: %w", err)
	}
	t.txReferenceBlockID = tx.ReferenceBlockID
	return nil
}

// refreshStatus updates the transaction's status based on its execution result.
//
// No errors expected during normal operations.
func (t *TransactionMetadata) refreshStatus(ctx context.Context) error {
	var err error

	if t.blockWithTx == nil {
		if err = t.refreshTransactionReferenceBlockID(); err != nil {
			// transaction was not sent from this node, and it has not been indexed yet.
			if errors.Is(err, storage.ErrNotFound) {
				t.txResult.Status = flow.TransactionStatusUnknown
				return nil
			}
			return err
		}

		t.txResult.Status, err = t.txStatusDeriver.DeriveUnknownTransactionStatus(t.txReferenceBlockID)
		if err != nil {
			irrecoverable.Throw(ctx, fmt.Errorf("failed to derive unknown transaction status: %w", err))
			return err
		}
		return nil
	}

	// When the transaction is included in an executed block, the `txResult` may be updated during `Refresh`
	// Recheck the status to ensure it's accurate.
	t.txResult.Status, err = t.txStatusDeriver.DeriveFinalizedTransactionStatus(t.blockWithTx.Height, t.txResult.IsExecuted())
	if err != nil {
		irrecoverable.Throw(ctx, fmt.Errorf("failed to derive finalized transaction status: %w", err))
		return err
	}
	return nil
}

// refreshBlock updates the block metadata if the transaction has been included in a block.
//
// Expected error returns during normal operation:
//   - [ErrBlockNotReady] - if the block for collection ID is not found.
func (t *TransactionMetadata) refreshBlock() error {
	if t.txResult.CollectionID == flow.ZeroID || t.blockWithTx != nil {
		return nil
	}

	block, err := t.blocks.ByCollectionID(t.txResult.CollectionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return subscription.ErrBlockNotReady
		}

		return fmt.Errorf("failed to lookup block containing collection: %w", err)
	}

	t.blockWithTx = block.ToHeader()
	t.txResult.BlockID = block.ID()
	t.txResult.BlockHeight = block.Height

	execResultInfo, err := t.execResultProvider.ExecutionResult(t.txResult.BlockID, t.criteria)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil
		}
		return fmt.Errorf("failed to get execution result for block %s: %w", t.txResult.BlockID, err)
	}
	t.execResultInfo = execResultInfo

	return nil
}

// refreshCollection updates the collection metadata if the transaction is included in a block.
//
// Expected error returns during normal operation:
//   - [ErrTransactionNotInBlock] - if the transaction is not found in the block.
func (t *TransactionMetadata) refreshCollection() error {
	if t.txResult.CollectionID != flow.ZeroID {
		return nil
	}

	collection, err := t.collections.LightByTransactionID(t.txResult.TransactionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("failed to lookup collection containing tx: %w", err)
	}
	t.txResult.CollectionID = collection.ID()
	return nil
}

// refreshTransactionResult attempts to retrieve the transaction result from storage or an execution node.
//
// Expected error returns during normal operation:
//   - [codes.NotFound] - if the transaction result is unavailable.
func (t *TransactionMetadata) refreshTransactionResult(ctx context.Context) error {
	// skip check if we already have the result, or if we don't know which block it is in yet
	if t.blockWithTx == nil || t.txResult.IsExecuted() {
		return nil
	}

	// TODO: need a way to check if the result in execResultInfo is still valid.
	// if it is ever abandoned, we need to trigger the streaming fork recovery process.

	txResult, _, err := t.txProvider.TransactionResult(ctx, t.blockWithTx, t.txResult.TransactionID, t.eventEncodingVersion, t.execResultInfo)
	if err != nil {
		// TODO: I don't like the fact we propagate this error from txProvider.
		// Fix it during error handling polishing project
		if status.Code(err) == codes.NotFound {
			// No result yet, indicate that it has not been executed
			return nil
		}

		return fmt.Errorf("unexpected error while getting transaction result: %w", err)
	}

	// If transaction result was found, fully replace it in metadata. New transaction status already included in result.
	if txResult != nil {
		// Preserve the CollectionID to ensure it is not lost during the transaction result assignment.
		txResult.CollectionID = t.txResult.CollectionID
		t.txResult = txResult
	}

	return nil
}
