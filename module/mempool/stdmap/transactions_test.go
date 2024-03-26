package stdmap_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionPool(t *testing.T) {
	tx1 := unittest.TransactionBodyFixture()
	item1 := &tx1

	tx2 := unittest.TransactionBodyFixture()
	item2 := &tx2

	pool := stdmap.NewTransactions(1000)

	t.Run("should be able to add first", func(t *testing.T) {
		added := pool.Add(item1)
		assert.True(t, added)
	})

	t.Run("should be able to add second", func(t *testing.T) {
		added := pool.Add(item2)
		assert.True(t, added)
	})

	t.Run("should be able to get size", func(t *testing.T) {
		size := pool.Size()
		assert.EqualValues(t, 2, size)
	})

	t.Run("should be able to get first", func(t *testing.T) {
		got, exists := pool.ByID(item1.ID())
		assert.True(t, exists)
		assert.Equal(t, item1, got)
	})

	t.Run("should be able to remove second", func(t *testing.T) {
		ok := pool.Remove(item2.ID())
		assert.True(t, ok)
	})

	t.Run("should be able to retrieve all", func(t *testing.T) {
		items := pool.All()
		assert.Len(t, items, 1)
		assert.Equal(t, item1, items[0])
	})

	t.Run("should be able to clear", func(t *testing.T) {
		assert.True(t, pool.Size() > 0)
		pool.Clear()
		assert.Equal(t, uint(0), pool.Size())
	})
}
