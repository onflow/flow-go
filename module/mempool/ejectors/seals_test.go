package ejectors_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/ejectors"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLatestSealEjector(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		const N = 10

		headers := storage.NewHeaders(metrics.NewNoopCollector(), db)
		ejector := ejectors.NewLatestSeal(headers)

		pool, err := stdmap.NewSeals(N, stdmap.WithEject(ejector.Eject))
		require.Nil(t, err)

		var (
			maxHeader flow.Header
			maxSeal   *flow.Seal
		)

		// add some seals to the pool to fill it up
		for i := 0; i < N+1; i++ {
			header := unittest.BlockHeaderFixture()
			err := headers.Store(&header)
			require.Nil(t, err)

			seal := unittest.SealFixture()
			seal.BlockID = header.ID()
			ok := pool.Add(seal)
			assert.True(t, ok)

			if header.Height >= maxHeader.Height {
				maxHeader = header
				maxSeal = seal
			}
		}

		// the max seal should have been evicted
		assert.False(t, pool.Has(maxSeal.ID()))
		assert.Equal(t, uint(N), pool.Size())
		t.Log(pool.Limit())
	})

}
