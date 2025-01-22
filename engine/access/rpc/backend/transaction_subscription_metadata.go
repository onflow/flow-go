package backend

import (
	"context"
	"errors"
	"fmt"

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

// transactionSubscriptionMetadata holds data representing the status state for each transaction subscription.
type transactionSubscriptionMetadata struct {
	*access.TransactionResult
	txReferenceBlockID   flow.Identifier
	blockWithTx          *flow.Header
	eventEncodingVersion entities.EventEncodingVersion

	backendTransactions *backendTransactions
}

func newTransactionSubscriptionMetadata(
	backendTransactions *backendTransactions,
	txID flow.Identifier,
	txReferenceBlockID flow.Identifier,
	eventEncodingVersion entities.EventEncodingVersion,
) *transactionSubscriptionMetadata {
	return &transactionSubscriptionMetadata{
		backendTransactions:  backendTransactions,
		TransactionResult:    &access.TransactionResult{TransactionID: txID},
		txReferenceBlockID:   txReferenceBlockID,
		eventEncodingVersion: eventEncodingVersion,
	}
}

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

func (tm *transactionSubscriptionMetadata) deriveTransactionResult(ctx context.Context) error {
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

func (tm *transactionSubscriptionMetadata) initBlockInfoFromBlocksRange(end uint64) error {
	refBlock, err := tm.backendTransactions.blocks.ByID(tm.txReferenceBlockID)
	if err != nil {
		return err
	}

	for height := refBlock.Header.Height; height <= end; height++ {
		if err := tm.searchForTransactionInBlock(height); err != nil {
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, ErrTransactionNotInBlock) {
				continue
			}
			return err
		}
	}

	return nil
}

// searchForTransactionInBlock searches for the block containing the specified transaction.
// It retrieves the block at the given height and checks if the transaction is included in that block.
// Expected errors:
// - ErrTransactionNotInBlock when unable to retrieve the collection
// - codes.Internal when other errors occur during block or collection lookup
func (tm *transactionSubscriptionMetadata) searchForTransactionInBlock(height uint64) error {
	block, err := tm.backendTransactions.blocks.ByHeight(height)
	if err != nil {
		return fmt.Errorf("error looking up block: %w", err)
	}

	collectionID, err := tm.backendTransactions.LookupCollectionIDInBlock(block, tm.TransactionID)
	if err != nil {
		return fmt.Errorf("error looking up transaction in block: %w", err)
	}

	if collectionID != flow.ZeroID {
		tm.blockWithTx = block.Header
		tm.BlockID = block.ID()
		tm.BlockHeight = height
		tm.CollectionID = collectionID
	}

	return nil
}

func (tm *transactionSubscriptionMetadata) initTransactionResult(ctx context.Context) error {
	txResult, err := tm.searchForTransactionResult(ctx)
	if err != nil {
		return err
	}

	if txResult == nil {
		if err = tm.deriveTransactionResult(ctx); err != nil {
			return err
		}
	} else {
		tm.TransactionResult = txResult
	}
	return nil
}

// searchForTransactionResult searches for the transaction result of a block. It retrieves the transaction result from
// storage and, in case of failure, attempts to fetch the transaction result directly from the execution node.
// This is necessary to ensure data availability despite sync storage latency.
//
// No errors expected during normal operations.
func (tm *transactionSubscriptionMetadata) searchForTransactionResult(ctx context.Context) (*access.TransactionResult, error) {
	// If there is no transaction block found, it is impossible to search for transaction result
	if tm.blockWithTx == nil {
		return nil, nil
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
				return nil, nil
			}
			// Other Error trying to retrieve the result, return with err
			return nil, err
		}
	}

	return txResult, nil
}
