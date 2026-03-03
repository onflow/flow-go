package extended

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// backendBase holds shared configuration, storage dependencies, and helper methods used by both
// the account transactions and account transfers backends.
type backendBase struct {
	config Config

	headers               storage.Headers
	collections           storage.CollectionsReader
	transactions          storage.TransactionsReader
	scheduledTransactions storage.ScheduledTransactionsReader

	transactionsProvider provider.TransactionProvider
	systemCollections    *systemcollection.Versioned
}

// normalizeLimit applies default page size when limit is 0, and returns an error if the limit
// exceeds the configured maximum.
//
// Any error indicates the limit is invalid.
func (b *backendBase) normalizeLimit(limit uint32) (uint32, error) {
	if limit == 0 {
		return b.config.DefaultPageSize, nil
	}
	if limit > b.config.MaxPageSize {
		return 0, fmt.Errorf("limit exceeds maximum: %d > %d", limit, b.config.MaxPageSize)
	}
	return limit, nil
}

// getTransactionResult retrieves the transaction result for a given transaction.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if the transaction is not found
func (b *backendBase) getTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	header *flow.Header,
	isSystemChunkTx bool,
	expandTransaction bool,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	// the system collection is not indexed and uses the zero ID by convention.
	var collectionID flow.Identifier

	if !isSystemChunkTx {
		collection, err := b.collections.LightByTransactionID(txID)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return nil, fmt.Errorf("could not retrieve collection: %w", err)
			}
			// if we have already looked up the transaction and confirmed it is NOT a system chunk tx,
			// then there should be an entry in the tx/collection index. however, the collection/tx
			// index is built asynchronously with the extended indexer and may not be available yet.
			// return an error, but don't throw an irrecoverable error.
			if expandTransaction {
				return nil, fmt.Errorf("could not retrieve collection for standard transaction: %w", err)
			}
			// if the collection is not found and we're not expanding the transaction,
			// proceed with zero collectionID.
		} else {
			collectionID = collection.ID()
		}
	}

	result, err := b.transactionsProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve transaction result: %w", err)
	}

	return result, nil
}

// getTransactionBody retrieves the transaction body for the given transaction ID.
// It checks submitted transactions, system transactions, and scheduled transactions in order.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if the transaction is not found
func (b *backendBase) getTransactionBody(ctx context.Context, header *flow.Header, txID flow.Identifier) (*flow.TransactionBody, bool, error) {
	// first, check if it's a submitted transaction since that's the most common
	txBody, err := b.transactions.ByID(txID)
	if err == nil {
		return txBody, false, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, false, fmt.Errorf("failed to retrieve transaction body: %w", err)
	}

	// next, check if the transaction is a system transaction because it's the cheapest lookup
	systemTx, ok := b.systemCollections.SearchAll(txID)
	if ok {
		return systemTx, true, nil
	}

	// finally, check if it's a scheduled transaction
	blockID, err := b.scheduledTransactions.BlockIDByTransactionID(txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, false, fmt.Errorf("transaction not found: %w", err)
		}
		return nil, false, fmt.Errorf("could not retrieve scheduled transaction block ID: %w", err)
	}

	// the provided header was looked up based on data stored in the db for the account transaction.
	// if the transaction is a scheduled transaction, it must match the block ID indexed for the
	// scheduled transaction, otherwise the node is in an inconsistent state.
	if blockID != header.ID() {
		return nil, false, fmt.Errorf("scheduled transaction found in block %s, but %s was provided", blockID, header.ID())
	}

	allScheduledTxs, err := b.transactionsProvider.ScheduledTransactionsByBlockID(ctx, header)
	if err != nil {
		return nil, false, fmt.Errorf("could not retrieve all scheduled transactions: %w", err)
	}

	for _, scheduledTx := range allScheduledTxs {
		if scheduledTx.ID() == txID {
			return scheduledTx, true, nil
		}
	}

	// at this point, the transaction is not known to the node.
	// this is unexpected. if the account transaction was indexed, then the transaction should be found
	// somewhere in storage.
	return nil, false, fmt.Errorf("indexed transaction not found: %w", storage.ErrNotFound)
}
