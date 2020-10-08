package epochs_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
)

// subsequent calls to Get should return the same transaction pool
func TestConsistency(t *testing.T) {

	create := func() mempool.Transactions { return stdmap.NewTransactions(100) }
	pools := epochs.NewTransactionPools(create)
	epoch := rand.Uint64()

	pool := pools.Get(epoch)
	assert.Equal(t, pool, pools.Get(epoch))
}

// test that different epochs don't interfere, also test concurrent access
func TestMultipleEpochs(t *testing.T) {

	create := func() mempool.Transactions { return stdmap.NewTransactions(100) }
	pools := epochs.NewTransactionPools(create)

	for i := 0; i < 10; i++ {
		go func() {
			epoch := rand.Uint64()

			var transactions []*flow.TransactionBody
			for i := 0; i < 10; i++ {
				pool := pools.Get(epoch)
				assert.Equal(t, uint(len(transactions)), pool.Size())
				for _, tx := range transactions {
					assert.True(t, pool.Has(tx.ID()))
				}

				tx := unittest.TransactionBodyFixture()
				transactions = append(transactions, &tx)
			}
		}()
	}
}
