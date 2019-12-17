package mempool_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/module/mempool"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/stretchr/testify/require"
)

type hashable []byte

func (h hashable) Hash() crypto.Hash {
	return crypto.Hash(h)
}

func TestHash(t *testing.T) {
	item1 := hashable("DEAD")
	item2 := hashable("BEEF")

	t.Run("insertion order should not impact the hash", func(t *testing.T) {
		// create the first mempool with item1 and gc2
		pool1, err := mempool.NewMempool()
		require.NoError(t, err)
		_ = pool1.Add(item1)
		_ = pool1.Add(item2)

		// create the second mempool with gc2 and item1
		pool2, err := mempool.NewMempool()
		require.NoError(t, err)
		_ = pool2.Add(item2)
		_ = pool2.Add(item1)

		// check pool1 and pool2 should have the same hash
		require.Equal(t, pool1.Hash(), pool2.Hash())
	})

	t.Run("having different items should produce different hash", func(t *testing.T) {
		// create the first mempool with only item1
		pool1, err := mempool.NewMempool()
		require.NoError(t, err)
		_ = pool1.Add(item1)

		// create the second mempool with item1 and item2
		pool2, err := mempool.NewMempool()
		require.NoError(t, err)
		_ = pool2.Add(item1)
		_ = pool1.Add(item2)

		// check pool1 and pool2 should have the different hash
		require.NotEqual(t, pool1.Hash(), pool2.Hash())
	})

	t.Run("insert duplicated items should not impact hash", func(t *testing.T) {
		// create the first mempool with item1 and item2
		pool, err := mempool.NewMempool()
		require.NoError(t, err)
		_ = pool.Add(item1)
		_ = pool.Add(item2)

		hash1 := pool.Hash()

		// add gc2 again
		_ = pool.Add(item2)

		// check hash should not change
		require.Equal(t, hash1, pool.Hash())
	})
}

func TestInsert(t *testing.T) {
	item1 := hashable("DEAD")

	t.Run("should be able to insert and retrieve", func(t *testing.T) {
		pool, err := mempool.NewMempool()
		require.NoError(t, err)
		_ = pool.Add(item1)

		t.Run("should be able to get size", func(t *testing.T) {
			size := pool.Size()
			assert.EqualValues(t, 1, size)
		})

		t.Run("should be able to get by hash", func(t *testing.T) {
			gotItem, err := pool.Get(item1.Hash())
			assert.NoError(t, err)
			assert.Equal(t, item1, gotItem)
		})

		t.Run("should be able to get all", func(t *testing.T) {
			items := pool.All()
			require.Len(t, items, 1)
			assert.Equal(t, item1, items[0])
		})

		t.Run("should be able to remove", func(t *testing.T) {
			ok := pool.Rem(item1.Hash())
			assert.True(t, ok)

			size := pool.Size()
			assert.EqualValues(t, 0, size)
		})
	})
}
