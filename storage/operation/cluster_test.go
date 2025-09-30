package operation_test

import (
	"math/rand"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterHeights(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		var (
			clusterID flow.ChainID = "cluster"
			height    uint64       = 42
			expected               = unittest.IdentifierFixture()
			err       error
		)

		t.Run("retrieve non-existent", func(t *testing.T) {
			var actual flow.Identifier
			err = operation.LookupClusterBlockHeight(db.Reader(), clusterID, height, &actual)
			t.Log(err)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("insert/retrieve", func(t *testing.T) {
			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexClusterBlockHeight(lctx, rw, clusterID, height, expected)
			})
			lctx.Release()
			assert.NoError(t, err)

			var actual flow.Identifier
			err = operation.LookupClusterBlockHeight(db.Reader(), clusterID, height, &actual)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("data mismatch error", func(t *testing.T) {
			// Use a different cluster ID and height to avoid conflicts with other tests
			testClusterID := flow.ChainID("test-cluster")
			testHeight := uint64(999)

			// First index a block ID for the cluster and height
			firstBlockID := unittest.IdentifierFixture()
			unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexClusterBlockHeight(lctx, rw, testClusterID, testHeight, firstBlockID)
				})
			})

			// Try to index a different block ID for the same cluster and height
			differentBlockID := unittest.IdentifierFixture()
			var err error
			unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					err = operation.IndexClusterBlockHeight(lctx, rw, testClusterID, testHeight, differentBlockID)
					return nil // Don't return the error here, we want to check it outside
				})
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cluster block height already indexed with different block ID")
			assert.Contains(t, err.Error(), "data for key is different")
			assert.ErrorIs(t, err, storage.ErrDataMismatch)
		})

		t.Run("multiple chain IDs", func(t *testing.T) {
			// use different cluster ID but same block height
			// - we first index *all* three blocks from different clusters for the same height
			// - then we retrieve *all* three block IDs in a second step
			// First writing all three is important to detect bugs, where the logic ignores the cluster ID
			// and only memorizes the latest block stored for a given height (irrespective of cluster ID).
			clusterBlockIDs := unittest.IdentifierListFixture(3)
			clusterIDs := []flow.ChainID{"cluster-0", "cluster-1", "cluster-2"}
			var actual flow.Identifier
			for i := 0; i < len(clusterBlockIDs); i++ {
				err = operation.LookupClusterBlockHeight(db.Reader(), clusterIDs[i], height, &actual)
				assert.ErrorIs(t, err, storage.ErrNotFound)

				lctx := lockManager.NewContext()
				require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexClusterBlockHeight(lctx, rw, clusterIDs[i], height, clusterBlockIDs[i])
				})
				lctx.Release() // Release lock immediately after operation
				assert.NoError(t, err)
			}
			for i := 0; i < len(clusterBlockIDs); i++ {
				err = operation.LookupClusterBlockHeight(db.Reader(), clusterIDs[i], height, &actual)
				assert.NoError(t, err)
				assert.Equal(t, clusterBlockIDs[i], actual)
			}
		})
	})
}

// Test_RetrieveClusterFinalizedHeight verifies proper retrieval of the latest finalized cluster block height.
func Test_RetrieveClusterFinalizedHeight(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		var (
			clusterID flow.ChainID = "cluster"
			err       error
		)

		t.Run("retrieve non-existent", func(t *testing.T) {
			var actual uint64
			err = operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterID, &actual)
			t.Log(err)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("insert/retrieve", func(t *testing.T) {

			unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.BootstrapClusterFinalizedHeight(lctx, rw, clusterID, 20)
				})
			})

			unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.UpdateClusterFinalizedHeight(lctx, rw, clusterID, 21)
				})
			})

			unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.UpdateClusterFinalizedHeight(lctx, rw, clusterID, 22)
				})
			})

			var actual uint64
			err = operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterID, &actual)
			assert.NoError(t, err)
			assert.Equal(t, uint64(22), actual)
		})

		t.Run("multiple chain IDs", func(t *testing.T) {
			// persist latest finalized cluster block height for three different collector clusters
			// - we first index *all* three latest finalized block heights from different clusters
			// - then we retrieve all three latest finalized block heights in a second step
			// First writing all three is important to detect bugs, where the logic ignores the cluster ID
			// and only memorizes the last value stored (irrespective of cluster ID).
			clusterFinalizedHeights := []uint64{117, 11, 791}
			clusterIDs := []flow.ChainID{"cluster-0", "cluster-1", "cluster-2"}
			var actual uint64
			for i := 0; i < len(clusterFinalizedHeights); i++ {
				err = operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterIDs[i], &actual)
				assert.ErrorIs(t, err, storage.ErrNotFound)

				unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
					return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return operation.BootstrapClusterFinalizedHeight(lctx, rw, clusterIDs[i], clusterFinalizedHeights[i])
					})
				})
			}
			for i := 0; i < len(clusterFinalizedHeights); i++ {
				err = operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterIDs[i], &actual)
				assert.NoError(t, err)
				assert.Equal(t, clusterFinalizedHeights[i], actual)
			}
		})
	})
}

func TestClusterBlockByReferenceHeight(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		t.Run("should be able to index cluster block by reference height", func(t *testing.T) {
			id := unittest.IdentifierFixture()
			height := rand.Uint64()
			unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexClusterBlockByReferenceHeight(lctx, rw.Writer(), height, id)
				})
			})

			var retrieved []flow.Identifier
			unittest.WithLock(t, lockManager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
				return operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), height, height, &retrieved)
			})
			require.Len(t, retrieved, 1)
			assert.Equal(t, id, retrieved[0])
		})
	})

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		t.Run("should be able to index multiple cluster blocks at same reference height", func(t *testing.T) {
			ids := unittest.IdentifierListFixture(10)
			height := rand.Uint64()
			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			defer lctx.Release()
			for _, id := range ids {
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexClusterBlockByReferenceHeight(lctx, rw.Writer(), height, id)
				})
				assert.NoError(t, err)
			}

			var retrieved []flow.Identifier
			err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), height, height, &retrieved)
			assert.NoError(t, err)
			assert.Len(t, retrieved, len(ids))
			assert.ElementsMatch(t, ids, retrieved)
		})
	})

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		t.Run("should be able to lookup cluster blocks across height range", func(t *testing.T) {
			ids := unittest.IdentifierListFixture(100)
			nextHeight := rand.Uint64()
			// keep track of height range
			minHeight, maxHeight := nextHeight, nextHeight
			// keep track of which ids are indexed at each nextHeight
			lookup := make(map[uint64][]flow.Identifier)
			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			defer lctx.Release()

			for i := 0; i < len(ids); i++ {
				// randomly adjust the nextHeight, increasing on average
				r := rand.Intn(100)
				if r < 20 {
					nextHeight -= 1 // 20% probability
				} else if r < 40 {
					// 20% probability: nextHeight stays the same
				} else if r < 80 {
					nextHeight += 1 // 40% probability
				} else {
					nextHeight += 2 // 20% probability
				}

				lookup[nextHeight] = append(lookup[nextHeight], ids[i])
				if nextHeight < minHeight {
					minHeight = nextHeight
				}
				if nextHeight > maxHeight {
					maxHeight = nextHeight
				}

				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.IndexClusterBlockByReferenceHeight(lctx, rw.Writer(), nextHeight, ids[i])
				})
				assert.NoError(t, err)
			}

			// determine which ids we expect to be retrieved for a given height range
			idsInHeightRange := func(min, max uint64) []flow.Identifier {
				var idsForHeight []flow.Identifier
				for height, id := range lookup {
					if min <= height && height <= max {
						idsForHeight = append(idsForHeight, id...)
					}
				}
				return idsForHeight
			}

			// Test cases are described as follows:
			// {---} represents the queried height range
			// [---] represents the indexed height range
			// [{ means the left endpoint of both ranges are the same
			// {-[ means the left endpoint of the queried range is strictly less than the indexed range
			t.Run("{-}--[-]", func(t *testing.T) {
				var retrieved []flow.Identifier
				err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), minHeight-100, minHeight-1, &retrieved)
				assert.NoError(t, err)
				assert.Len(t, retrieved, 0)
			})
			t.Run("{-[--}-]", func(t *testing.T) {
				var retrieved []flow.Identifier
				min := minHeight - 100
				max := minHeight + (maxHeight-minHeight)/2
				err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), min, max, &retrieved)
				assert.NoError(t, err)

				expected := idsInHeightRange(min, max)
				assert.NotEmpty(t, expected, "test assumption broken")
				assert.Len(t, retrieved, len(expected))
				assert.ElementsMatch(t, expected, retrieved)
			})
			t.Run("{[--}--]", func(t *testing.T) {
				var retrieved []flow.Identifier
				min := minHeight
				max := minHeight + (maxHeight-minHeight)/2
				err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), min, max, &retrieved)
				assert.NoError(t, err)

				expected := idsInHeightRange(min, max)
				assert.NotEmpty(t, expected, "test assumption broken")
				assert.Len(t, retrieved, len(expected))
				assert.ElementsMatch(t, expected, retrieved)
			})
			t.Run("[-{--}-]", func(t *testing.T) {
				var retrieved []flow.Identifier
				min := minHeight + 1
				max := maxHeight - 1
				err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), min, max, &retrieved)
				assert.NoError(t, err)

				expected := idsInHeightRange(min, max)
				assert.NotEmpty(t, expected, "test assumption broken")
				assert.Len(t, retrieved, len(expected))
				assert.ElementsMatch(t, expected, retrieved)
			})
			t.Run("[{----}]", func(t *testing.T) {
				var retrieved []flow.Identifier
				err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), minHeight, maxHeight, &retrieved)
				assert.NoError(t, err)

				expected := idsInHeightRange(minHeight, maxHeight)
				assert.NotEmpty(t, expected, "test assumption broken")
				assert.Len(t, retrieved, len(expected))
				assert.ElementsMatch(t, expected, retrieved)
			})
			t.Run("[--{--}]", func(t *testing.T) {
				var retrieved []flow.Identifier
				min := minHeight + (maxHeight-minHeight)/2
				max := maxHeight
				err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), min, max, &retrieved)
				assert.NoError(t, err)

				expected := idsInHeightRange(min, max)
				assert.NotEmpty(t, expected, "test assumption broken")
				assert.Len(t, retrieved, len(expected))
				assert.ElementsMatch(t, expected, retrieved)
			})
			t.Run("[-{--]-}", func(t *testing.T) {
				var retrieved []flow.Identifier
				min := minHeight + (maxHeight-minHeight)/2
				max := maxHeight + 100
				err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), min, max, &retrieved)
				assert.NoError(t, err)

				expected := idsInHeightRange(min, max)
				assert.NotEmpty(t, expected, "test assumption broken")
				assert.Len(t, retrieved, len(expected))
				assert.ElementsMatch(t, expected, retrieved)
			})
			t.Run("[-]--{-}", func(t *testing.T) {
				var retrieved []flow.Identifier
				err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), maxHeight+1, maxHeight+100, &retrieved)
				assert.NoError(t, err)
				assert.Len(t, retrieved, 0)
			})
		})
	})
}

// expected average case # of blocks to lookup on Mainnet
func BenchmarkLookupClusterBlocksByReferenceHeightRange_1200(b *testing.B) {
	benchmarkLookupClusterBlocksByReferenceHeightRange(b, 1200)
}

// 5x average case on Mainnet
func BenchmarkLookupClusterBlocksByReferenceHeightRange_6_000(b *testing.B) {
	benchmarkLookupClusterBlocksByReferenceHeightRange(b, 6_000)
}

func BenchmarkLookupClusterBlocksByReferenceHeightRange_100_000(b *testing.B) {
	benchmarkLookupClusterBlocksByReferenceHeightRange(b, 100_000)
}

func benchmarkLookupClusterBlocksByReferenceHeightRange(b *testing.B, n int) {
	lockManager := storage.NewTestingLockManager()
	dbtest.BenchWithStorages(b, func(b *testing.B, r storage.Reader, wr dbtest.WithWriter) {
		for i := 0; i < n; i++ {
			lctx := lockManager.NewContext()
			require.NoError(b, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			err := wr(func(w storage.Writer) error {
				return operation.IndexClusterBlockByReferenceHeight(lctx, w, rand.Uint64()%1000, unittest.IdentifierFixture())
			})
			require.NoError(b, err)
			lctx.Release()
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var blockIDs []flow.Identifier
			lctx := lockManager.NewContext()
			require.NoError(b, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, r, 0, 1000, &blockIDs)
			require.NoError(b, err)
			lctx.Release()
		}
	})
}
