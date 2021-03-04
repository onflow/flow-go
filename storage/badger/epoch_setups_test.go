package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// TestEpochSetupStoreAndRetrieve tests that a setup can be stored, retrieved and attempted to be stored again without an error
func TestEpochSetupStoreAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewEpochSetups(metrics, db)

		// attempt to get a setup that doesn't exist
		_, err := store.ByID(unittest.IdentifierFixture())
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a setup in db
		expected := unittest.EpochSetupFixture()
		err = operation.RetryOnConflict(db.Update, store.StoreTx(expected))
		require.NoError(t, err)

		// retrieve the setup by ID
		actual, err := store.ByID(expected.ID())
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same epoch setup
		err = operation.RetryOnConflict(db.Update, store.StoreTx(expected))
		require.True(t, errors.Is(err, storage.ErrAlreadyExists))

		// test get by view
		middleView := expected.FirstView + (expected.FinalView-expected.FirstView)/2
		counter, err := store.CounterByView(middleView)
		require.NoError(t, err)
		require.Equal(t, expected.Counter, counter)
	})
}

// TestEpochSetupLoadLookup checks that the lookup cache is loaded with existing
// items upon instantiation.
func TestEpochSetupLoadLookup(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// add items to the DB
		setups := []*flow.EpochSetup{}
		for i := 0; i < 10; i++ {
			epochSetup := unittest.EpochSetupFixture()
			epochSetup.FirstView = uint64(i) * 100
			epochSetup.FinalView = epochSetup.FirstView + 99
			setups = append(setups, epochSetup)
		}

		// use a first instance to add items to the db
		firstStorage := badgerstorage.NewEpochSetups(metrics.NewNoopCollector(), db)
		for _, s := range setups {
			err := operation.RetryOnConflict(db.Update, firstStorage.StoreTx(s))
			require.NoError(t, err)
		}

		// instantiate a second instance and ensure that the cache is loaded
		// items from the db
		secondStorage := badgerstorage.NewEpochSetups(metrics.NewNoopCollector(), db)
		for _, s := range setups {
			for i := s.FirstView; i <= s.FinalView; i++ {
				e, err := secondStorage.CounterByView(i)
				require.NoError(t, err)
				require.Equal(t, s.Counter, e)
			}
		}
	})
}
