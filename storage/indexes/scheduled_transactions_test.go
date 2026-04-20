package indexes_test

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	"github.com/onflow/flow-go/storage/indexes/iterator"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// RunWithBootstrappedScheduledTxIndex creates a Pebble DB and bootstraps the scheduled
// transactions index at the given start height. The callback receives the DB, lock manager,
// and the bootstrapped index.
func RunWithBootstrappedScheduledTxIndex(
	tb testing.TB,
	startHeight uint64,
	f func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex),
) {
	unittest.RunWithPebbleDB(tb, func(db *pebble.DB) {
		lm := storage.NewTestingLockManager()
		storageDB := pebbleimpl.ToDB(db)

		var idx *indexes.ScheduledTransactionsIndex
		err := unittest.WithLock(tb, lm, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				var bootstrapErr error
				idx, bootstrapErr = indexes.BootstrapScheduledTransactions(lctx, rw, storageDB, startHeight, nil)
				return bootstrapErr
			})
		})
		require.NoError(tb, err)

		f(storageDB, lm, idx)
	})
}

// storeScheduledTxs is a helper that stores transactions under the required lock.
func storeScheduledTxs(
	tb testing.TB,
	lm storage.LockManager,
	idx *indexes.ScheduledTransactionsIndex,
	db storage.DB,
	blockHeight uint64,
	txs []access.ScheduledTransaction,
) error {
	return unittest.WithLock(tb, lm, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return idx.Store(lctx, rw, blockHeight, txs)
		})
	})
}

// executeTx is a helper that calls Executed under the required lock.
func executeTx(
	tb testing.TB,
	lm storage.LockManager,
	idx *indexes.ScheduledTransactionsIndex,
	db storage.DB,
	id uint64,
	transactionID flow.Identifier,
) error {
	return unittest.WithLock(tb, lm, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return idx.Executed(lctx, rw, id, transactionID)
		})
	})
}

// cancelTx is a helper that calls Cancelled under the required lock.
func cancelTx(
	tb testing.TB,
	lm storage.LockManager,
	idx *indexes.ScheduledTransactionsIndex,
	db storage.DB,
	id uint64,
	feesReturned uint64,
	feesDeducted uint64,
	transactionID flow.Identifier,
) error {
	return unittest.WithLock(tb, lm, storage.LockIndexScheduledTransactionsIndex, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return idx.Cancelled(lctx, rw, id, feesReturned, feesDeducted, transactionID)
		})
	})
}

// collectAll is a test helper that collects all results from the index using CollectResults.
func collectAll(tb testing.TB, idx *indexes.ScheduledTransactionsIndex, limit uint32, cursor *access.ScheduledTransactionCursor, filter storage.IndexFilter[*access.ScheduledTransaction]) ([]access.ScheduledTransaction, *access.ScheduledTransactionCursor) {
	tb.Helper()
	iter, err := idx.All(cursor)
	require.NoError(tb, err)
	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter)
	require.NoError(tb, err)
	return collected, nextCursor
}

// collectByAddress is a test helper that collects results for an address using CollectResults.
func collectByAddress(tb testing.TB, idx *indexes.ScheduledTransactionsIndex, addr flow.Address, limit uint32, cursor *access.ScheduledTransactionCursor, filter storage.IndexFilter[*access.ScheduledTransaction]) ([]access.ScheduledTransaction, *access.ScheduledTransactionCursor) {
	tb.Helper()
	iter, err := idx.ByAddress(addr, cursor)
	require.NoError(tb, err)
	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter)
	require.NoError(tb, err)
	return collected, nextCursor
}

// makeScheduledTx builds a minimal ScheduledTransaction with the given ID and address.
func makeScheduledTx(id uint64, addr flow.Address) access.ScheduledTransaction {
	return access.ScheduledTransaction{
		ID:                               id,
		Priority:                         1,
		Timestamp:                        1000,
		Fees:                             500,
		TransactionHandlerOwner:          addr,
		TransactionHandlerTypeIdentifier: "A.0000000000000001.Contract",
		TransactionHandlerUUID:           42,
		TransactionHandlerPublicPath:     "handler",
		Status:                           access.ScheduledTxStatusScheduled,
	}
}

func TestScheduledTransactionsIndex_StoreAndByID(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()
		tx := makeScheduledTx(100, addr)

		err := storeScheduledTxs(t, lm, idx, db, 2, []access.ScheduledTransaction{tx})
		require.NoError(t, err)

		got, err := idx.ByID(100)
		require.NoError(t, err)
		assert.Equal(t, tx.ID, got.ID)
		assert.Equal(t, tx.Priority, got.Priority)
		assert.Equal(t, tx.Timestamp, got.Timestamp)
		assert.Equal(t, tx.Fees, got.Fees)
		assert.Equal(t, tx.TransactionHandlerOwner, got.TransactionHandlerOwner)
		assert.Equal(t, tx.TransactionHandlerTypeIdentifier, got.TransactionHandlerTypeIdentifier)
		assert.Equal(t, tx.TransactionHandlerUUID, got.TransactionHandlerUUID)
		assert.Equal(t, tx.TransactionHandlerPublicPath, got.TransactionHandlerPublicPath)
		assert.Equal(t, access.ScheduledTxStatusScheduled, got.Status)
	})
}

func TestScheduledTransactionsIndex_StoreDuplicate(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()
		tx := makeScheduledTx(200, addr)

		err := storeScheduledTxs(t, lm, idx, db, 2, []access.ScheduledTransaction{tx})
		require.NoError(t, err)

		// Storing the same tx ID again always returns ErrAlreadyExists.
		err = storeScheduledTxs(t, lm, idx, db, 3, []access.ScheduledTransaction{tx})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestScheduledTransactionsIndex_Executed_Happy(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()
		tx := makeScheduledTx(400, addr)
		executedTxID := unittest.IdentifierFixture()

		err := storeScheduledTxs(t, lm, idx, db, 2, []access.ScheduledTransaction{tx})
		require.NoError(t, err)

		err = executeTx(t, lm, idx, db, 400, executedTxID)
		require.NoError(t, err)

		got, err := idx.ByID(400)
		require.NoError(t, err)
		assert.Equal(t, access.ScheduledTxStatusExecuted, got.Status)
		assert.Equal(t, executedTxID, got.ExecutedTransactionID)
	})
}

func TestScheduledTransactionsIndex_Executed_NotFound(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		err := executeTx(t, lm, idx, db, 9999, unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestScheduledTransactionsIndex_Executed_AlreadyTerminal(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()
		tx := makeScheduledTx(500, addr)

		err := storeScheduledTxs(t, lm, idx, db, 2, []access.ScheduledTransaction{tx})
		require.NoError(t, err)

		err = executeTx(t, lm, idx, db, 500, unittest.IdentifierFixture())
		require.NoError(t, err)

		// Second call should fail.
		err = executeTx(t, lm, idx, db, 500, unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrInvalidStatusTransition)
	})
}

func TestScheduledTransactionsIndex_Cancelled_Happy(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()
		tx := makeScheduledTx(600, addr)
		cancelledTxID := unittest.IdentifierFixture()

		err := storeScheduledTxs(t, lm, idx, db, 2, []access.ScheduledTransaction{tx})
		require.NoError(t, err)

		err = cancelTx(t, lm, idx, db, 600, 50, 10, cancelledTxID)
		require.NoError(t, err)

		got, err := idx.ByID(600)
		require.NoError(t, err)
		assert.Equal(t, access.ScheduledTxStatusCancelled, got.Status)
		assert.Equal(t, uint64(50), got.FeesReturned)
		assert.Equal(t, uint64(10), got.FeesDeducted)
		assert.Equal(t, cancelledTxID, got.CancelledTransactionID)
	})
}

func TestScheduledTransactionsIndex_Cancelled_AlreadyTerminal(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()
		tx := makeScheduledTx(700, addr)

		err := storeScheduledTxs(t, lm, idx, db, 2, []access.ScheduledTransaction{tx})
		require.NoError(t, err)

		err = cancelTx(t, lm, idx, db, 700, 10, 5, unittest.IdentifierFixture())
		require.NoError(t, err)

		// Second cancellation should fail.
		err = cancelTx(t, lm, idx, db, 700, 10, 5, unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrInvalidStatusTransition)
	})
}

func TestScheduledTransactionsIndex_ExecutedThenCancelled(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()
		tx := makeScheduledTx(800, addr)

		err := storeScheduledTxs(t, lm, idx, db, 2, []access.ScheduledTransaction{tx})
		require.NoError(t, err)

		err = executeTx(t, lm, idx, db, 800, unittest.IdentifierFixture())
		require.NoError(t, err)

		// Cancelling an executed tx should fail.
		err = cancelTx(t, lm, idx, db, 800, 10, 5, unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrInvalidStatusTransition)
	})
}

func TestScheduledTransactionsIndex_All_Pagination(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()

		// Store txs with IDs 1-5, one per block height.
		for i := uint64(1); i <= 5; i++ {
			tx := makeScheduledTx(i, addr)
			err := storeScheduledTxs(t, lm, idx, db, i+1, []access.ScheduledTransaction{tx})
			require.NoError(t, err)
		}

		// Page 1: limit=2, expect IDs 5, 4 (highest first).
		page1, cursor1 := collectAll(t, idx, 2, nil, nil)
		require.Len(t, page1, 2)
		require.NotNil(t, cursor1)
		assert.Equal(t, uint64(5), page1[0].ID)
		assert.Equal(t, uint64(4), page1[1].ID)

		// Page 2: use cursor, expect IDs 3, 2.
		page2, cursor2 := collectAll(t, idx, 2, cursor1, nil)
		require.Len(t, page2, 2)
		require.NotNil(t, cursor2)
		assert.Equal(t, uint64(3), page2[0].ID)
		assert.Equal(t, uint64(2), page2[1].ID)

		// Page 3: expect only ID 1, no next cursor.
		page3, cursor3 := collectAll(t, idx, 2, cursor2, nil)
		require.Len(t, page3, 1)
		assert.Nil(t, cursor3)
		assert.Equal(t, uint64(1), page3[0].ID)
	})
}

func TestScheduledTransactionsIndex_ByAddress_Pagination(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr1 := unittest.RandomAddressFixture()
		addr2 := unittest.RandomAddressFixture()

		// Store 3 txs for addr1 (IDs 1, 2, 3) and 2 txs for addr2 (IDs 4, 5).
		blockHeight := uint64(2)
		for _, tx := range []access.ScheduledTransaction{
			makeScheduledTx(1, addr1),
			makeScheduledTx(2, addr1),
			makeScheduledTx(3, addr1),
		} {
			err := storeScheduledTxs(t, lm, idx, db, blockHeight, []access.ScheduledTransaction{tx})
			require.NoError(t, err)
			blockHeight++
		}
		for _, tx := range []access.ScheduledTransaction{
			makeScheduledTx(4, addr2),
			makeScheduledTx(5, addr2),
		} {
			err := storeScheduledTxs(t, lm, idx, db, blockHeight, []access.ScheduledTransaction{tx})
			require.NoError(t, err)
			blockHeight++
		}

		// First page for addr1: limit=2, expect IDs 3, 2 (highest first).
		page1, cursor1 := collectByAddress(t, idx, addr1, 2, nil, nil)
		require.Len(t, page1, 2)
		require.NotNil(t, cursor1)
		assert.Equal(t, uint64(3), page1[0].ID)
		assert.Equal(t, uint64(2), page1[1].ID)

		// Second page for addr1: expect ID 1, no next cursor.
		page2, cursor2 := collectByAddress(t, idx, addr1, 2, cursor1, nil)
		require.Len(t, page2, 1)
		assert.Nil(t, cursor2)
		assert.Equal(t, uint64(1), page2[0].ID)

		// addr2 is unaffected: limit=10, expect IDs 5, 4.
		pageAddr2, _ := collectByAddress(t, idx, addr2, 10, nil, nil)
		require.Len(t, pageAddr2, 2)
		assert.Equal(t, uint64(5), pageAddr2[0].ID)
		assert.Equal(t, uint64(4), pageAddr2[1].ID)
	})
}

func TestScheduledTransactionsIndex_All_Filter(t *testing.T) {
	t.Parallel()

	RunWithBootstrappedScheduledTxIndex(t, 1, func(db storage.DB, lm storage.LockManager, idx *indexes.ScheduledTransactionsIndex) {
		addr := unittest.RandomAddressFixture()

		// Store 3 txs: IDs 1, 2, 3.
		for i := uint64(1); i <= 3; i++ {
			tx := makeScheduledTx(i, addr)
			err := storeScheduledTxs(t, lm, idx, db, i+1, []access.ScheduledTransaction{tx})
			require.NoError(t, err)
		}

		// Execute tx with ID=2.
		err := executeTx(t, lm, idx, db, 2, unittest.IdentifierFixture())
		require.NoError(t, err)

		// Filter to only Executed txs — should return exactly tx with ID=2.
		executedOnly := func(tx *access.ScheduledTransaction) bool {
			return tx.Status == access.ScheduledTxStatusExecuted
		}
		collected, _ := collectAll(t, idx, 10, nil, executedOnly)
		require.Len(t, collected, 1)
		assert.Equal(t, uint64(2), collected[0].ID)
		assert.Equal(t, access.ScheduledTxStatusExecuted, collected[0].Status)
	})
}
