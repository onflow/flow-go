package extended

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/events"
	"github.com/onflow/flow-go/storage"
)

const scheduledTransactionsIndexerName = "scheduled_transactions"

// ScheduledTransactions indexes scheduled transaction lifecycle events from the
// FlowTransactionScheduler system contract.
//
// It processes Scheduled, PendingExecution, Executed, and Canceled events and writes
// corresponding entries to the scheduled transactions storage index.
//
// A scheduled transaction that appeared in a PendingExecution event but has no matching
// Executed event in the same block is considered failed. The corresponding Flow transaction
// that was submitted by the scheduled executor account is identified by its authorizer
// (the scheduled executor account) and an empty payer address.
//
// This indexer will automatically backfill any scheduled transactions that are executed or cancelled
// which were scheduled before the indexer was initialized. This is done by executing scripts when an
// unknown transaction is executed or cancelled within a block. There are a couple important
// considerations to keep in mind:
//  1. If there are many unknown transactions with a block, the script execution may be slow and block
//     the indexing process until it completes. Since the extended indexers are run in a batch, this
//     will block all other indexers that are indexing the same block. In general, there should be
//     relatively few unknown transactions executed. However, if this becomes a problem, we will need
//     to consider a more efficient way to backfill the index.
//  2. Since script executions are required to backfill the index, the indexer must be started after
//     the registers db is initialized.
//
// Not safe for concurrent use.
type ScheduledTransactions struct {
	log     zerolog.Logger
	store   storage.ScheduledTransactionsIndexBootstrapper
	metrics module.ExtendedIndexingMetrics

	scheduledExecutorAddr flow.Address

	scheduledEventType   flow.EventType
	pendingExecutionType flow.EventType
	executedEventType    flow.EventType
	canceledEventType    flow.EventType

	requester *ScheduledTransactionRequester
}

var _ Indexer = (*ScheduledTransactions)(nil)
var _ IndexProcessor[access.ScheduledTransaction, ScheduledTransactionsMetadata] = (*ScheduledTransactions)(nil)

// ScheduledTransactionsMetadata collects all event-derived data for a single block's scheduled
// transaction lifecycle. It contains the newly scheduled transactions as well as the executed,
// canceled, and failed lifecycle entries.
//
// Note: the complete indexed dataset is NOT available from [IndexProcessor.ProcessBlockData] alone
// because backfilling missing transactions requires storage and script execution. The full dataset
// is only assembled inside [ScheduledTransactions.IndexBlockData].
type ScheduledTransactionsMetadata struct {
	NewTxs          []access.ScheduledTransaction
	ExecutedEntries []executedEntry
	CanceledEntries []canceledEntry
	FailedEntries   []failedEntry
}

// executedEntry pairs a decoded Executed event with the Flow transaction ID that emitted it.
type executedEntry struct {
	event         *events.TransactionSchedulerExecutedEvent
	transactionID flow.Identifier
}

// canceledEntry pairs a decoded Canceled event with the Flow transaction ID that emitted it.
type canceledEntry struct {
	event         *events.TransactionSchedulerCanceledEvent
	transactionID flow.Identifier
}

// failedEntry pairs a scheduled tx ID with the Flow transaction ID of the executor transaction
// that attempted (and failed) to execute the scheduled transaction.
type failedEntry struct {
	scheduledTxID uint64
	transactionID flow.Identifier
}

// NewScheduledTransactions creates a new ScheduledTransactions indexer.
//
// No error returns are expected during normal operation.
func NewScheduledTransactions(
	log zerolog.Logger,
	store storage.ScheduledTransactionsIndexBootstrapper,
	scriptExecutor scriptExecutor,
	metrics module.ExtendedIndexingMetrics,
	chainID flow.ChainID,
) *ScheduledTransactions {
	sc := systemcontracts.SystemContractsForChain(chainID)
	scheduler := sc.FlowTransactionScheduler
	prefix := fmt.Sprintf("A.%s.%s.", scheduler.Address.Hex(), scheduler.Name)

	return &ScheduledTransactions{
		log:                   log.With().Str("component", "scheduled_tx_indexer").Logger(),
		store:                 store,
		metrics:               metrics,
		requester:             NewScheduledTransactionRequester(scriptExecutor, chainID),
		scheduledExecutorAddr: sc.ScheduledTransactionExecutor.Address,
		scheduledEventType:    flow.EventType(prefix + "Scheduled"),
		pendingExecutionType:  flow.EventType(prefix + "PendingExecution"),
		executedEventType:     flow.EventType(prefix + "Executed"),
		canceledEventType:     flow.EventType(prefix + "Canceled"),
	}
}

// Name returns the indexer name.
func (s *ScheduledTransactions) Name() string { return scheduledTransactionsIndexerName }

// NextHeight returns the next block height to index.
//
// No error returns are expected during normal operation.
func (s *ScheduledTransactions) NextHeight() (uint64, error) {
	return nextHeight(s.store)
}

// IndexBlockData processes one block's events and transactions, and updates the scheduled
// transactions index.
//
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until the batch is committed.
//
// CAUTION: Not safe for concurrent use.
//
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height
//   - [ErrFutureHeight]: if the data is for a future height
func (s *ScheduledTransactions) IndexBlockData(lctx lockctx.Proof, data BlockData, rw storage.ReaderBatchWriter) error {
	expectedHeight, err := s.NextHeight()
	if err != nil {
		return fmt.Errorf("failed to get next height: %w", err)
	}
	if data.Header.Height > expectedHeight {
		return ErrFutureHeight
	}
	if data.Header.Height < expectedHeight {
		return ErrAlreadyIndexed
	}

	_, meta, err := s.ProcessBlockData(data)
	if err != nil {
		return err
	}

	newTxs := meta.NewTxs

	// when a node is bootstrapped after a scheduled transaction was first scheduled, it will not exist
	// in the local index. In this case, calls to Executed, Cancelled, and Failed will fail because the
	// entry doesn't exist in the db. In practice, this is 100% of nodes since the indexes are reset
	// at the beginning of each spork.
	//
	// The contract doesn't provide a way to query all unexecuted transactions, so we need to find their
	// ID first, then query their data. This means it's not possible to backfill the index on startup
	// without iterating all possible IDs.
	//
	// To work around this, the logic that follows performs a just-in-time lookup of the data for each
	// unknown transaction that is executed or cancelled within a block. This is one in 3 steps:
	// 1. Collect the IDs of all transactions that are not found when attempting to update.
	// 2. Execute a script to lookup the data for each ID, and populate the executed/cancelled/failed updates
	// 3. Store the updated transactions in the index.
	var missingIDs []uint64

	for _, entry := range meta.ExecutedEntries {
		if err := s.store.Executed(lctx, rw, entry.event.ID, entry.transactionID); err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("failed to mark tx %d executed: %w", entry.event.ID, err)
			}
			missingIDs = append(missingIDs, entry.event.ID)
		}
	}
	for _, entry := range meta.CanceledEntries {
		if err := s.store.Cancelled(lctx, rw, entry.event.ID, uint64(entry.event.FeesReturned), uint64(entry.event.FeesDeducted), entry.transactionID); err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("failed to mark tx %d cancelled: %w", entry.event.ID, err)
			}
			missingIDs = append(missingIDs, entry.event.ID)
		}
	}
	for _, entry := range meta.FailedEntries {
		if err := s.store.Failed(lctx, rw, entry.scheduledTxID, entry.transactionID); err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("failed to mark tx %d failed: %w", entry.scheduledTxID, err)
			}
			missingIDs = append(missingIDs, entry.scheduledTxID)
		}
	}
	if len(missingIDs) > 0 {
		// scripts are executed against end of the block state, so the height must be before the current block,
		// otherwise the executed/canceled events may not be found. Use one block before the current block.
		// This is safe for genesis/spork root blocks because the root block does not execute transactions and
		// thus will never have any scheduled transaction events, so the block here will always be after the root block.
		missingTxs, err := s.requester.Fetch(context.TODO(), missingIDs, data.Header.Height-1, meta)
		if err != nil {
			return fmt.Errorf("failed to fetch scheduled transaction data from state: %w", err)
		}

		newTxs = append(newTxs, missingTxs...)
	}

	// at this point, all missing scheduled transactions should be present in the newTxs slice.
	if len(newTxs) < len(missingIDs) {
		return fmt.Errorf("missing backfilled scheduled transactions: expected %d, got %d", len(missingIDs), len(newTxs))
	}

	// finally store all new transactions in a single call to Store since store may only be called
	// once per block.
	if err := s.store.Store(lctx, rw, data.Header.Height, newTxs); err != nil {
		if !errors.Is(err, storage.ErrAlreadyExists) {
			return fmt.Errorf("failed to store new scheduled transactions: %w", err)
		}
	}

	s.metrics.ScheduledTransactionIndexed(
		len(newTxs)-len(missingIDs),
		len(meta.ExecutedEntries),
		len(meta.FailedEntries),
		len(meta.CanceledEntries),
		len(missingIDs),
	)

	return nil
}

// ProcessBlockData processes the block data and returns event-derived metadata for the block's
// scheduled transaction lifecycle events.
//
// The returned []access.ScheduledTransaction slice is always nil because all updates in the block
// cannot be represented by a single slice of objects. Instead, data is passed via
// [ScheduledTransactionsMetadata], partitioned into their respective lifecycle events.
//
// No error returns are expected during normal operation.
func (s *ScheduledTransactions) ProcessBlockData(data BlockData) ([]access.ScheduledTransaction, ScheduledTransactionsMetadata, error) {
	meta, err := s.collectScheduledTransactionData(data)
	if err != nil {
		return nil, ScheduledTransactionsMetadata{}, fmt.Errorf("failed to collect scheduled transaction data: %w", err)
	}
	return nil, *meta, nil
}

// collectScheduledTransactionData collects the scheduled transaction data from the block events.
//
// No error returns are expected during normal operation.
func (s *ScheduledTransactions) collectScheduledTransactionData(data BlockData) (*ScheduledTransactionsMetadata, error) {
	var newTxs []access.ScheduledTransaction
	var executedEntries []executedEntry
	var canceledEntries []canceledEntry
	var failedEntries []failedEntry

	// pendingEventTxIndex is the transaction index of the transaction that emitted the PendingExecution events.
	// This is the system transaction that added the scheduled transactions into the system collection.
	var pendingEventTxIndex *uint32

	// pendingIDs tracks the IDs that appear in PendingExecution events so we can match them with Executed events.
	// Any missing IDs are considered failed.
	pendingIDs := make(map[uint64]struct{})

	// track which IDs have Scheduled, Canceled, and Executed events to ensure an ID doesn't show up
	// more than once in the same block. This should not happen, and the indexer does not handle it.
	seenIDs := make(map[uint64]uint32)
	checkDuplicate := func(id uint64, eventIndex uint32) error {
		if lastID, ok := seenIDs[id]; ok {
			return fmt.Errorf("scheduled transaction ID %d appears more than once in block %d (txs %d and %d)",
				id, data.Header.Height, lastID, eventIndex)
		}
		seenIDs[id] = eventIndex
		return nil
	}

	for _, event := range data.Events {
		switch event.Type {
		case s.scheduledEventType:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Scheduled event payload: %w", err)
			}
			e, err := events.DecodeTransactionSchedulerScheduled(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Scheduled event: %w", err)
			}

			if err := checkDuplicate(e.ID, event.TransactionIndex); err != nil {
				return nil, err
			}

			newTxs = append(newTxs, access.ScheduledTransaction{
				ID:                               e.ID,
				Priority:                         access.ScheduledTransactionPriority(e.Priority),
				Timestamp:                        uint64(e.Timestamp),
				ExecutionEffort:                  e.ExecutionEffort,
				Fees:                             uint64(e.Fees),
				TransactionHandlerOwner:          e.TransactionHandlerOwner,
				TransactionHandlerTypeIdentifier: e.TransactionHandlerTypeIdentifier,
				TransactionHandlerUUID:           e.TransactionHandlerUUID,
				TransactionHandlerPublicPath:     e.TransactionHandlerPublicPath,
				Status:                           access.ScheduledTxStatusScheduled,
				CreatedTransactionID:             event.TransactionID,
			})

		case s.pendingExecutionType:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode PendingExecution event payload: %w", err)
			}
			e, err := events.DecodeTransactionSchedulerPendingExecution(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode PendingExecution event: %w", err)
			}
			pendingIDs[e.ID] = struct{}{}
			if pendingEventTxIndex == nil {
				pendingEventTxIndex = &event.TransactionIndex
			}

		case s.executedEventType:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Executed event payload: %w", err)
			}
			e, err := events.DecodeTransactionSchedulerExecuted(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Executed event: %w", err)
			}

			if err := checkDuplicate(e.ID, event.TransactionIndex); err != nil {
				return nil, err
			}

			executedEntries = append(executedEntries, executedEntry{event: e, transactionID: event.TransactionID})

			// sanity check: every Executed event must have a corresponding PendingExecution event.
			// otherwise, there is a bug in the indexer, or elsewhere in the system.
			if _, ok := pendingIDs[e.ID]; !ok {
				return nil, fmt.Errorf("Executed event for tx %d has no corresponding PendingExecution in block %d: protocol invariant violated",
					e.ID, data.Header.Height)
			}
			delete(pendingIDs, e.ID) // remove it so we can find failed transactions

		case s.canceledEventType:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Canceled event payload: %w", err)
			}
			e, err := events.DecodeTransactionSchedulerCanceled(cadenceEvent)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Canceled event: %w", err)
			}

			if err := checkDuplicate(e.ID, event.TransactionIndex); err != nil {
				return nil, err
			}

			canceledEntries = append(canceledEntries, canceledEntry{event: e, transactionID: event.TransactionID})
		}
	}

	// Any remaining pendingIDs were scheduled for execution but not executed — they failed.
	if len(pendingIDs) > 0 {
		if pendingEventTxIndex == nil {
			// this shouldn't be possible and indicates a bug in the indexer
			return nil, fmt.Errorf("found pending scheduled transactions, but no PendingExecution event found in block %d", data.Header.Height)
		}

		// find the transaction that attempted to execute the scheduled transactions, and mark it as failed.
		// start searching from the system transaction that adds the scheduled transactions into the
		// system collection to reduce overhead.
		for _, tx := range data.Transactions[*pendingEventTxIndex:] {
			if !s.isExecutorTransaction(tx) {
				continue
			}
			// the executor transaction must have a scheduled tx ID argument.
			if len(tx.Arguments) < 1 {
				return nil, fmt.Errorf("executor transaction %s has no scheduled tx ID argument", tx.ID())
			}

			id, err := decodeScheduledTxIDArg(tx.Arguments[0])
			if err != nil {
				return nil, fmt.Errorf("failed to decode scheduled tx ID from executor transaction: %w", err)
			}
			if _, ok := pendingIDs[id]; ok {
				failedEntries = append(failedEntries, failedEntry{scheduledTxID: id, transactionID: tx.ID()})
				delete(pendingIDs, id)
			}
		}

		// sanity check: after matching with the actual transaction in the block, pendingIDs should be empty.
		// otherwise, there were pending execution events that did not have a corresponding executor transaction.
		// this indicates there is a bug in the indexer, or elsewhere in the system.
		if len(pendingIDs) > 0 {
			ids := make([]string, 0, len(pendingIDs))
			for id := range pendingIDs {
				ids = append(ids, strconv.FormatUint(id, 10))
			}
			return nil, fmt.Errorf("PendingExecution tx (%s) have no corresponding executor transaction in block %d",
				strings.Join(ids, ", "), data.Header.Height)
		}
	}

	return &ScheduledTransactionsMetadata{
		NewTxs:          newTxs,
		ExecutedEntries: executedEntries,
		CanceledEntries: canceledEntries,
		FailedEntries:   failedEntries,
	}, nil
}

// isExecutorTransaction returns true if the transaction was submitted by the scheduled executor
// account: sole authorizer is the scheduled executor address and payer is the empty address.
func (s *ScheduledTransactions) isExecutorTransaction(tx *flow.TransactionBody) bool {
	return tx.Payer == flow.EmptyAddress &&
		len(tx.Authorizers) == 1 &&
		tx.Authorizers[0] == s.scheduledExecutorAddr
}

// decodeScheduledTxIDArg decodes a JSON-CDC encoded UInt64 argument as a scheduled tx ID.
//
// Any error indicates a malformed argument.
func decodeScheduledTxIDArg(arg []byte) (uint64, error) {
	value, err := jsoncdc.Decode(nil, arg)
	if err != nil {
		return 0, fmt.Errorf("failed to JSON-CDC decode argument: %w", err)
	}
	id, ok := value.(cadence.UInt64)
	if !ok {
		return 0, fmt.Errorf("expected UInt64 argument, got %T", value)
	}
	return uint64(id), nil
}
