package operation_test

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterHeights(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		var (
			clusterID flow.ChainID = "cluster"
			height    uint64       = 42
			expected               = unittest.IdentifierFixture()
			err       error
		)

		t.Run("retrieve non-existent", func(t *testing.T) {
			var actual flow.Identifier
			err = db.View(operation.LookupClusterBlockHeight(clusterID, height, &actual))
			t.Log(err)
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		})

		t.Run("insert/retrieve", func(t *testing.T) {
			err = db.Update(operation.IndexClusterBlockHeight(clusterID, height, expected))
			assert.Nil(t, err)

			var actual flow.Identifier
			err = db.View(operation.LookupClusterBlockHeight(clusterID, height, &actual))
			assert.Nil(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("multiple chain IDs", func(t *testing.T) {
			for i := 0; i < 3; i++ {
				// use different cluster ID but same block height
				clusterID = flow.ChainID(fmt.Sprintf("cluster-%d", i))
				expected = unittest.IdentifierFixture()

				var actual flow.Identifier
				err = db.View(operation.LookupClusterBlockHeight(clusterID, height, &actual))
				assert.True(t, errors.Is(err, storage.ErrNotFound))

				err = db.Update(operation.IndexClusterBlockHeight(clusterID, height, expected))
				assert.Nil(t, err)

				err = db.View(operation.LookupClusterBlockHeight(clusterID, height, &actual))
				assert.Nil(t, err)
				assert.Equal(t, expected, actual)
			}
		})
	})
}

func TestClusterBoundaries(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		var (
			clusterID flow.ChainID = "cluster"
			expected  uint64       = 42
			err       error
		)

		t.Run("retrieve non-existant", func(t *testing.T) {
			var actual uint64
			err = db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &actual))
			t.Log(err)
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		})

		t.Run("insert/retrieve", func(t *testing.T) {
			err = db.Update(operation.InsertClusterFinalizedHeight(clusterID, 21))
			assert.Nil(t, err)

			err = db.Update(operation.UpdateClusterFinalizedHeight(clusterID, expected))
			assert.Nil(t, err)

			var actual uint64
			err = db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &actual))
			assert.Nil(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("multiple chain IDs", func(t *testing.T) {
			for i := 0; i < 3; i++ {
				// use different cluster ID but same boundary
				clusterID = flow.ChainID(fmt.Sprintf("cluster-%d", i))
				expected = uint64(i)

				var actual uint64
				err = db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &actual))
				assert.True(t, errors.Is(err, storage.ErrNotFound))

				err = db.Update(operation.InsertClusterFinalizedHeight(clusterID, expected))
				assert.Nil(t, err)

				err = db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &actual))
				assert.Nil(t, err)
				assert.Equal(t, expected, actual)
			}
		})
	})
}

func TestClusterBlockByReferenceHeight(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("should be able to index cluster block by reference height", func(t *testing.T) {
			id := unittest.IdentifierFixture()
			height := rand.Uint64()
			err := db.Update(operation.IndexClusterBlockByReferenceHeight(height, id))
			assert.NoError(t, err)

			var retrieved []flow.Identifier
			err = db.View(operation.LookupClusterBlocksByReferenceHeightRange(height, height, &retrieved))
			assert.NoError(t, err)
			require.Len(t, retrieved, 1)
			assert.Equal(t, id, retrieved[0])
		})
	})

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("should be able to index multiple cluster blocks at same reference height", func(t *testing.T) {
			ids := unittest.IdentifierListFixture(10)
			height := rand.Uint64()
			for _, id := range ids {
				err := db.Update(operation.IndexClusterBlockByReferenceHeight(height, id))
				assert.NoError(t, err)
			}

			var retrieved []flow.Identifier
			err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(height, height, &retrieved))
			assert.NoError(t, err)
			assert.Len(t, retrieved, len(ids))
			assert.ElementsMatch(t, ids, retrieved)
		})
	})

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("should be able to lookup cluster blocks across height range", func(t *testing.T) {
			ids := unittest.IdentifierListFixture(100)
			nextHeight := rand.Uint64()
			// keep track of height range
			minHeight, maxHeight := nextHeight, nextHeight
			// keep track of which ids are indexed at each nextHeight
			lookup := make(map[uint64][]flow.Identifier)

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

				err := db.Update(operation.IndexClusterBlockByReferenceHeight(nextHeight, ids[i]))
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
				err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(minHeight-100, minHeight-1, &retrieved))
				assert.NoError(t, err)
				assert.Len(t, retrieved, 0)
			})
			t.Run("{-[--}-]", func(t *testing.T) {
				var retrieved []flow.Identifier
				min := minHeight - 100
				max := minHeight + (maxHeight-minHeight)/2
				err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(min, max, &retrieved))
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
				err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(min, max, &retrieved))
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
				err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(min, max, &retrieved))
				assert.NoError(t, err)

				expected := idsInHeightRange(min, max)
				assert.NotEmpty(t, expected, "test assumption broken")
				assert.Len(t, retrieved, len(expected))
				assert.ElementsMatch(t, expected, retrieved)
			})
			t.Run("[{----}]", func(t *testing.T) {
				var retrieved []flow.Identifier
				err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(minHeight, maxHeight, &retrieved))
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
				err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(min, max, &retrieved))
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
				err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(min, max, &retrieved))
				assert.NoError(t, err)

				expected := idsInHeightRange(min, max)
				assert.NotEmpty(t, expected, "test assumption broken")
				assert.Len(t, retrieved, len(expected))
				assert.ElementsMatch(t, expected, retrieved)
			})
			t.Run("[-]--{-}", func(t *testing.T) {
				var retrieved []flow.Identifier
				err := db.View(operation.LookupClusterBlocksByReferenceHeightRange(maxHeight+1, maxHeight+100, &retrieved))
				assert.NoError(t, err)
				assert.Len(t, retrieved, 0)
			})
		})
	})
}
