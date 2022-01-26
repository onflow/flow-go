package herocache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionPool(t *testing.T) {
	tx1 := unittest.TransactionBodyFixture()
	tx2 := unittest.TransactionBodyFixture()

	transactions := herocache.NewTransactions(1000, unittest.Logger())

	t.Run("should be able to add first", func(t *testing.T) {
		added := transactions.Add(&tx1)
		assert.True(t, added)
	})

	t.Run("should be able to add second", func(t *testing.T) {
		added := transactions.Add(&tx2)
		assert.True(t, added)
	})

	t.Run("should be able to get size", func(t *testing.T) {
		size := transactions.Size()
		assert.EqualValues(t, 2, size)
	})

	t.Run("should be able to get first", func(t *testing.T) {
		actual, exists := transactions.ByID(tx1.ID())
		assert.True(t, exists)
		assert.Equal(t, &tx1, actual)
	})

	t.Run("should be able to remove second", func(t *testing.T) {
		ok := transactions.Rem(tx2.ID())
		assert.True(t, ok)
	})

	t.Run("should be able to retrieve all", func(t *testing.T) {
		items := transactions.All()
		assert.Len(t, items, 1)
		assert.Equal(t, &tx1, items[0])
	})

	t.Run("should be able to clear", func(t *testing.T) {
		assert.True(t, transactions.Size() > 0)
		transactions.Clear()
		assert.Equal(t, uint(0), transactions.Size())
	})
}

// TestAllReturnsInOrder checks All method of the HeroCache-based transactions mempool returns all
// transactions in the same order as they are returned.
func TestAllReturnsInOrder(t *testing.T) {
	total := 100
	txs := unittest.TransactionBodyListFixture(total)
	transactions := herocache.NewTransactions(uint32(total), unittest.Logger())

	// storing all transactions
	for i := 0; i < total; i++ {
		require.True(t, transactions.Add(&txs[i]))
		tx, ok := transactions.ByID(txs[i].ID())
		require.True(t, ok)
		require.Equal(t, txs[i], *tx)
	}

	// all transactions must be retrieved in the same order as they are added
	all := transactions.All()
	for i := 0; i < total; i++ {
		require.Equal(t, txs[i], *all[i])
	}
}
