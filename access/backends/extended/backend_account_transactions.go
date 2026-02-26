package extended

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type AccountTransactionExpandOptions struct {
	Result      bool
	Transaction bool
}

func (o *AccountTransactionExpandOptions) HasExpand() bool {
	return o.Result || o.Transaction
}

type AccountTransactionFilter struct {
	Roles []accessmodel.TransactionRole
}

func (f *AccountTransactionFilter) Filter() storage.IndexFilter[*accessmodel.AccountTransaction] {
	if len(f.Roles) == 0 {
		return nil
	}

	rolesMap := make(map[accessmodel.TransactionRole]bool, len(f.Roles))
	for _, role := range f.Roles {
		rolesMap[role] = true
	}

	return func(tx *accessmodel.AccountTransaction) bool {
		for _, role := range tx.Roles {
			if rolesMap[role] {
				return true
			}
		}
		return false
	}
}

// AccountTransactionsBackend implements the extended API for querying account transactions.
type AccountTransactionsBackend struct {
	log    zerolog.Logger
	config Config
	store  storage.AccountTransactionsReader

	headers               storage.Headers
	collections           storage.CollectionsReader
	transactions          storage.TransactionsReader
	scheduledTransactions storage.ScheduledTransactionsReader

	transactionsProvider provider.TransactionProvider
	systemCollections    *systemcollection.Versioned
}

// New creates a new AccountTransactionsBackend instance.
func NewAccountTransactionsBackend(
	log zerolog.Logger,
	config Config,
	store storage.AccountTransactionsReader,
	headers storage.Headers,
	collections storage.CollectionsReader,
	transactions storage.TransactionsReader,
	scheduledTransactions storage.ScheduledTransactionsReader,
	systemCollections *systemcollection.Versioned,
	transactionsProvider provider.TransactionProvider,
) *AccountTransactionsBackend {
	return &AccountTransactionsBackend{
		log:                   log,
		config:                config,
		store:                 store,
		headers:               headers,
		collections:           collections,
		transactions:          transactions,
		scheduledTransactions: scheduledTransactions,
		systemCollections:     systemCollections,
		transactionsProvider:  transactionsProvider,
	}
}

// GetAccountTransactions returns a paginated list of transactions for the given account address.
// Results are ordered descending by block height (newest first).
//
// If the account is found but has no transactions, the response will include an empty array and no error.
//
// Expected error returns during normal operations:
//   - [codes.FailedPrecondition] if the account transaction index has not been initialized
//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
func (b *AccountTransactionsBackend) GetAccountTransactions(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.AccountTransactionCursor,
	filter AccountTransactionFilter,
	expandOptions AccountTransactionExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.AccountTransactionsPage, error) {
	if limit == 0 {
		limit = b.config.DefaultPageSize
	}
	if limit > b.config.MaxPageSize {
		limit = b.config.MaxPageSize
	}

	page, err := b.store.TransactionsByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNotBootstrapped):
			return nil, status.Errorf(codes.FailedPrecondition, "account transaction index not initialized: %v", err)
		case errors.Is(err, storage.ErrHeightNotIndexed):
			return nil, status.Errorf(codes.OutOfRange, "requested height not indexed: %v", err)
		default:
			irrecoverable.Throw(ctx, fmt.Errorf("failed to get account transactions: %w", err))
			return nil, err
		}
	}

	// enrich the transactions with additional details requested by the client
	// Note: if no transactions are found, the response will include an empty array and no error.
	for i := range page.Transactions {
		err := b.enrichTransaction(ctx, &page.Transactions[i], expandOptions, encodingVersion)
		if err != nil {
			err = fmt.Errorf("failed to populate details for transaction %s: %w", page.Transactions[i].TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
	}

	return &page, nil
}

// enrichTransaction adds additional details to the transaction.
//
// Since the extended indexer only indexes sealed data, all transaction and result data should exist
// in storage for the given height.
//
// No error returns are expected during normal operation.
func (b *AccountTransactionsBackend) enrichTransaction(
	ctx context.Context,
	tx *accessmodel.AccountTransaction,
	expandOptions AccountTransactionExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) error {
	blockID, err := b.headers.BlockIDByHeight(tx.BlockHeight)
	if err != nil {
		return fmt.Errorf("could not retrieve block ID: %w", err)
	}

	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve block header: %w", err)
	}

	// always add the block timestamp
	tx.BlockTimestamp = header.Timestamp

	// only add the transaction body and result if requested
	if !expandOptions.HasExpand() {
		return nil
	}

	var isSystemChunkTx bool
	if expandOptions.Transaction {
		var txBody *flow.TransactionBody
		txBody, isSystemChunkTx, err = b.getTransactionBody(ctx, header, tx.TransactionID)
		if err != nil {
			return fmt.Errorf("could not retrieve transaction body: %w", err)
		}
		tx.Transaction = txBody
	}

	if expandOptions.Result {
		result, err := b.getTransactionResult(ctx, tx.TransactionID, header, isSystemChunkTx, expandOptions.Transaction, encodingVersion)
		if err != nil {
			return fmt.Errorf("could not retrieve transaction result: %w", err)
		}
		tx.Result = result
	}

	return nil
}

// getTransactionBody retrieves the transaction body for the given txID by searching in order:
// submitted transactions, system transactions, and finally scheduled transactions.
// The second return value indicates whether the transaction is a system transaction
// (system chunk or scheduled execution).
//
// If the transaction is a scheduled transaction, the block ID stored for it must match the
// provided header. A mismatch indicates an inconsistency in the node's storage, which is
// treated as an irrecoverable exception.
//
// Similarly, if the transaction was indexed for an account but cannot be found in any storage
// location, the node's state is inconsistent, which is also treated as an irrecoverable
// exception.
//
// No error returns are expected during normal operation.
func (b *AccountTransactionsBackend) getTransactionBody(ctx context.Context, header *flow.Header, txID flow.Identifier) (*flow.TransactionBody, bool, error) {
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

// getTransactionResult retrieves the transaction result for a given transaction.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if the transaction is not found
func (b *AccountTransactionsBackend) getTransactionResult(
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
