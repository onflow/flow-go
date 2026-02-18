package extended

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
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
	limit = b.normalizeLimit(limit)

	page, err := b.store.TransactionsByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "account transactions", err)
	}

	if !expandResults {
		return &page, nil
	}

	// enrich the transactions with additional details requested by the client
	// Note: if no transactions are found, the response will include an empty array and no error.
	for i := range page.Transactions {
		tx := &page.Transactions[i]
		txBody, result, err := b.lookupTransactionDetails(ctx, tx.TransactionID, tx.BlockHeight, encodingVersion)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to populate details for transaction %s: %v", tx.TransactionID, err)
		}
		tx.Transaction = txBody
		tx.Result = result
	}

	return &page, nil
}
