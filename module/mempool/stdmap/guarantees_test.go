package stdmap_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

func TestGuaranteePool(t *testing.T) {
	item1 := &flow.CollectionGuarantee{
		CollectionID: flow.Identifier{0x01},
	}
	item2 := &flow.CollectionGuarantee{
		CollectionID: flow.Identifier{0x02},
	}

	pool, err := stdmap.NewGuarantees(1000)
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
}
