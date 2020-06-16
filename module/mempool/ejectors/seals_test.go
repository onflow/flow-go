package ejectors_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/ejectors"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestLatestSealEjector(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		const N = 10

		mm := metrics.NewNoopCollector()
		headers := storage.NewHeaders(mm, db)

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
			seal := unittest.BlockSealFixture()
			seal.BlockID = header.ID()
			ok := pool.Add(seal)
			assert.True(t, ok)

			err := headers.Store(&header)
			require.Nil(t, err)

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
