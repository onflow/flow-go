package extended

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/onflow/flow-go/storage/indexes/iterator"
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
	Statuses                 []accessmodel.ScheduledTransactionStatus
	Priority                 *accessmodel.ScheduledTransactionPriority
	StartTime                *uint64 // inclusive UFix64 timestamp lower bound
	EndTime                  *uint64 // inclusive UFix64 timestamp upper bound
	TransactionHandlerOwner  *flow.Address
	TransactionHandlerTypeID *string
	TransactionHandlerUUID   *uint64
}

func (f *ScheduledTransactionFilter) isEmpty() bool {
	if f == nil {
		return true
	}
	if len(f.Statuses) == 0 &&
		f.Priority == nil &&
		f.StartTime == nil &&
		f.EndTime == nil &&
		f.TransactionHandlerOwner == nil &&
		f.TransactionHandlerTypeID == nil &&
		f.TransactionHandlerUUID == nil {
		return true
	}
	return false
}

// Filter builds a [storage.IndexFilter] from the non-nil filter fields.
func (f *ScheduledTransactionFilter) Filter() storage.IndexFilter[*accessmodel.ScheduledTransaction] {
	if f.isEmpty() {
		return nil
	}

	statuses := make(map[accessmodel.ScheduledTransactionStatus]bool)
	for _, status := range f.Statuses {
		statuses[status] = true
	}

	return func(tx *accessmodel.ScheduledTransaction) bool {
		if len(statuses) > 0 && !statuses[tx.Status] {
			return false
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

	log                   zerolog.Logger
	chainID               flow.ChainID
	state                 protocol.State
	store                 storage.ScheduledTransactionsIndexReader
	contracts             storage.ContractDeploymentsIndexReader
	scheduledTransactions storage.ScheduledTransactionsReader
	scriptExecutor        execution.ScriptExecutor
}

// NewScheduledTransactionsBackend creates a new [ScheduledTransactionsBackend].
func NewScheduledTransactionsBackend(
	log zerolog.Logger,
	base *backendBase,
	chainID flow.ChainID,
	store storage.ScheduledTransactionsIndexReader,
	contracts storage.ContractDeploymentsIndexReader,
	scheduledTransactions storage.ScheduledTransactionsReader,
	state protocol.State,
	scriptExecutor execution.ScriptExecutor,
) *ScheduledTransactionsBackend {
	return &ScheduledTransactionsBackend{
		backendBase:           base,
		log:                   log,
		chainID:               chainID,
		store:                 store,
		contracts:             contracts,
		scheduledTransactions: scheduledTransactions,
		state:                 state,
		scriptExecutor:        scriptExecutor,
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
		return nil, mapReadError(ctx, "scheduled transaction", err)
	}

	if err := b.expand(ctx, &tx, expandOptions, encodingVersion); err != nil {
		err = fmt.Errorf("failed to expand scheduled transaction %d: %w", tx.ID, err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	return &tx, nil
}

// GetScheduledTransactions returns a paginated list of scheduled transactions.
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

	iter, err := b.store.All(cursor)
	if err != nil {
		return nil, mapReadError(ctx, "scheduled transactions", err)
	}

	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter.Filter())
	if err != nil {
		err = fmt.Errorf("error collecting scheduled transactions: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	page := &accessmodel.ScheduledTransactionsPage{
		Transactions: collected,
		NextCursor:   nextCursor,
	}

	for i := range page.Transactions {
		tx := &page.Transactions[i]
		if err := b.expand(ctx, tx, expandOptions, encodingVersion); err != nil {
			err = fmt.Errorf("failed to expand scheduled transaction %d: %w", tx.ID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
	}

	return page, nil
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

	iter, err := b.store.ByAddress(address, cursor)
	if err != nil {
		return nil, mapReadError(ctx, "scheduled transactions", err)
	}

	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter.Filter())
	if err != nil {
		err = fmt.Errorf("error collecting scheduled transactions: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	page := &accessmodel.ScheduledTransactionsPage{
		Transactions: collected,
		NextCursor:   nextCursor,
	}

	for i := range page.Transactions {
		tx := &page.Transactions[i]
		if err := b.expand(ctx, tx, expandOptions, encodingVersion); err != nil {
			err = fmt.Errorf("failed to expand scheduled transaction %d: %w", tx.ID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
	}

	return page, nil
}

// populateBlockTimestamps looks up the block headers for the creation and completion
// transactions and sets CreatedAt and CompletedAt on the transaction.
//
// No error returns are expected during normal operation.
func (b *ScheduledTransactionsBackend) populateBlockTimestamps(
	tx *accessmodel.ScheduledTransaction,
) (executedHeader *flow.Header, err error) {
	// `CreatedTransactionID` may be empty if this scheduled transaction was backfilled
	if tx.CreatedTransactionID != flow.ZeroID {
		header, err := b.lookupAnyTransactionBlock(tx.CreatedTransactionID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("failed to get creation block timestamp for scheduled tx %d: %w", tx.ID, err)
		}
		if err == nil {
			tx.CreatedAt = header.Timestamp
		}
		// if the created transaction was not found, don't populate the timestamp, but continue to
		// return a response. the created transaction ID will be populated, so it will be clear this
		// information is not available yet.
	}

	switch tx.Status {
	case accessmodel.ScheduledTxStatusExecuted, accessmodel.ScheduledTxStatusFailed:
		header, err := b.lookupScheduledTransactionBlock(tx.ExecutedTransactionID)
		if err != nil {
			// if the scheduled transaction record was found in the extended index, then the scheduled
			// transaction to block ID mapping must exist in storage.
			err = irrecoverable.NewException(fmt.Errorf("failed to get completion block timestamp for scheduled tx %d: %w", tx.ID, err))
			return nil, err
		}
		// Note: the executed transaction header must be found, so the method can guarantee a header
		// is returned for all executed scheduled transactions if no error is encountered.
		tx.CompletedAt = header.Timestamp
		return header, nil

	case accessmodel.ScheduledTxStatusCancelled:
		header, err := b.lookupAnyTransactionBlock(tx.CancelledTransactionID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("failed to get creation block timestamp for scheduled tx %d: %w", tx.ID, err)
		}
		if err == nil {
			tx.CompletedAt = header.Timestamp
		}
		return nil, nil
		// if the cancelled transaction was not found, don't populate the timestamp, but continue to
		// return a response. the cancelled transaction ID will be populated, so it will be clear this
		// information is not available yet.
	}

	return nil, nil
}

// lookupAnyTransactionBlock looks up the block timestamp for a transaction by its ID.
// It supports both scheduled and standard transactions.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if the transaction's block could not be resolved.
func (b *ScheduledTransactionsBackend) lookupAnyTransactionBlock(txID flow.Identifier) (*flow.Header, error) {
	header, err := b.lookupScheduledTransactionBlock(txID)
	if err == nil {
		return header, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("failed to get block timestamp for scheduled tx %s: %w", txID, err)
	}
	// the transaction may not be a scheduled transaction, so try to look up the block for a
	// standard transaction.

	header, err = b.lookupStandardTransactionBlock(txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block timestamp for standard tx %s: %w", txID, err)
	}

	return header, nil
}

// lookupStandardTransactionBlock looks up the block timestamp for a standard transaction by its ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if the transaction's block could not be resolved.
func (b *ScheduledTransactionsBackend) lookupStandardTransactionBlock(txID flow.Identifier) (*flow.Header, error) {
	collection, err := b.collections.LightByTransactionID(txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection for tx: %w", err)
	}
	collectionID := collection.ID()

	block, err := b.blocks.ByCollectionID(collectionID)
	if err != nil {
		// The txID → collectionID index (LightByTransactionID) and the collectionID → blockID index
		// (checked here) are built by separate async components: the collection Indexer and
		// the FinalizedBlockProcessor respectively. During catch-up or under load, the
		// FinalizedBlockProcessor may lag behind, causing ErrNotFound here even though the
		// collection is indexed. This is a transient state that resolves once finalization
		// processing catches up.
		//
		// Note: this will also fail if the transaction is a system transaction.
		return nil, fmt.Errorf("failed to get block ID for collection %s: %w", collectionID, err)
	}
	return block.ToHeader(), nil
}

// lookupScheduledTransactionBlock looks up the block timestamp for a scheduled transaction by its ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if the scheduled transaction did not exist in storage.
func (b *ScheduledTransactionsBackend) lookupScheduledTransactionBlock(txID flow.Identifier) (*flow.Header, error) {
	blockID, err := b.scheduledTransactions.BlockIDByTransactionID(txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block ID for scheduled tx: %w", err)
	}

	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		// if the scheduled transaction was found, the block must exist in storage.
		err = irrecoverable.NewException(fmt.Errorf("failed to get header for block %s: %w", blockID, err))
		return nil, err
	}
	return header, nil
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
	// always populate the block timestamps
	executedHeader, err := b.populateBlockTimestamps(tx)
	if err != nil {
		return fmt.Errorf("failed to get block timestamp for scheduled tx %d: %w", tx.ID, err)
	}

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
		return nil
	}

	if expandOptions.Transaction {
		txBody, err := b.systemCollections.
			ByHeight(executedHeader.Height).
			ExecuteCallbacksTransaction(b.chainID.Chain(), tx.ID, tx.ExecutionEffort)
		if err != nil {
			return fmt.Errorf("failed to construct scheduled transaction body: %w", err)
		}

		if txBody.ID() != tx.ExecutedTransactionID {
			return fmt.Errorf("scheduled transaction body ID %s does not match executed transaction ID %s", txBody.ID(), tx.ExecutedTransactionID)
		}
		tx.Transaction = txBody
	}

	if expandOptions.Result {
		result, err := b.getTransactionResult(ctx, tx.ExecutedTransactionID, executedHeader, true, expandOptions.Transaction, encodingVersion)
		if err != nil {
			return fmt.Errorf("failed to get transaction result for tx %s: %w", tx.ExecutedTransactionID, err)
		}
		tx.Result = result
	}

	return nil
}

// expandHandlerContract expands the handler contract for a scheduled transaction.
//
// No error returns are expected during normal operation.
func (b *ScheduledTransactionsBackend) expandHandlerContract(
	ctx context.Context,
	tx *accessmodel.ScheduledTransaction,
) error {
	address, contractName, err := transactionHandlerContract(tx.TransactionHandlerTypeIdentifier)
	if err != nil {
		return fmt.Errorf("failed to get contract ID for tx handler %s: %w", tx.TransactionHandlerTypeIdentifier, err)
	}

	contract, err := b.contracts.ByContract(address, contractName)

	if err == nil {
		tx.HandlerContract = &contract
		return nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to get contract for tx handler %s: %w", tx.TransactionHandlerTypeIdentifier, err)
	}

	// it's possible that the contracts index is backfilling and not caught up yet. in this case,
	// fetch the code from the node's state.

	latest, err := b.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("failed to get latest sealed block: %w", err)
	}

	code, err := b.getContractFromState(ctx, address, contractName, latest.Height)
	if err != nil {
		return fmt.Errorf("failed to get contract code for tx handler %s: %w", address, err)
	}

	tx.HandlerContract = &accessmodel.ContractDeployment{
		Address:       address,
		ContractName:  contractName,
		Code:          code,
		CodeHash:      accessmodel.CadenceCodeHash(code),
		IsPlaceholder: true,
	}

	return nil
}

func (b *ScheduledTransactionsBackend) getContractFromState(ctx context.Context, address flow.Address, contractName string, height uint64) ([]byte, error) {
	contract, err := b.scriptExecutor.GetAccountCode(ctx, address, contractName, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract code for tx handler %s: %w", address, err)
	}
	return contract, nil
}

// transactionHandlerContract extracts the contract ID from a transaction handler type identifier.
// The handler type identifier is a fully qualified Cadence type identifier of the transaction handler,
// e.g.
//
//	A.1654653399040a61.MyScheduler.Handler -> A.1654653399040a61.MyScheduler
func transactionHandlerContract(handlerTypeIdentifier string) (address flow.Address, contractName string, err error) {
	parts := strings.Split(handlerTypeIdentifier, ".")
	if len(parts) < 3 {
		return flow.Address{}, "", fmt.Errorf("invalid handler type identifier: %s", handlerTypeIdentifier)
	}

	address, err = flow.StringToAddress(parts[1])
	if err != nil {
		return flow.Address{}, "", fmt.Errorf("failed to parse address from handler type identifier: %w", err)
	}

	contractName = parts[2]

	return address, contractName, nil
}
