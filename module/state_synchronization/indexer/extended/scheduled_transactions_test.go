package extended_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	executionmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	"github.com/onflow/flow-go/storage/indexes/iterator"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"

	. "github.com/onflow/flow-go/module/state_synchronization/indexer/extended"
)

const scheduledTestHeight = uint64(100)

// TestScheduledTransactionsIndexer_NoEvents verifies that indexing a block with no scheduler
// events stores an empty slice and advances the height.
func TestScheduledTransactionsIndexer_NoEvents(t *testing.T) {
	t.Parallel()

	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{},
	})

	latest, err := store.LatestIndexedHeight()
	require.NoError(t, err)
	assert.Equal(t, scheduledTestHeight, latest)

	allIter, err := store.All(nil)
	require.NoError(t, err)
	allTxs, _, err := iterator.CollectResults(allIter, 1000, nil)
	require.NoError(t, err)
	assert.Empty(t, allTxs)
}

// TestScheduledTransactionsIndexer_NextHeight_NotBootstrapped verifies that NextHeight returns
// the configured first height before any blocks have been indexed.
func TestScheduledTransactionsIndexer_NextHeight_NotBootstrapped(t *testing.T) {
	t.Parallel()

	indexer, _, _, _ := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)

	height, err := indexer.NextHeight()
	require.NoError(t, err)
	assert.Equal(t, scheduledTestHeight, height)
}

// TestScheduledTransactionsIndexer_ScheduledEvent verifies that a Scheduled event creates a new
// entry with status Scheduled and all fields correctly parsed, including ScheduledTransactionID.
func TestScheduledTransactionsIndexer_ScheduledEvent(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	owner := unittest.RandomAddressFixture()
	event := createScheduledEvent(t, sc, 1, 3, 1000, 500, 200, owner, "A.1234.SomeContract.Handler", 42, "")
	schedulingTxID := unittest.IdentifierFixture()
	event.TransactionID = schedulingTxID

	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{event},
	})

	tx, err := store.ByID(1)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), tx.ID)
	assert.Equal(t, access.ScheduledTransactionPriority(3), tx.Priority)
	assert.Equal(t, uint64(1000), tx.Timestamp)
	assert.Equal(t, uint64(500), tx.ExecutionEffort)
	assert.Equal(t, uint64(200), tx.Fees)
	assert.Equal(t, owner, tx.TransactionHandlerOwner)
	assert.Equal(t, "A.1234.SomeContract.Handler", tx.TransactionHandlerTypeIdentifier)
	assert.Equal(t, uint64(42), tx.TransactionHandlerUUID)
	assert.Equal(t, "", tx.TransactionHandlerPublicPath)
	assert.Equal(t, access.ScheduledTxStatusScheduled, tx.Status)
	assert.Equal(t, schedulingTxID, tx.CreatedTransactionID)
}

// TestScheduledTransactionsIndexer_ScheduledEventPublicPath verifies that the optional
// transactionHandlerPublicPath field is correctly stored when present.
func TestScheduledTransactionsIndexer_ScheduledEventPublicPath(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	owner := unittest.RandomAddressFixture()
	event := createScheduledEvent(t, sc, 1, 1, 1000, 100, 50, owner, "A.abcd.Contract.Handler", 10, "handlerCapability")

	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{event},
	})

	tx, err := store.ByID(1)
	require.NoError(t, err)
	assert.Equal(t, "/public/handlerCapability", tx.TransactionHandlerPublicPath)
}

// TestScheduledTransactionsIndexer_ExecutedWithPending verifies that a tx scheduled at height 1
// and then executed at height 2 (with PendingExecution + Executed) is correctly updated to
// Executed status. Execution must occur in a separate block from scheduling because the storage
// layer reads only committed state. Verifies ExecutedTransactionID is set.
func TestScheduledTransactionsIndexer_ExecutedWithPending(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)

	owner := unittest.RandomAddressFixture()

	// Height 1: schedule tx with id=5
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))
	scheduledEvt := createScheduledEvent(t, sc, 5, 1, 2000, 300, 100, owner, "A.abc.Contract.Handler", 99, "")
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header1,
		Events: []flow.Event{scheduledEvt},
	})

	// Height 2: execute tx with id=5
	executedTxID := unittest.IdentifierFixture()
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight+1))
	pendingEvt := createPendingExecutionEvent(t, sc, 5, 1, 300, 100, owner, "A.abc.Contract.Handler")
	executedEvt := createExecutedEvent(t, sc, 5, 1, 300, owner, "A.abc.Contract.Handler", 99, "")
	executedEvt.TransactionID = executedTxID
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header2,
		Events: []flow.Event{pendingEvt, executedEvt},
	})

	tx, err := store.ByID(5)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusExecuted, tx.Status)
	assert.Equal(t, uint64(300), tx.ExecutionEffort)
	assert.Equal(t, executedTxID, tx.ExecutedTransactionID)
}

// TestScheduledTransactionsIndexer_CanceledEvent verifies that a Canceled event at height 2
// updates an entry (created at height 1) to Cancelled status with correct fee fields.
// Verifies CancelledTransactionID is set.
func TestScheduledTransactionsIndexer_CanceledEvent(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)

	owner := unittest.RandomAddressFixture()

	// Height 1: schedule tx with id=7
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))
	scheduledEvt := createScheduledEvent(t, sc, 7, 2, 5000, 400, 150, owner, "A.def.Contract.Handler", 77, "")
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header1,
		Events: []flow.Event{scheduledEvt},
	})

	// Height 2: cancel tx with id=7
	cancelTxID := unittest.IdentifierFixture()
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight+1))
	canceledEvt := createCanceledEvent(t, sc, 7, 2, 100, 50, owner, "A.def.Contract.Handler")
	canceledEvt.TransactionID = cancelTxID
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header2,
		Events: []flow.Event{canceledEvt},
	})

	tx, err := store.ByID(7)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusCancelled, tx.Status)
	assert.Equal(t, uint64(100), tx.FeesReturned)
	assert.Equal(t, uint64(50), tx.FeesDeducted)
	assert.Equal(t, cancelTxID, tx.CancelledTransactionID)
}

// TestScheduledTransactionsIndexer_FailedTransaction verifies that a scheduled tx with a
// PendingExecution event but no Executed event is marked as Failed when a corresponding
// executor transaction is present in the block. Verifies ExecutedTransactionID is set.
func TestScheduledTransactionsIndexer_FailedTransaction(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)

	owner := unittest.RandomAddressFixture()

	// Height 1: schedule tx with id=42
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))
	scheduledEvt := createScheduledEvent(t, sc, 42, 1, 3000, 200, 80, owner, "A.xyz.Contract.Handler", 15, "")
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header1,
		Events: []flow.Event{scheduledEvt},
	})

	// Height 2: PendingExecution for tx 42, no Executed event.
	// The executor transaction attempted to execute the scheduled tx but failed.
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight+1))
	pendingEvt := createPendingExecutionEvent(t, sc, 42, 1, 200, 80, owner, "A.xyz.Contract.Handler")
	executorTx := makeExecutorTransactionBody(t, sc.ScheduledTransactionExecutor.Address, 42)
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header:       header2,
		Events:       []flow.Event{pendingEvt},
		Transactions: []*flow.TransactionBody{executorTx},
	})

	tx, err := store.ByID(42)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusFailed, tx.Status)
	assert.Equal(t, executorTx.ID(), tx.ExecutedTransactionID)
}

// TestScheduledTransactionsIndexer_PendingWithoutExecuted verifies that a PendingExecution event
// without either a matching Executed event or a corresponding executor transaction returns an error.
func TestScheduledTransactionsIndexer_PendingWithoutExecuted(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	owner := unittest.RandomAddressFixture()
	pendingEvt := createPendingExecutionEvent(t, sc, 10, 1, 300, 100, owner, "A.abc.Contract.Handler")

	err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{pendingEvt},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "have no corresponding executor transaction")
}

// TestScheduledTransactionsIndexer_ExecutedWithoutPending verifies that an Executed event without
// a matching PendingExecution event returns an error (protocol invariant violation).
func TestScheduledTransactionsIndexer_ExecutedWithoutPending(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	owner := unittest.RandomAddressFixture()
	executedEvt := createExecutedEvent(t, sc, 11, 1, 300, owner, "A.abc.Contract.Handler", 55, "")

	err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{executedEvt},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol invariant violated")
}

// TestScheduledTransactionsIndexer_DuplicateID verifies that having the same scheduled
// transaction ID appear more than once in a block (across Scheduled, Executed, or Canceled
// events) returns an error.
func TestScheduledTransactionsIndexer_DuplicateID(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()

	t.Run("duplicate Scheduled events", func(t *testing.T) {
		t.Parallel()
		indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

		evt1 := createScheduledEvent(t, sc, 5, 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", 1, "")
		evt2 := createScheduledEvent(t, sc, 5, 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", 2, "")

		err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
			Events: []flow.Event{evt1, evt2},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "appears more than once")
	})

	t.Run("duplicate Executed events", func(t *testing.T) {
		t.Parallel()
		indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

		// One PendingExecution event allows the first Executed to pass the pendingIDs check.
		// The second Executed with the same ID is caught by seenIDs before reaching pendingIDs.
		pendingEvt := createPendingExecutionEvent(t, sc, 5, 1, 100, 50, owner, "A.abc.Contract.Handler")
		executedEvt1 := createExecutedEvent(t, sc, 5, 1, 100, owner, "A.abc.Contract.Handler", 1, "")
		executedEvt2 := createExecutedEvent(t, sc, 5, 1, 100, owner, "A.abc.Contract.Handler", 1, "")

		err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
			Events: []flow.Event{pendingEvt, executedEvt1, executedEvt2},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "appears more than once")
	})

	t.Run("duplicate Canceled events", func(t *testing.T) {
		t.Parallel()
		indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

		canceledEvt1 := createCanceledEvent(t, sc, 5, 1, 100, 50, owner, "A.abc.Contract.Handler")
		canceledEvt2 := createCanceledEvent(t, sc, 5, 1, 100, 50, owner, "A.abc.Contract.Handler")

		err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
			Events: []flow.Event{canceledEvt1, canceledEvt2},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "appears more than once")
	})

	t.Run("Scheduled then Executed in same block", func(t *testing.T) {
		t.Parallel()
		indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

		scheduledEvt := createScheduledEvent(t, sc, 5, 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", 1, "")
		executedEvt := createExecutedEvent(t, sc, 5, 1, 100, owner, "A.abc.Contract.Handler", 1, "")

		err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
			Events: []flow.Event{scheduledEvt, executedEvt},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "appears more than once")
	})

	t.Run("Scheduled then Canceled in same block", func(t *testing.T) {
		t.Parallel()
		indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

		scheduledEvt := createScheduledEvent(t, sc, 5, 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", 1, "")
		canceledEvt := createCanceledEvent(t, sc, 5, 1, 100, 50, owner, "A.abc.Contract.Handler")

		err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
			Events: []flow.Event{scheduledEvt, canceledEvt},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "appears more than once")
	})
}

// TestScheduledTransactionsIndexer_AlreadyIndexed verifies that indexing the same height twice
// returns ErrAlreadyIndexed.
func TestScheduledTransactionsIndexer_AlreadyIndexed(t *testing.T) {
	t.Parallel()

	indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	// First index succeeds
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{},
	})

	// Second index of same height returns ErrAlreadyIndexed
	err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{},
	})
	require.ErrorIs(t, err, ErrAlreadyIndexed)
}

// TestScheduledTransactionsIndexer_FutureHeight verifies that indexing a future height (skipping
// heights) returns ErrFutureHeight.
func TestScheduledTransactionsIndexer_FutureHeight(t *testing.T) {
	t.Parallel()

	indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight+5))

	err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{},
	})
	require.ErrorIs(t, err, ErrFutureHeight)
}

// TestScheduledTransactionsIndexer_MultipleScheduledInOneBlock verifies that multiple Scheduled
// events in a single block are all stored with correct fields.
func TestScheduledTransactionsIndexer_MultipleScheduledInOneBlock(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	owner := unittest.RandomAddressFixture()
	evt1 := createScheduledEvent(t, sc, 1, 1, 1000, 100, 10, owner, "A.abc.Contract.HandlerA", 1, "")
	evt2 := createScheduledEvent(t, sc, 2, 2, 2000, 200, 20, owner, "A.abc.Contract.HandlerB", 2, "")
	evt3 := createScheduledEvent(t, sc, 3, 3, 3000, 300, 30, owner, "A.abc.Contract.HandlerC", 3, "")

	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{evt1, evt2, evt3},
	})

	allIter2, err := store.All(nil)
	require.NoError(t, err)
	allTxs2, _, err := iterator.CollectResults(allIter2, 1000, nil)
	require.NoError(t, err)
	require.Len(t, allTxs2, 3)

	tx1, err := store.ByID(1)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTransactionPriority(1), tx1.Priority)

	tx2, err := store.ByID(2)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTransactionPriority(2), tx2.Priority)

	tx3, err := store.ByID(3)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTransactionPriority(3), tx3.Priority)
}

// TestScheduledTransactionsIndexer_MultiplePendingExecuted verifies that multiple scheduled txs
// with PendingExecution and Executed events in the same block are all marked as Executed.
func TestScheduledTransactionsIndexer_MultiplePendingExecuted(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)

	owner := unittest.RandomAddressFixture()

	// Height 1: schedule txs 10 and 11
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))
	evt10 := createScheduledEvent(t, sc, 10, 1, 1000, 100, 10, owner, "A.abc.Contract.Handler", 10, "")
	evt11 := createScheduledEvent(t, sc, 11, 1, 1000, 200, 10, owner, "A.abc.Contract.Handler", 11, "")
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header1,
		Events: []flow.Event{evt10, evt11},
	})

	// Height 2: execute both txs
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight+1))
	pending10 := createPendingExecutionEvent(t, sc, 10, 1, 100, 10, owner, "A.abc.Contract.Handler")
	pending11 := createPendingExecutionEvent(t, sc, 11, 1, 200, 10, owner, "A.abc.Contract.Handler")
	executed10 := createExecutedEvent(t, sc, 10, 1, 100, owner, "A.abc.Contract.Handler", 10, "")
	executed11 := createExecutedEvent(t, sc, 11, 1, 200, owner, "A.abc.Contract.Handler", 11, "")
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header2,
		Events: []flow.Event{pending10, pending11, executed10, executed11},
	})

	tx10, err := store.ByID(10)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusExecuted, tx10.Status)

	tx11, err := store.ByID(11)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusExecuted, tx11.Status)
}

// TestScheduledTransactionsIndexer_MixedFailedAndExecuted verifies that in a block where some
// scheduled txs succeed and others fail, each is correctly marked with the appropriate status.
func TestScheduledTransactionsIndexer_MixedFailedAndExecuted(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)

	owner := unittest.RandomAddressFixture()

	// Height 1: schedule txs 20 and 21
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))
	evt20 := createScheduledEvent(t, sc, 20, 1, 1000, 100, 10, owner, "A.abc.Contract.Handler", 20, "")
	evt21 := createScheduledEvent(t, sc, 21, 1, 1000, 150, 10, owner, "A.abc.Contract.Handler", 21, "")
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header1,
		Events: []flow.Event{evt20, evt21},
	})

	// Height 2: tx 20 succeeds, tx 21 fails (executor tx present, no Executed event)
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight+1))
	pending20 := createPendingExecutionEvent(t, sc, 20, 1, 100, 10, owner, "A.abc.Contract.Handler")
	pending21 := createPendingExecutionEvent(t, sc, 21, 1, 150, 10, owner, "A.abc.Contract.Handler")
	executed20 := createExecutedEvent(t, sc, 20, 1, 100, owner, "A.abc.Contract.Handler", 20, "")
	executorTx21 := makeExecutorTransactionBody(t, sc.ScheduledTransactionExecutor.Address, 21)
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header:       header2,
		Events:       []flow.Event{pending20, pending21, executed20},
		Transactions: []*flow.TransactionBody{executorTx21},
	})

	tx20, err := store.ByID(20)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusExecuted, tx20.Status)

	tx21, err := store.ByID(21)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusFailed, tx21.Status)
	assert.Equal(t, executorTx21.ID(), tx21.ExecutedTransactionID)
}

// TestScheduledTransactionsIndexer_NonExecutorTxSkipped verifies that non-executor transactions
// (wrong payer, wrong authorizer, etc.) before the executor transaction are correctly skipped
// when searching for the executor of a failed scheduled transaction.
func TestScheduledTransactionsIndexer_NonExecutorTxSkipped(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, store, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)

	owner := unittest.RandomAddressFixture()

	// Height 1: schedule tx with id=30
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))
	scheduledEvt := createScheduledEvent(t, sc, 30, 1, 1000, 100, 10, owner, "A.abc.Contract.Handler", 30, "")
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header1,
		Events: []flow.Event{scheduledEvt},
	})

	// Height 2: PendingExecution for tx 30, a non-executor tx, then the real executor tx.
	// The non-executor tx has the wrong payer and should be skipped.
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight+1))
	pendingEvt := createPendingExecutionEvent(t, sc, 30, 1, 100, 10, owner, "A.abc.Contract.Handler")
	nonExecutorTx := &flow.TransactionBody{
		Payer:       unittest.RandomAddressFixture(), // wrong payer
		Authorizers: []flow.Address{sc.ScheduledTransactionExecutor.Address},
	}
	executorTx := makeExecutorTransactionBody(t, sc.ScheduledTransactionExecutor.Address, 30)
	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header:       header2,
		Events:       []flow.Event{pendingEvt},
		Transactions: []*flow.TransactionBody{nonExecutorTx, executorTx},
	})

	tx, err := store.ByID(30)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusFailed, tx.Status)
	assert.Equal(t, executorTx.ID(), tx.ExecutedTransactionID)
}

// TestScheduledTransactionsIndexer_ExecutorTxNoArguments verifies that an executor transaction
// with no arguments returns an error rather than silently skipping the failed tx.
func TestScheduledTransactionsIndexer_ExecutorTxNoArguments(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	owner := unittest.RandomAddressFixture()
	pendingEvt := createPendingExecutionEvent(t, sc, 50, 1, 100, 10, owner, "A.abc.Contract.Handler")
	executorTx := &flow.TransactionBody{
		Payer:       flow.EmptyAddress,
		Authorizers: []flow.Address{sc.ScheduledTransactionExecutor.Address},
		Arguments:   nil,
	}

	err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
		Header:       header,
		Events:       []flow.Event{pendingEvt},
		Transactions: []*flow.TransactionBody{executorTx},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no scheduled tx ID argument")
}

// TestScheduledTransactionsIndexer_ExecutorTxMalformedArg verifies that an executor transaction
// with an argument that cannot be decoded as a UInt64 returns an error.
func TestScheduledTransactionsIndexer_ExecutorTxMalformedArg(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	indexer, _, lm, db := newScheduledTxIndexerForTest(t, flow.Testnet, scheduledTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))

	owner := unittest.RandomAddressFixture()
	pendingEvt := createPendingExecutionEvent(t, sc, 50, 1, 100, 10, owner, "A.abc.Contract.Handler")

	// Valid JSON-CDC encoding but wrong type (String instead of UInt64)
	malformedArg, encErr := jsoncdc.Encode(cadence.String("not-a-uint64"))
	require.NoError(t, encErr)
	executorTx := &flow.TransactionBody{
		Payer:       flow.EmptyAddress,
		Authorizers: []flow.Address{sc.ScheduledTransactionExecutor.Address},
		Arguments:   [][]byte{malformedArg},
	}

	err := indexScheduledBlockExpectError(t, indexer, lm, db, BlockData{
		Header:       header,
		Events:       []flow.Event{pendingEvt},
		Transactions: []*flow.TransactionBody{executorTx},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode scheduled tx ID from executor transaction")
}

// TestScheduledTransactionsIndexer_NextHeight_MockErrors verifies error propagation from the store.
func TestScheduledTransactionsIndexer_NextHeight_MockErrors(t *testing.T) {
	t.Parallel()

	t.Run("unexpected error from LatestIndexedHeight propagates", func(t *testing.T) {
		mockStore := storagemock.NewScheduledTransactionsIndexBootstrapper(t)
		unexpectedErr := fmt.Errorf("disk I/O failure")
		mockStore.On("LatestIndexedHeight").Return(uint64(0), unexpectedErr)

		indexer := NewScheduledTransactions(unittest.Logger(), mockStore, nil, flow.Testnet)

		_, err := indexer.NextHeight()
		require.Error(t, err)
		require.ErrorIs(t, err, unexpectedErr)
	})

	t.Run("inconsistent state: not bootstrapped but initialized", func(t *testing.T) {
		mockStore := storagemock.NewScheduledTransactionsIndexBootstrapper(t)
		mockStore.On("LatestIndexedHeight").Return(uint64(0), storage.ErrNotBootstrapped)
		mockStore.On("UninitializedFirstHeight").Return(uint64(42), true)

		indexer := NewScheduledTransactions(unittest.Logger(), mockStore, nil, flow.Testnet)

		_, err := indexer.NextHeight()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "but index is initialized")
	})

	t.Run("store error propagates from IndexBlockData", func(t *testing.T) {
		const testHeight = uint64(100)
		mockStore := storagemock.NewScheduledTransactionsIndexBootstrapper(t)
		// LatestIndexedHeight returns testHeight-1, so NextHeight = testHeight
		mockStore.On("LatestIndexedHeight").Return(testHeight-1, nil)
		storeErr := fmt.Errorf("unexpected storage error")
		mockStore.On("Store", mock.Anything, mock.Anything, testHeight, mock.Anything).Return(storeErr)

		lm := storage.NewTestingLockManager()
		indexer := NewScheduledTransactions(unittest.Logger(), mockStore, nil, flow.Testnet)
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))

		err := unittest.WithLock(t, lm, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
			return indexer.IndexBlockData(lctx, BlockData{Header: header, Events: []flow.Event{}}, nil)
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storeErr)
	})
}

// ===== Test Setup Helpers =====

// newScheduledTxIndexerForTest creates a ScheduledTransactions indexer backed by a real pebble DB.
func newScheduledTxIndexerForTest(
	t *testing.T,
	chainID flow.ChainID,
	firstHeight uint64,
) (*ScheduledTransactions, storage.ScheduledTransactionsIndexBootstrapper, storage.LockManager, storage.DB) {
	pdb, dbDir := unittest.TempPebbleDB(t)
	db := pebbleimpl.ToDB(pdb)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})

	lm := storage.NewTestingLockManager()
	store, err := indexes.NewScheduledTransactionsBootstrapper(db, firstHeight)
	require.NoError(t, err)

	indexer := NewScheduledTransactions(unittest.Logger(), store, nil, chainID)
	return indexer, store, lm, db
}

// indexScheduledBlock runs IndexBlockData with proper locking and batch commit.
func indexScheduledBlock(
	t *testing.T,
	indexer *ScheduledTransactions,
	lm storage.LockManager,
	db storage.DB,
	data BlockData,
) {
	err := unittest.WithLock(t, lm, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return indexer.IndexBlockData(lctx, data, rw)
		})
	})
	require.NoError(t, err)
}

// indexScheduledBlockExpectError runs IndexBlockData and returns the error.
func indexScheduledBlockExpectError(
	t *testing.T,
	indexer *ScheduledTransactions,
	lm storage.LockManager,
	db storage.DB,
	data BlockData,
) error {
	return unittest.WithLock(t, lm, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return indexer.IndexBlockData(lctx, data, rw)
		})
	})
}

// ===== JIT Lookup Integration Test =====

// TestScheduledTransactionsIndexer_JITLookup verifies the end-to-end JIT path: when
// IndexBlockData encounters an unknown transaction (storage returns ErrNotFound), it
// delegates to the requester, and the result is written to storage.
// The script execution details are covered by TestScheduledTransactionRequester_* tests.
func TestScheduledTransactionsIndexer_JITLookup(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newScheduledTxIndexerWithScriptExecutor(t, flow.Testnet, scheduledTestHeight, scriptExecutor)

	// Bootstrap so Executed/Cancelled/Failed return ErrNotFound on the next block,
	// triggering the JIT path.
	bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight))
	indexScheduledBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})

	executedTxID := unittest.IdentifierFixture()
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(scheduledTestHeight+1))
	pendingEvt := createPendingExecutionEvent(t, sc, 5, 1, 300, 100, owner, "A.abc.Contract.Handler")
	executedEvt := createExecutedEvent(t, sc, 5, 1, 300, owner, "A.abc.Contract.Handler", 99, "")
	executedEvt.TransactionID = executedTxID

	scriptHeight := header.Height - 1
	comp := MakeTransactionDataComposite(sc, 5, 1, 1000, 300, 100, owner, "A.abc.Contract.Handler")
	scriptExecutor.On("ExecuteAtBlockHeight",
		mock.Anything, mock.Anything, mock.Anything, scriptHeight,
	).Return(MakeJITScriptResponse(t, comp), nil).Once()

	indexScheduledBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{pendingEvt, executedEvt},
	})

	tx, err := store.ByID(5)
	require.NoError(t, err)
	assert.Equal(t, access.ScheduledTxStatusExecuted, tx.Status)
	assert.Equal(t, executedTxID, tx.ExecutedTransactionID)
}

// ===== JIT Lookup Helpers =====

// newScheduledTxIndexerWithScriptExecutor creates an indexer backed by a real pebble DB
// with the given script executor.
func newScheduledTxIndexerWithScriptExecutor(
	t *testing.T,
	chainID flow.ChainID,
	firstHeight uint64,
	scriptExecutor *executionmock.ScriptExecutor,
) (*ScheduledTransactions, storage.ScheduledTransactionsIndexBootstrapper, storage.LockManager, storage.DB) {
	pdb, dbDir := unittest.TempPebbleDB(t)
	db := pebbleimpl.ToDB(pdb)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})

	lm := storage.NewTestingLockManager()
	store, err := indexes.NewScheduledTransactionsBootstrapper(db, firstHeight)
	require.NoError(t, err)

	indexer := NewScheduledTransactions(unittest.Logger(), store, scriptExecutor, chainID)
	return indexer, store, lm, db
}

// makeExecutorTransactionBody creates a transaction body that matches the executor transaction
// criteria: payer is the zero address, sole authorizer is the executor address, and the first
// argument is a JSON-CDC encoded UInt64 with the given scheduled tx ID.
func makeExecutorTransactionBody(t *testing.T, executorAddr flow.Address, scheduledTxID uint64) *flow.TransactionBody {
	t.Helper()
	arg, err := jsoncdc.Encode(cadence.UInt64(scheduledTxID))
	require.NoError(t, err)
	return &flow.TransactionBody{
		Payer:       flow.EmptyAddress,
		Authorizers: []flow.Address{executorAddr},
		Arguments:   [][]byte{arg},
	}
}

// schedulerEventType returns the full event type string for the given event name.
func schedulerEventType(sc *systemcontracts.SystemContracts, eventName string) flow.EventType {
	return flow.EventType(fmt.Sprintf("A.%s.%s.%s",
		sc.FlowTransactionScheduler.Address.Hex(),
		sc.FlowTransactionScheduler.Name,
		eventName,
	))
}

// schedulerEventLocation returns the Cadence address location for the scheduler contract.
func schedulerEventLocation(sc *systemcontracts.SystemContracts) common.Location {
	addr := common.Address(sc.FlowTransactionScheduler.Address)
	return common.NewAddressLocation(nil, addr, sc.FlowTransactionScheduler.Name)
}

// createScheduledEvent builds a CCF-encoded Scheduled event for the FlowTransactionScheduler.
// If publicPath is empty, the transactionHandlerPublicPath field is set to nil.
func createScheduledEvent(
	t *testing.T,
	sc *systemcontracts.SystemContracts,
	id uint64,
	priority uint8,
	timestamp uint64,
	executionEffort uint64,
	fees uint64,
	owner flow.Address,
	typeIdentifier string,
	uuid uint64,
	publicPath string,
) flow.Event {
	t.Helper()

	var publicPathValue cadence.Value
	if publicPath != "" {
		path := cadence.MustNewPath(common.PathDomainPublic, publicPath)
		publicPathValue = cadence.NewOptional(path)
	} else {
		publicPathValue = cadence.NewOptional(nil)
	}

	location := schedulerEventLocation(sc)
	eventCadenceType := cadence.NewEventType(
		location,
		"Scheduled",
		[]cadence.Field{
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "priority", Type: cadence.UInt8Type},
			{Identifier: "timestamp", Type: cadence.UFix64Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
			{Identifier: "fees", Type: cadence.UFix64Type},
			{Identifier: "transactionHandlerOwner", Type: cadence.AddressType},
			{Identifier: "transactionHandlerTypeIdentifier", Type: cadence.StringType},
			{Identifier: "transactionHandlerUUID", Type: cadence.UInt64Type},
			{Identifier: "transactionHandlerPublicPath", Type: cadence.NewOptionalType(cadence.PublicPathType)},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.UInt64(id),
		cadence.UInt8(priority),
		cadence.UFix64(timestamp),
		cadence.UInt64(executionEffort),
		cadence.UFix64(fees),
		cadence.NewAddress(owner),
		cadence.String(typeIdentifier),
		cadence.UInt64(uuid),
		publicPathValue,
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             schedulerEventType(sc, "Scheduled"),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}

// createPendingExecutionEvent builds a CCF-encoded PendingExecution event.
func createPendingExecutionEvent(
	t *testing.T,
	sc *systemcontracts.SystemContracts,
	id uint64,
	priority uint8,
	executionEffort uint64,
	fees uint64,
	owner flow.Address,
	typeIdentifier string,
) flow.Event {
	t.Helper()

	location := schedulerEventLocation(sc)
	eventCadenceType := cadence.NewEventType(
		location,
		"PendingExecution",
		[]cadence.Field{
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "priority", Type: cadence.UInt8Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
			{Identifier: "fees", Type: cadence.UFix64Type},
			{Identifier: "transactionHandlerOwner", Type: cadence.AddressType},
			{Identifier: "transactionHandlerTypeIdentifier", Type: cadence.StringType},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.UInt64(id),
		cadence.UInt8(priority),
		cadence.UInt64(executionEffort),
		cadence.UFix64(fees),
		cadence.NewAddress(owner),
		cadence.String(typeIdentifier),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             schedulerEventType(sc, "PendingExecution"),
		TransactionIndex: 0,
		EventIndex:       1,
		Payload:          payload,
	}
}

// createExecutedEvent builds a CCF-encoded Executed event.
func createExecutedEvent(
	t *testing.T,
	sc *systemcontracts.SystemContracts,
	id uint64,
	priority uint8,
	executionEffort uint64,
	owner flow.Address,
	typeIdentifier string,
	uuid uint64,
	publicPath string,
) flow.Event {
	t.Helper()

	var publicPathValue cadence.Value
	if publicPath != "" {
		path := cadence.MustNewPath(common.PathDomainPublic, publicPath)
		publicPathValue = cadence.NewOptional(path)
	} else {
		publicPathValue = cadence.NewOptional(nil)
	}

	location := schedulerEventLocation(sc)
	eventCadenceType := cadence.NewEventType(
		location,
		"Executed",
		[]cadence.Field{
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "priority", Type: cadence.UInt8Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
			{Identifier: "transactionHandlerOwner", Type: cadence.AddressType},
			{Identifier: "transactionHandlerTypeIdentifier", Type: cadence.StringType},
			{Identifier: "transactionHandlerUUID", Type: cadence.UInt64Type},
			{Identifier: "transactionHandlerPublicPath", Type: cadence.NewOptionalType(cadence.PublicPathType)},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.UInt64(id),
		cadence.UInt8(priority),
		cadence.UInt64(executionEffort),
		cadence.NewAddress(owner),
		cadence.String(typeIdentifier),
		cadence.UInt64(uuid),
		publicPathValue,
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             schedulerEventType(sc, "Executed"),
		TransactionIndex: 0,
		EventIndex:       2,
		Payload:          payload,
	}
}

// createCanceledEvent builds a CCF-encoded Canceled event.
func createCanceledEvent(
	t *testing.T,
	sc *systemcontracts.SystemContracts,
	id uint64,
	priority uint8,
	feesReturned uint64,
	feesDeducted uint64,
	owner flow.Address,
	typeIdentifier string,
) flow.Event {
	t.Helper()

	location := schedulerEventLocation(sc)
	eventCadenceType := cadence.NewEventType(
		location,
		"Canceled",
		[]cadence.Field{
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "priority", Type: cadence.UInt8Type},
			{Identifier: "feesReturned", Type: cadence.UFix64Type},
			{Identifier: "feesDeducted", Type: cadence.UFix64Type},
			{Identifier: "transactionHandlerOwner", Type: cadence.AddressType},
			{Identifier: "transactionHandlerTypeIdentifier", Type: cadence.StringType},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.UInt64(id),
		cadence.UInt8(priority),
		cadence.UFix64(feesReturned),
		cadence.UFix64(feesDeducted),
		cadence.NewAddress(owner),
		cadence.String(typeIdentifier),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             schedulerEventType(sc, "Canceled"),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}
