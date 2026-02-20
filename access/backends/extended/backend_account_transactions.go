package extended

import (
	"context"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessmodel "github.com/onflow/flow-go/model/access"
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
	*backendBase

	log   zerolog.Logger
	store storage.AccountTransactionsReader
}

// NewAccountTransactionsBackend creates a new AccountTransactionsBackend instance.
func NewAccountTransactionsBackend(
	log zerolog.Logger,
	base *backendBase,
	store storage.AccountTransactionsReader,
) *AccountTransactionsBackend {
	return &AccountTransactionsBackend{
		backendBase: base,
		log:         log,
		store:       store,
	}
}

// GetAccountTransactions returns a paginated list of transactions for the given account address.
// Results are ordered descending by block height (newest first).
//
// If the account is found but has no transactions, the response will include an empty array and no error.
//
// Expected error returns during normal operations:
//   - [codes.NotFound] if the account is found but has no transactions
//   - [codes.FailedPrecondition] if the account transaction index has not been initialized
//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
//   - [codes.InvalidArgument] if the query parameters are invalid
func (b *AccountTransactionsBackend) GetAccountTransactions(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.AccountTransactionCursor,
	filter AccountTransactionFilter,
	expandResults bool,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.AccountTransactionsPage, error) {
	limit = b.normalizeLimit(limit)

	page, err := b.store.ByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "account transactions", err)
	}

	// storage will return an empty page and no error if the account has no transfers indexed.
	if len(page.Transactions) == 0 && cursor != nil {
		return nil, status.Errorf(codes.NotFound, "no account transactions found for account %s", address)
	}

	// enrich the transactions with additional details requested by the client
	// Note: if no transactions are found, the response will include an empty array and no error.
	for i := range page.Transactions {
		tx := &page.Transactions[i]
		header, err := b.headers.ByHeight(tx.BlockHeight)
		if err != nil {
			err = fmt.Errorf("failed to retrieve block header for transaction %s: %w", tx.TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
		tx.BlockTimestamp = header.Timestamp

		if !expandResults {
			continue
		}

		txBody, result, err := b.lookupTransactionDetails(ctx, tx.TransactionID, header, encodingVersion)
		if err != nil {
			err = fmt.Errorf("failed to populate details for transaction %s: %w", tx.TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}

		tx.Transaction = txBody
		tx.Result = result
	}

	return &page, nil
}
