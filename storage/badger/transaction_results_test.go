package store_test

import (
	"fmt"
	mathRand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
)

func TestBatchStoringTransactionResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st := store.NewTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txResults := make([]flow.TransactionResult, 0)
		for i := 0; i < 10; i++ {
			txID := unittest.IdentifierFixture()
			expected := flow.TransactionResult{
				TransactionID: txID,
				ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
			}
			txResults = append(txResults, expected)
		}
		writeBatch := db.NewBatch()
		defer writeBatch.Close()

		err := st.BatchStore(blockID, txResults, writeBatch)
		require.NoError(t, err)

		err = writeBatch.Commit()
		require.NoError(t, err)

		for _, txResult := range txResults {
			actual, err := st.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txResult, *actual)
		}

		// test loading from database
		newst := store.NewTransactionResults(metrics, db, 1000)
		for _, txResult := range txResults {
			actual, err := newst.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txResult, *actual)
		}

		// check retrieving by index from both cache and db
		for i := len(txResults) - 1; i >= 0; i-- {
			actual, err := st.ByBlockIDTransactionIndex(blockID, uint32(i))
			require.NoError(t, err)
			assert.Equal(t, txResults[i], *actual)

			actual, err = newst.ByBlockIDTransactionIndex(blockID, uint32(i))
			require.NoError(t, err)
			assert.Equal(t, txResults[i], *actual)
		}
	})
}

// For more info, see comment in TransactionResults.BatchRemoveByBlockID().
/*
	func TestBatchStoreAndBatchRemoveTransactionResults(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			metrics := metrics.NewNoopCollector()
			st := store.NewTransactionResults(metrics, db, 1000)

			blockID := unittest.IdentifierFixture()
			txResults := make([]flow.TransactionResult, 0)
			for i := 0; i < 10; i++ {
				txID := unittest.IdentifierFixture()
				expected := flow.TransactionResult{
					TransactionID: txID,
					ErrorMessage:  fmt.Sprintf("a runtime error %d", i),
				}
				txResults = append(txResults, expected)
			}

			// Store transaction results
			err := db.WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
				return st.BatchStore(blockID, txResults, rbw)
			})
			require.NoError(t, err)

			// Retrieve transaction results
			for _, txResult := range txResults {
				actual, err := st.ByBlockIDTransactionID(blockID, txResult.TransactionID)
				require.NoError(t, err)
				assert.Equal(t, txResult, *actual)
			}

			// Remove transaction results
			err = st.RemoveByBlockID(blockID)
			require.NoError(t, err)

			// Retrieve transaction results
			for _, txResult := range txResults {
				_, err := st.ByBlockIDTransactionID(blockID, txResult.TransactionID)
				require.True(t, errors.Is(err, storage.ErrNotFound))
			}
		})
	}
*/
func TestReadingNotstTransaction(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st := store.NewTransactionResults(metrics, db, 1000)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		_, err := st.ByBlockIDTransactionID(blockID, txID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = st.ByBlockIDTransactionIndex(blockID, txIndex)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestKeyConversion(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	txID := unittest.IdentifierFixture()
	key := store.KeyFromBlockIDTransactionID(blockID, txID)
	bID, tID := store.KeyToBlockIDTransactionID(key)
	require.Equal(t, blockID, bID)
	require.Equal(t, txID, tID)
}

func TestIndexKeyConversion(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	txIndex := mathRand.Uint32()
	key := store.KeyFromBlockIDIndex(blockID, txIndex)
	bID, tID := store.KeyToBlockIDIndex(key)
	require.Equal(t, blockID, bID)
	require.Equal(t, txIndex, tID)
}

func BenchmarkTransactionResultCacheKey(b *testing.B) {
	b.Run("new: create cache key", func(b *testing.B) {
		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()

		var key store.TwoIdentifier
		for range b.N {
			key = store.KeyFromBlockIDTransactionID(blockID, txID)
		}
		_ = key
	})

	b.Run("old: create cache key", func(b *testing.B) {
		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()

		var key string
		for range b.N {
			key = DeprecatedKeyFromBlockIDTransactionID(blockID, txID)
		}
		_ = key
	})

	b.Run("new: parse cache key", func(b *testing.B) {
		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		key := store.KeyFromBlockIDTransactionID(blockID, txID)

		var id1, id2 flow.Identifier
		for range b.N {
			id1, id2 = store.KeyToBlockIDTransactionID(key)
		}
		_ = id1
		_ = id2
	})

	b.Run("old: parse cache key", func(b *testing.B) {
		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		key := DeprecatedKeyFromBlockIDTransactionID(blockID, txID)

		var id1, id2 flow.Identifier
		for range b.N {
			id1, id2, _ = DeprecatedKeyToBlockIDTransactionID(key)
		}
		_ = id1
		_ = id2
	})
}

// This deprecated function is for benchmark purpose.
func DeprecatedKeyFromBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) string {
	return fmt.Sprintf("%x%x", blockID, txID)
}

// This deprecated function is for benchmark purpose.
func DeprecatedKeyToBlockIDTransactionID(key string) (flow.Identifier, flow.Identifier, error) {
	blockIDStr := key[:64]
	txIDStr := key[64:]
	blockID, err := flow.HexStringToIdentifier(blockIDStr)
	if err != nil {
		return flow.ZeroID, flow.ZeroID, fmt.Errorf("could not get block ID: %w", err)
	}

	txID, err := flow.HexStringToIdentifier(txIDStr)
	if err != nil {
		return flow.ZeroID, flow.ZeroID, fmt.Errorf("could not get transaction id: %w", err)
	}

	return blockID, txID, nil
}
