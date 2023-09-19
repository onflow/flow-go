package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBatchStoringLightTransactionResults(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewLightTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResults := getLightTransactionResultsFixture(10)

		t.Run("batch store results", func(t *testing.T) {
			writeBatch := bstorage.NewBatch(db)
			err := store.BatchStore(blockID, txResults, writeBatch)
			require.NoError(t, err)

			err = writeBatch.Flush()
			require.NoError(t, err)

			// add a results to a new block to validate they are not included in lookups
			writeBatch = bstorage.NewBatch(db)
			err = store.BatchStore(unittest.IdentifierFixture(), getLightTransactionResultsFixture(2), writeBatch)
			require.NoError(t, err)

			err = writeBatch.Flush()
			require.NoError(t, err)
		})

		t.Run("read results with cache", func(t *testing.T) {
			for _, txResult := range txResults {
				actual, err := store.ByBlockIDTransactionID(blockID, txResult.TransactionID)
				require.NoError(t, err)
				assert.Equal(t, txResult, *actual)
			}
		})

		newStore := bstorage.NewLightTransactionResults(metrics, db, 1000)
		t.Run("read results without cache", func(t *testing.T) {
			// test loading from database (without cache)
			// create a new instance using the same db so it has an empty cache
			for _, txResult := range txResults {
				actual, err := newStore.ByBlockIDTransactionID(blockID, txResult.TransactionID)
				require.NoError(t, err)
				assert.Equal(t, txResult, *actual)
			}
		})

		t.Run("cached and non-cached results are equal", func(t *testing.T) {
			// check retrieving by index from both cache and db
			for i := len(txResults) - 1; i >= 0; i-- {
				actual, err := store.ByBlockIDTransactionIndex(blockID, uint32(i))
				require.NoError(t, err)
				assert.Equal(t, txResults[i], *actual)

				actual, err = newStore.ByBlockIDTransactionIndex(blockID, uint32(i))
				require.NoError(t, err)
				assert.Equal(t, txResults[i], *actual)
			}
		})

		t.Run("read all results for block", func(t *testing.T) {
			actuals, err := store.ByBlockID(blockID)
			require.NoError(t, err)

			assert.Equal(t, len(txResults), len(actuals))
			for i := range txResults {
				assert.Equal(t, txResults[i], actuals[i])
			}
		})
	})
}

func TestReadingNotStoredLightTransactionResults(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewLightTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		_, err := store.ByBlockIDTransactionID(blockID, txID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = store.ByBlockIDTransactionIndex(blockID, txIndex)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func getLightTransactionResultsFixture(n int) []flow.LightTransactionResult {
	txResults := make([]flow.LightTransactionResult, 0, n)
	for i := 0; i < n; i++ {
		expected := flow.LightTransactionResult{
			TransactionID:   unittest.IdentifierFixture(),
			Failed:          i%2 == 0,
			ComputationUsed: unittest.Uint64InRange(1, 1000),
		}
		txResults = append(txResults, expected)
	}
	return txResults
}
