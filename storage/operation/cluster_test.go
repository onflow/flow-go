package operation_test

import (
	"fmt"
	"math/rand"
	"testing"

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
				return operation.IndexClusterBlockHeight(lctx, rw.Writer(), clusterID, height, expected)
			})
			lctx.Release()
			assert.NoError(t, err)

			var actual flow.Identifier
			err = operation.LookupClusterBlockHeight(db.Reader(), clusterID, height, &actual)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("multiple chain IDs", func(t *testing.T) {
			for i := 0; i < 3; i++ {
				// use different cluster ID but same block height
				clusterID = flow.ChainID(fmt.Sprintf("cluster-%d", i))
				expected = unittest.IdentifierFixture()

				var actual flow.Identifier
				err = operation.LookupClusterBlockHeight(db.Reader(), clusterID, height, &actual)
				assert.ErrorIs(t, err, storage.ErrNotFound)

				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					lctx := lockManager.NewContext()
					defer lctx.Release()
					if err := lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock); err != nil {
						return err
					}
					return operation.IndexClusterBlockHeight(lctx, rw.Writer(), clusterID, height, expected)
				})
				assert.NoError(t, err)

				err = operation.LookupClusterBlockHeight(db.Reader(), clusterID, height, &actual)
				assert.NoError(t, err)
				assert.Equal(t, expected, actual)
			}
		})
	})
}

func TestClusterBoundaries(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		var (
			clusterID flow.ChainID = "cluster"
			expected  uint64       = 42
			err       error
		)

		t.Run("retrieve non-existant", func(t *testing.T) {
			var actual uint64
			err = operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterID, &actual)
			t.Log(err)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("insert/retrieve", func(t *testing.T) {
			lockManager := storage.NewTestingLockManager()

			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), clusterID, 21)
			})
			assert.NoError(t, err)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), clusterID, expected)
			})
			assert.NoError(t, err)

			var actual uint64
			err = operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterID, &actual)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("multiple chain IDs", func(t *testing.T) {
			lockManager := storage.NewTestingLockManager()
			for i := 0; i < 3; i++ {
				lctx := lockManager.NewContext()
				require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
				// use different cluster ID but same boundary
				clusterID = flow.ChainID(fmt.Sprintf("cluster-%d", i))
				expected = uint64(i)

				var actual uint64
				err = operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterID, &actual)
				assert.ErrorIs(t, err, storage.ErrNotFound)

				err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.UpsertClusterFinalizedHeight(lctx, rw.Writer(), clusterID, expected)
				})

				err = operation.RetrieveClusterFinalizedHeight(db.Reader(), clusterID, &actual)
				assert.NoError(t, err)
				assert.Equal(t, expected, actual)
				lctx.Release()
			}
		})
	})
}

func TestClusterBlockByReferenceHeight(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		t.Run("should be able to index cluster block by reference height", func(t *testing.T) {
			id := unittest.IdentifierFixture()
			height := rand.Uint64()
			lctx := lockManager.NewContext()
			require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			defer lctx.Release()
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexClusterBlockByReferenceHeight(lctx, rw.Writer(), height, id)
			})
			assert.NoError(t, err)

			var retrieved []flow.Identifier
			err = operation.LookupClusterBlocksByReferenceHeightRange(lctx, db.Reader(), height, height, &retrieved)
			assert.NoError(t, err)
			require.Len(t, retrieved, 1)
			assert.Equal(t, id, retrieved[0])
		})
	})

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
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
					nextHeight -= 1 // 20%
				} else if r < 40 {
					// nextHeight stays the same - 20%
				} else if r < 80 {
					nextHeight += 1 // 40%
				} else {
					nextHeight += 2 // 20%
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
		lctx := lockManager.NewContext()
		require.NoError(b, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		defer lctx.Release()
		for i := 0; i < n; i++ {
			err := wr(func(w storage.Writer) error {
				return operation.IndexClusterBlockByReferenceHeight(lctx, w, rand.Uint64()%1000, unittest.IdentifierFixture())
			})
			require.NoError(b, err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var blockIDs []flow.Identifier
			err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, r, 0, 1000, &blockIDs)
			require.NoError(b, err)
		}
	})
}
