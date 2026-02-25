package extended

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// API defines the extended access API for querying account transaction history.
type API interface {
	// GetAccountTransactions returns a paginated list of transactions for the given account address.
	// Results are ordered descending by block height (newest first).
	//
	// If the account is found but has no transactions, the response will include an empty array and no error.
	//
	// Expected error returns during normal operations:
	//   - [codes.FailedPrecondition] if the account transaction index has not been initialized
	//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
	GetAccountTransactions(
		ctx context.Context,
		address flow.Address,
		limit uint32,
		cursor *accessmodel.AccountTransactionCursor,
		filter AccountTransactionFilter,
		expandOptions AccountTransactionExpandOptions,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.AccountTransactionsPage, error)
}
