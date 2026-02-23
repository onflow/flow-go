package extended

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ScheduledTransactionExpandOptions struct {
	Result          bool
	Transaction     bool
	HandlerContract bool
}

func (o *ScheduledTransactionExpandOptions) HasExpand() bool {
	return o.Result || o.Transaction || o.HandlerContract
}

// ScheduledTransactionFilter specifies optional filter criteria for scheduled transaction queries.
// All fields are optional; nil/zero fields are ignored.
type ScheduledTransactionFilter struct {
	Statuses                 []accessmodel.ScheduledTxStatus
	Priority                 *uint8
	StartTime                *uint64 // inclusive UFix64 timestamp lower bound
	EndTime                  *uint64 // inclusive UFix64 timestamp upper bound
	TransactionHandlerOwner  *flow.Address
	TransactionHandlerTypeID *string
	TransactionHandlerUUID   *uint64
}

// Filter builds a [storage.IndexFilter] from the non-nil filter fields.
func (f *ScheduledTransactionFilter) Filter() storage.IndexFilter[*accessmodel.ScheduledTransaction] {
	return func(tx *accessmodel.ScheduledTransaction) bool {
		if len(f.Statuses) > 0 {
			// TODO: use a map for faster lookup.
			if !slices.Contains(f.Statuses, tx.Status) {
				return false
			}
		}
		if f.Priority != nil && tx.Priority != *f.Priority {
			return false
		}
		if f.StartTime != nil && tx.Timestamp < *f.StartTime {
			return false
		}
		if f.EndTime != nil && tx.Timestamp > *f.EndTime {
			return false
		}
		if f.TransactionHandlerOwner != nil && tx.TransactionHandlerOwner != *f.TransactionHandlerOwner {
			return false
		}
		if f.TransactionHandlerTypeID != nil && tx.TransactionHandlerTypeIdentifier != *f.TransactionHandlerTypeID {
			return false
		}
		if f.TransactionHandlerUUID != nil && tx.TransactionHandlerUUID != *f.TransactionHandlerUUID {
			return false
		}
		return true
	}
}

// ScheduledTransactionsBackend implements the extended API for querying scheduled transactions.
type ScheduledTransactionsBackend struct {
	*backendBase

	log               zerolog.Logger
	store             storage.ScheduledTransactionsIndexReader
	scheduledTxLookup storage.ScheduledTransactionsReader
	state             protocol.State
	scriptExecutor    execution.ScriptExecutor
}

// NewScheduledTransactionsBackend creates a new [ScheduledTransactionsBackend].
func NewScheduledTransactionsBackend(
	log zerolog.Logger,
	base *backendBase,
	store storage.ScheduledTransactionsIndexReader,
	scheduledTxLookup storage.ScheduledTransactionsReader,
	state protocol.State,
	scriptExecutor execution.ScriptExecutor,
) *ScheduledTransactionsBackend {
	return &ScheduledTransactionsBackend{
		backendBase:       base,
		log:               log,
		store:             store,
		scheduledTxLookup: scheduledTxLookup,
		state:             state,
		scriptExecutor:    scriptExecutor,
	}
}

// GetScheduledTransaction returns a single scheduled transaction by its scheduler-assigned ID.
//
// Expected error returns during normal operations:
//   - [codes.NotFound]: if no transaction with the given ID exists
//   - [codes.FailedPrecondition]: if the index has not been initialized
func (b *ScheduledTransactionsBackend) GetScheduledTransaction(
	ctx context.Context,
	id uint64,
	expandOptions ScheduledTransactionExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ScheduledTransaction, error) {
	tx, err := b.store.ByID(id)
	if err != nil {
		return nil, b.mapReadError(ctx, "scheduled transaction", err)
	}

	if !expandOptions.HasExpand() {
		return &tx, nil
	}

	if err := b.expand(ctx, &tx, expandOptions, encodingVersion); err != nil {
		err = fmt.Errorf("failed to expand scheduled transaction %d: %w", tx.ID, err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	return &tx, nil
}

// GetScheduledTransactions returns a paginated list of scheduled transactions.
// When filter.Address is set, results are scoped to that address; otherwise all are returned.
//
// Expected error returns during normal operations:
//   - [codes.FailedPrecondition]: if the index has not been initialized
//   - [codes.InvalidArgument]: if the query parameters are invalid
func (b *ScheduledTransactionsBackend) GetScheduledTransactions(
	ctx context.Context,
	limit uint32,
	cursor *accessmodel.ScheduledTransactionCursor,
	filter ScheduledTransactionFilter,
	expandOptions ScheduledTransactionExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ScheduledTransactionsPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	page, err := b.store.All(limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "scheduled transactions", err)
	}

	if !expandOptions.HasExpand() {
		return &page, nil
	}

	for _, tx := range page.Transactions {
		if err := b.expand(ctx, &tx, expandOptions, encodingVersion); err != nil {
			err = fmt.Errorf("failed to expand scheduled transaction %d: %w", tx.ID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
	}

	return &page, nil
}

// GetScheduledTransactionsByAddress returns a paginated list of scheduled transactions for the given address.
//
// Expected error returns during normal operations:
//   - [codes.FailedPrecondition]: if the index has not been initialized
//   - [codes.InvalidArgument]: if the query parameters are invalid
func (b *ScheduledTransactionsBackend) GetScheduledTransactionsByAddress(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.ScheduledTransactionCursor,
	filter ScheduledTransactionFilter,
	expandOptions ScheduledTransactionExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ScheduledTransactionsPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	page, err := b.store.ByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "scheduled transactions", err)
	}

	if !expandOptions.HasExpand() {
		return &page, nil
	}

	for _, tx := range page.Transactions {
		if err := b.expand(ctx, &tx, expandOptions, encodingVersion); err != nil {
			err = fmt.Errorf("failed to expand scheduled transaction %d: %w", tx.ID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
	}

	return &page, nil
}

// expand enriches an executed scheduled transaction with its transaction result.
// For non-executed transactions, this is a no-op.
//
// No error returns are expected during normal operation.
func (b *ScheduledTransactionsBackend) expand(
	ctx context.Context,
	tx *accessmodel.ScheduledTransaction,
	expandOptions ScheduledTransactionExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) error {
	if expandOptions.HandlerContract {
		err := b.expandHandlerContract(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to expand handler contract for scheduled tx %d: %w", tx.ID, err)
		}
	}

	if !expandOptions.Transaction && !expandOptions.Result {
		return nil
	}

	// if the transaction was not executed, there's nothing to expand.
	if tx.Status != accessmodel.ScheduledTxStatusExecuted && tx.Status != accessmodel.ScheduledTxStatusFailed {
		continue
	}

	txID, err := b.scheduledTxLookup.TransactionIDByID(tx.ID)
	if err != nil {
		// the transaction is marked as executed, so it must exist in storage.
		return fmt.Errorf("failed to lookup transaction ID for scheduled tx %d: %w", tx.ID, err)
	}

	blockID, err := b.scheduledTxLookup.BlockIDByTransactionID(txID)
	if err != nil {
		return fmt.Errorf("failed to lookup block ID for tx %s: %w", txID, err)
	}

	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("failed to get header for block %s: %w", blockID, err)
	}

	if expandOptions.Transaction {
		allScheduledTxs, err := b.transactionsProvider.ScheduledTransactionsByBlockID(ctx, header)
		if err != nil {
			return fmt.Errorf("could not retrieve all scheduled transactions: %w", err)
		}

		for _, scheduledTx := range allScheduledTxs {
			if scheduledTx.ID() == txID {
				tx.Transaction = scheduledTx
				break
			}
		}
		if tx.Transaction == nil {
			return fmt.Errorf("scheduled transaction %s not found in block %s", txID, blockID)
		}
	}

	if expandOptions.Result {
		result, err := b.lookupTransactionResult(ctx, txID, header, true, encodingVersion)
		if err != nil {
			return fmt.Errorf("failed to get transaction result for tx %s: %w", txID, err)
		}
		tx.Result = result
	}

	return nil
}

func (b *ScheduledTransactionsBackend) expandHandlerContract(
	ctx context.Context,
	tx *accessmodel.ScheduledTransaction,
) error {
	latest, err := b.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("failed to get latest sealed header: %w", err)
	}

	// TODO: switch to the contracts index when it's implemented
	account, err := b.scriptExecutor.GetAccountAtBlockHeight(ctx, tx.TransactionHandlerOwner, latest.Height)
	if err != nil {
		return fmt.Errorf("failed to get account for tx handler %s: %w", tx.TransactionHandlerOwner, err)
	}

	contractID, err := transactionHandlerContract(tx.TransactionHandlerTypeIdentifier)
	if err != nil {
		return fmt.Errorf("failed to get contract ID for tx handler %s: %w", tx.TransactionHandlerTypeIdentifier, err)
	}
	contract, ok := account.Contracts[contractID]
	if !ok {
		return fmt.Errorf("contract %q not found in account %s", contractID, tx.TransactionHandlerOwner)
	}

	tx.HandlerContract = &accessmodel.Contract{
		Identifier: contractID,
		Body:       string(contract),
	}

	return nil
}

// transactionHandlerContract extracts the contract ID from a transaction handler type identifier.
// The handler type identifier is a fully qualified Cadence type identifier of the transaction handler,
// e.g. A.1654653399040a61.MyScheduler.Handler -> A.1654653399040a61.MyScheduler
// The contract ID is the part before the last dot.
func transactionHandlerContract(handlerTypeIdentifier string) (string, error) {
	parts := strings.Split(handlerTypeIdentifier, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid handler type identifier: %s", handlerTypeIdentifier)
	}
	return strings.Join(parts[:3], "."), nil
}
