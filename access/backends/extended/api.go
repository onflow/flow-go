package extended

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// API defines the extended access API for querying account transaction and transfer history.
type API interface {
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
	GetAccountTransactions(
		ctx context.Context,
		address flow.Address,
		limit uint32,
		cursor *accessmodel.AccountTransactionCursor,
		filter AccountTransactionFilter,
		expandOptions AccountTransactionExpandOptions,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.AccountTransactionsPage, error)

	// GetAccountFungibleTokenTransfers returns a paginated list of fungible token transfers for
	// the given account address. Results are ordered descending by block height (newest first).
	//
	// If the account has no transfers, the response will include an empty array and no error.
	//
	// Expected error returns during normal operations:
	//   - [codes.NotFound] if the account is not found
	//   - [codes.FailedPrecondition] if the fungible token transfer index has not been initialized
	//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
	//   - [codes.InvalidArgument] if the query parameters are invalid
	GetAccountFungibleTokenTransfers(
		ctx context.Context,
		address flow.Address,
		limit uint32,
		cursor *accessmodel.TransferCursor,
		filter AccountTransferFilter,
		expandOptions AccountTransferExpandOptions,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.FungibleTokenTransfersPage, error)

	// GetAccountNonFungibleTokenTransfers returns a paginated list of non-fungible token transfers
	// for the given account address. Results are ordered descending by block height (newest first).
	//
	// If the account has no transfers, the response will include an empty array and no error.
	//
	// Expected error returns during normal operations:
	//   - [codes.NotFound] if the account is not found
	//   - [codes.FailedPrecondition] if the non-fungible token transfer index has not been initialized
	//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
	//   - [codes.InvalidArgument] if the query parameters are invalid
	GetAccountNonFungibleTokenTransfers(
		ctx context.Context,
		address flow.Address,
		limit uint32,
		cursor *accessmodel.TransferCursor,
		filter AccountTransferFilter,
		expandOptions AccountTransferExpandOptions,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.NonFungibleTokenTransfersPage, error)

	// GetScheduledTransaction returns a single scheduled transaction by its scheduler-assigned ID.
	//
	// Expected error returns during normal operations:
	//   - [codes.NotFound]: if no transaction with the given ID exists
	//   - [codes.FailedPrecondition]: if the index has not been initialized
	GetScheduledTransaction(
		ctx context.Context,
		id uint64,
		expandOptions ScheduledTransactionExpandOptions,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.ScheduledTransaction, error)

	// GetScheduledTransactions returns a paginated list of scheduled transactions.
	//
	// Expected error returns during normal operations:
	//   - [codes.FailedPrecondition]: if the index has not been initialized
	//   - [codes.InvalidArgument]: if the query parameters are invalid
	GetScheduledTransactions(
		ctx context.Context,
		limit uint32,
		cursor *accessmodel.ScheduledTransactionCursor,
		filter ScheduledTransactionFilter,
		expandOptions ScheduledTransactionExpandOptions,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.ScheduledTransactionsPage, error)

	// GetScheduledTransactionsByAddress returns a paginated list of scheduled transactions for the given address.
	//
	// Expected error returns during normal operations:
	//   - [codes.FailedPrecondition]: if the index has not been initialized
	//   - [codes.InvalidArgument]: if the query parameters are invalid
	GetScheduledTransactionsByAddress(
		ctx context.Context,
		address flow.Address,
		limit uint32,
		cursor *accessmodel.ScheduledTransactionCursor,
		filter ScheduledTransactionFilter,
		expandOptions ScheduledTransactionExpandOptions,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.ScheduledTransactionsPage, error)
}
