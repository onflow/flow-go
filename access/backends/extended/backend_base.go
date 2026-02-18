package extended

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
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

// normalizeLimit applies default and maximum page size constraints to the given limit.
func (b *backendBase) normalizeLimit(limit uint32) uint32 {
	if limit == 0 {
		return b.config.DefaultPageSize
	}
	if limit > b.config.MaxPageSize {
		return b.config.MaxPageSize
	}
	return limit
}

// mapReadError converts storage read errors to appropriate gRPC status errors.
func (b *backendBase) mapReadError(ctx context.Context, label string, err error) error {
	switch {
	case errors.Is(err, storage.ErrNotBootstrapped):
		return status.Errorf(codes.FailedPrecondition, "%s index not initialized: %v", label, err)
	case errors.Is(err, storage.ErrHeightNotIndexed):
		return status.Errorf(codes.OutOfRange, "requested height not indexed: %v", err)
	default:
		irrecoverable.Throw(ctx, fmt.Errorf("failed to get %s: %w", label, err))
		return err
	}
}

// lookupTransactionDetails retrieves the transaction body and result for a given transaction.
//
// Since the extended indexer only indexes sealed data, all transaction and result data should exist
// in storage for the given height.
//
// No error returns are expected during normal operation.
func (b *backendBase) lookupTransactionDetails(
	ctx context.Context,
	txID flow.Identifier,
	blockHeight uint64,
	encodingVersion entities.EventEncodingVersion,
) (*flow.TransactionBody, *accessmodel.TransactionResult, error) {
	header, err := b.headers.ByHeight(blockHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve block header: %w", err)
	}

	txBody, isSystemChunkTx, err := b.getTransactionBody(ctx, header, txID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve transaction body: %w", err)
	}

	var collectionID flow.Identifier
	collection, err := b.collections.LightByTransactionID(txID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, nil, fmt.Errorf("could not retrieve collection: %w", err)
		}
		if !isSystemChunkTx {
			return nil, nil, fmt.Errorf("could not retrieve collection: %w", err)
		}
		// for system chunk transactions, use the zero ID
		collectionID = flow.ZeroID
	} else {
		collectionID = collection.ID()
	}

	result, err := b.transactionsProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve transaction result: %w", err)
	}

	return txBody, result, nil
}

// getTransactionBody retrieves the transaction body for the given transaction ID.
// It checks submitted transactions, system transactions, and scheduled transactions in order.
//
// No error returns are expected during normal operation.
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
			// TODO: throw irrecoverable error?
			return nil, false, fmt.Errorf("transaction not found: %w", err)
		}
		return nil, false, fmt.Errorf("could not retrieve scheduled transaction block ID: %w", err)
	}

	// the provided header was looked up based on data stored in the db for the account transaction.
	// if the transaction is a scheduled transaction, it must match the block ID indexed for the
	// scheduled transaction, otherwise the node is in an inconsistent state.
	if blockID != header.ID() {
		err := fmt.Errorf("scheduled transaction found in block %s, but %s was provided", blockID, header.ID())
		irrecoverable.Throw(ctx, err)
		return nil, false, err
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
	err = fmt.Errorf("indexed transaction not found")
	irrecoverable.Throw(ctx, err)
	return nil, false, err
}
