package epochs_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// subsequent calls to Get should return the same transaction pool
func TestConsistency(t *testing.T) {

	create := func(_ uint64) mempool.Transactions {
		return herocache.NewTransactions(100, unittest.Logger(), metrics.NewNoopCollector())
	}
	pools := epochs.NewTransactionPools(create)
	epoch := rand.Uint64()

	pool := pools.ForEpoch(epoch)
	assert.Equal(t, pool, pools.ForEpoch(epoch))
}

// test that different epochs don't interfere, also test concurrent access
func TestMultipleEpochs(t *testing.T) {

	create := func(_ uint64) mempool.Transactions {
		return herocache.NewTransactions(100, unittest.Logger(), metrics.NewNoopCollector())
	}
	pools := epochs.NewTransactionPools(create)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			epoch := rand.Uint64()

			var transactions []*flow.TransactionBody
			for i := 0; i < 10; i++ {
				pool := pools.ForEpoch(epoch)
				assert.Equal(t, uint(len(transactions)), pool.Size())
				for _, tx := range transactions {
					assert.True(t, pool.Has(tx.ID()))
				}

				tx := unittest.TransactionBodyFixture()
				transactions = append(transactions, &tx)
				pool.Add(&tx)
			}
		}()
	}
	unittest.AssertReturnsBefore(t, wg.Wait, time.Second)
}

func TestCombinedSize(t *testing.T) {

	create := func(_ uint64) mempool.Transactions {
		return herocache.NewTransactions(100, unittest.Logger(), metrics.NewNoopCollector())
	}
	pools := epochs.NewTransactionPools(create)

	nEpochs := rand.Uint64() % 10
	transactionsPerEpoch := rand.Uint64() % 10
	expected := uint(nEpochs * transactionsPerEpoch)

	for epoch := uint64(0); epoch < nEpochs; epoch++ {
		pool := pools.ForEpoch(epoch)
		for i := 0; i < int(transactionsPerEpoch); i++ {
			next := unittest.TransactionBodyFixture()
			pool.Add(&next)
		}
	}

	assert.Equal(t, expected, pools.CombinedSize())
}
