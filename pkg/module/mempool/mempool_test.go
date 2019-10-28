package mempool_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/pkg/model/collection"
	"github.com/dapperlabs/flow-go/pkg/module/mempool"
)

func TestHash(t *testing.T) {
	gc1 := &collection.GuaranteedCollection{
		Hash: []byte("DEAD"),
	}
	gc2 := &collection.GuaranteedCollection{
		Hash: []byte("BEEF"),
	}

	t.Run("insertion order should not impact the hash", func(t *testing.T) {
		// create the first mempool with gc1 and gc2
		pool1, err := mempool.New()
		require.Nil(t, err)
		pool1.Add(gc1)
		pool1.Add(gc2)

		// create the second mempool with gc2 and gc1
		pool2, err := mempool.New()
		require.Nil(t, err)
		pool2.Add(gc2)
		pool2.Add(gc1)

		// check pool1 and pool2 should have the same hash
		require.Equal(t, pool1.Hash(), pool2.Hash())
	})

	t.Run("having different items should produce different hash", func(t *testing.T) {
		// create the first mempool with only gc1
		pool1, err := mempool.New()
		require.Nil(t, err)
		pool1.Add(gc1)

		// create the second mempool with gc1 and gc2
		pool2, err := mempool.New()
		require.Nil(t, err)
		pool2.Add(gc1)
		pool1.Add(gc2)

		// check pool1 and pool2 should have the different hash
		require.NotEqual(t, pool1.Hash(), pool2.Hash())
	})

	t.Run("insert duplicated items should not impact hash", func(t *testing.T) {
		// create the first mempool with gc1 and gc2
		pool, err := mempool.New()
		require.Nil(t, err)
		pool.Add(gc1)
		pool.Add(gc2)

		hash1 := pool.Hash()

		// add gc2 again
		pool.Add(gc2)

		// check hash should not change
		require.Equal(t, hash1, pool.Hash())
	})
}
