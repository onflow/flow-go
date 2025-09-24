package store_test

import (
	"errors"
	"fmt"
	mathRand "math/rand"
	"slices"
	"testing"

	"github.com/jordanschalm/lockctx"
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
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st, err := store.NewTransactionResults(metrics, db, 1000)
		require.NoError(t, err)

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
		unittest.WithLock(t, lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return st.BatchStore(lctx, blockID, txResults, rw)
			})
		})

		for _, txResult := range txResults {
			actual, err := st.ByBlockIDTransactionID(blockID, txResult.TransactionID)
			require.NoError(t, err)
			assert.Equal(t, txResult, *actual)
		}

		// test loading from database
		newst, err := store.NewTransactionResults(metrics, db, 1000)
		require.NoError(t, err)
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

func TestBatchStoreAndBatchRemoveTransactionResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		const blockCount = 10
		const txCountPerBlock = 10

		metrics := metrics.NewNoopCollector()
		st, err := store.NewTransactionResults(metrics, db, 1000)
		require.NoError(t, err)

		blockIDs := make([]flow.Identifier, blockCount)
		txResults := make(map[flow.Identifier][]flow.TransactionResult)
		for i := range blockCount {
			blockID := unittest.IdentifierFixture()
			blockIDs[i] = blockID

			for j := range txCountPerBlock {
				txID := unittest.IdentifierFixture()
				expected := flow.TransactionResult{
					TransactionID: txID,
					ErrorMessage:  fmt.Sprintf("a runtime error %d", j),
				}
				txResults[blockID] = append(txResults[blockID], expected)
			}
		}

		// Store transaction results of multiple blocks
		err = db.WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
			for _, blockID := range blockIDs {
				err := st.BatchStore(blockID, txResults[blockID], rbw)
				if err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)

		// Retrieve transaction results
		for blockID, txResult := range txResults {
			for _, result := range txResult {
				actual, err := st.ByBlockIDTransactionID(blockID, result.TransactionID)
				require.NoError(t, err)
				assert.Equal(t, result, *actual)
			}
		}

		// Remove 2 blocks of transaction results
		removeBlockIDs := blockIDs[:2]

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, blockID := range removeBlockIDs {
				err := st.BatchRemoveByBlockID(blockID, rw)
				if err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)

		// Retrieve transaction results
		for blockID, txResult := range txResults {
			removedBlock := slices.Contains(removeBlockIDs, blockID)

			for _, result := range txResult {
				actual, err := st.ByBlockIDTransactionID(blockID, result.TransactionID)
				if removedBlock {
					require.True(t, errors.Is(err, storage.ErrNotFound))
				} else {
					require.NoError(t, err)
					assert.Equal(t, result, *actual)
				}
			}
		}
	})
}

func TestReadingNotstTransaction(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		st, err := store.NewTransactionResults(metrics, db, 1000)
		require.NoError(t, err)

		blockID := unittest.IdentifierFixture()
		txID := unittest.IdentifierFixture()
		txIndex := rand.Uint32()

		_, err = st.ByBlockIDTransactionID(blockID, txID)
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
