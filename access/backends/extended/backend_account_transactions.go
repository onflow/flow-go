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

type TransactionFilter storage.IndexFilter[*accessmodel.AccountTransaction]

func HasRoles(roles ...accessmodel.TransactionRole) TransactionFilter {
	searchRoles := make(map[accessmodel.TransactionRole]struct{}, len(roles))
	for _, role := range roles {
		searchRoles[role] = struct{}{}
	}
	return func(tx *accessmodel.AccountTransaction) bool {
		for _, role := range tx.Roles {
			if _, ok := searchRoles[role]; ok {
				return true
			}
		}
		return false
	}
}

type AccountTransactionFilter struct {
	Roles []accessmodel.TransactionRole
}

func (f *AccountTransactionFilter) Filter() storage.IndexFilter[*accessmodel.AccountTransaction] {
	return func(tx *accessmodel.AccountTransaction) bool {
		if len(f.Roles) > 0 {
			return HasRoles(f.Roles...)(tx)
		}
		return true
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
	chainID flow.ChainID,
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
//   - [codes.Internal] if there is an unexpected error
func (b *AccountTransactionsBackend) GetAccountTransactions(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.AccountTransactionCursor,
	filter AccountTransactionFilter,
	expandResults bool,
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

	if !expandResults {
		return &page, nil
	}

	// enrich the transactions with additional details requested by the client
	// Note: if no transactions are found, the response will include an empty array and no error.
	for i := range page.Transactions {
		err := b.enrichTransaction(ctx, &page.Transactions[i], encodingVersion)
		if err != nil {
			// all errors are internal since data should exist in storage
			return nil, status.Errorf(codes.Internal, "failed populate details for transaction %s: %v", page.Transactions[i].TransactionID, err)
		}
	}

	return &page, nil
}

// enrichTransaction adds additional details to the transaction to the transaction.
//
// Since the extended indexer only indexes sealed data, all transaction and result data should exist
// in storage for the given height.
//
// No error returns are expected during normal operation.
func (b *AccountTransactionsBackend) enrichTransaction(
	ctx context.Context,
	tx *accessmodel.AccountTransaction,
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

	txBody, isSystemChunkTx, err := b.getTransactionBody(ctx, header, tx.TransactionID)
	if err != nil {
		return fmt.Errorf("could not retrieve transaction body: %w", err)
	}

	var collectionID flow.Identifier
	collection, err := b.collections.LightByTransactionID(tx.TransactionID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not retrieve collection: %w", err)
		}
		if !isSystemChunkTx {
			return fmt.Errorf("could not retrieve collection: %w", err)
		}
		// for system chunk transactions, use the zero ID
		collectionID = flow.ZeroID
	} else {
		collectionID = collection.ID()
	}

	result, err := b.transactionsProvider.TransactionResult(ctx, header, tx.TransactionID, collectionID, encodingVersion)
	if err != nil {
		return fmt.Errorf("could not retrieve transaction result: %w", err)
	}

	tx.Transaction = txBody
	tx.Result = result

	return nil
}

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
