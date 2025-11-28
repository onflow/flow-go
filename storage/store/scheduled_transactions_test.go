package store

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

func TestScheduledTransactionsStoreAndRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		g := fixtures.NewGeneratorSuite()
		metrics := metrics.NewNoopCollector()

		t.Run("with cache", func(t *testing.T) {
			store := NewScheduledTransactions(metrics, db, DefaultCacheSize)
			runTest(t, g, db, store)
		})

		// tests that store and retrieve operations still succeed when there are cache misses.
		t.Run("reduced cache", func(t *testing.T) {
			store := NewScheduledTransactions(metrics, db, 1)
			runTest(t, g, db, store)
		})
	})
}

func runTest(t *testing.T, g *fixtures.GeneratorSuite, db storage.DB, store *ScheduledTransactions) {
	blockID := g.Identifiers().Fixture()
	txIDs := g.Identifiers().List(50)
	data := make(map[flow.Identifier]uint64, len(txIDs))
	for _, txID := range txIDs {
		data[txID] = g.Random().Uint64()
	}

	lockManager := storage.NewTestingLockManager()
	lctx := lockManager.NewContext()
	defer lctx.Release()
	if err := lctx.AcquireLock(storage.LockIndexScheduledTransaction); err != nil {
		t.Fatalf("could not acquire lock for indexing scheduled transactions: %v", err)
	}

	// index data within a batch
	batch := db.NewBatch()
	defer batch.Close()

	for txID, scheduledTxID := range data {
		err := store.BatchIndex(lctx, blockID, txID, scheduledTxID, batch)
		require.NoError(t, err)
	}

	err := batch.Commit()
	require.NoError(t, err)

	// check all indexes are available
	for txID, scheduledTxID := range data {
		actualTxID, err := store.TransactionIDByID(scheduledTxID)
		require.NoError(t, err)
		require.Equal(t, txID, actualTxID)

		actualBlockID, err := store.BlockIDByTransactionID(txID)
		require.NoError(t, err)
		require.Equal(t, blockID, actualBlockID)
	}

	// check unknown values return storage.ErrNotFound
	unknownTxID, err := store.TransactionIDByID(99)
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.Equal(t, flow.ZeroID, unknownTxID)

	unknownBlockID, err := store.BlockIDByTransactionID(g.Identifiers().Fixture())
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.Equal(t, flow.ZeroID, unknownBlockID)
}
