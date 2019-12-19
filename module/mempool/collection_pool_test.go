package mempool_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/stretchr/testify/require"
)

func TestCollectionPool(t *testing.T) {
	item := &flow.GuaranteedCollection{
		CollectionHash: crypto.Hash("DEADBEEF"),
	}

	pool, err := mempool.NewCollectionPool()
	require.NoError(t, err)

	t.Run("should be able to add", func(t *testing.T) {
		err = pool.Add(item)
		assert.NoError(t, err)
	})

	t.Run("should be able to get size", func(t *testing.T) {
		size := pool.Size()
		assert.EqualValues(t, 1, size)
	})

	t.Run("should be able to get", func(t *testing.T) {
		got, err := pool.Get(item.Hash())
		assert.NoError(t, err)
		assert.Equal(t, item, got)
	})

	t.Run("should be able to get all", func(t *testing.T) {
		items := pool.All()
		assert.Len(t, items, 1)
		assert.Equal(t, item, items[0])
	})

	t.Run("should be able to remove", func(t *testing.T) {
		ok := pool.Rem(item.Hash())
		assert.True(t, ok)
	})
}
