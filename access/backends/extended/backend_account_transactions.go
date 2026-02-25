package extended

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
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
	*backendBase

	log   zerolog.Logger
	store storage.AccountTransactionsReader
	chain flow.Chain
}

// NewAccountTransactionsBackend creates a new AccountTransactionsBackend instance.
func NewAccountTransactionsBackend(
	log zerolog.Logger,
	base *backendBase,
	store storage.AccountTransactionsReader,
	chain flow.Chain,
) *AccountTransactionsBackend {
	return &AccountTransactionsBackend{
		backendBase: base,
		log:         log,
		store:       store,
		chain:       chain,
	}
}

// GetAccountTransactions returns a paginated list of transactions for the given account address.
// Results are ordered descending by block height (newest first).
//
// If the account is found but has no transactions, the response will include an empty array and no error.
//
// Expected error returns during normal operations:
//   - [codes.NotFound] if the account is not found
//   - [codes.FailedPrecondition] if the account transaction index has not been initialized
//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
//   - [codes.InvalidArgument] if the query parameters are invalid
func (b *AccountTransactionsBackend) GetAccountTransactions(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.AccountTransactionCursor,
	filter AccountTransactionFilter,
	expandOptions AccountTransactionExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.AccountTransactionsPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	if !b.chain.IsValid(address) {
		return nil, status.Errorf(codes.NotFound, "account %s is not valid on chain %s", address, b.chain.ChainID())
	}
	// TODO: check if account exists for the chain

	page, err := b.store.ByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "account transactions", err)
	}

	// storage will return an empty page and no error if the account has no transfers indexed.
	if len(page.Transactions) == 0 {
		return &page, nil
	}

	// expand the transactions with additional details requested by the client
	// Note: if no transactions are found, the response will include an empty array and no error.
	for i := range page.Transactions {
		tx := &page.Transactions[i]
		if err := b.expand(ctx, tx, expandOptions, encodingVersion); err != nil {
			return nil, fmt.Errorf("failed to expand transaction: %w", err)
		}
	}

	return &page, nil
}

// expand adds additional details to the transaction.
//
// Since the extended indexer only indexes sealed data, all transaction and result data should exist
// in storage for the given height.
//
// No error returns are expected during normal operation.
func (b *AccountTransactionsBackend) expand(
	ctx context.Context,
	tx *accessmodel.AccountTransaction,
	expandOptions AccountTransactionExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) error {
	header, err := b.headers.ByHeight(tx.BlockHeight)
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
