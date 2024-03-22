package stdmap_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionTimingsPool(t *testing.T) {
	item1 := &flow.TransactionTiming{TransactionID: unittest.IdentifierFixture(),
		Received: time.Now().Add(-10 * time.Second), Executed: time.Now()}
	item2 := &flow.TransactionTiming{TransactionID: unittest.IdentifierFixture(), Received: time.Now()}

	pool, err := stdmap.NewTransactionTimings(1000)
	require.NoError(t, err)

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

	t.Run("should be able to adjust the first", func(t *testing.T) {
		finalized := time.Now()
		entity, updated := pool.Adjust(item1.ID(), func(t *flow.TransactionTiming) *flow.TransactionTiming {
			t.Finalized = finalized
			return t
		})
		assert.True(t, updated)
		assert.Equal(t, finalized, entity.Finalized)
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

	t.Run("should not panic if item does not exist yet", func(t *testing.T) {
		entity, updated := pool.Adjust(unittest.IdentifierFixture(), func(tt *flow.TransactionTiming) *flow.TransactionTiming {
			assert.Fail(t, "should not have found this item")
			return tt
		})
		assert.False(t, updated)
		assert.Nil(t, entity)
	})
}
