package cargo_test

import (
	"testing"

	"github.com/onflow/flow-go/engine/execution/state/cargo"
	"github.com/onflow/flow-go/engine/execution/state/cargo/storage"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestCargo(t *testing.T) {

	t.Run("finalization falling behind", func(t *testing.T) {

		blocks := unittest.BlockHeaderFixtures(10)
		genesis, headers := blocks[0], blocks[1:]

		store := storage.NewInMemoryStorage(100, genesis, nil)
		c, err := cargo.NewCargo(store, 100, genesis)

		require.NoError(t, err)

		for _, h := range headers {
			err = c.BlockExecuted(h, nil)
			require.NoError(t, err)
		}

		for _, h := range headers {
			err = c.BlockFinalized(h)
			require.NoError(t, err)

			ret, err := store.LastCommittedBlock()
			require.NoError(t, err)
			require.Equal(t, h, ret)
		}

	})

	// TODO concurrent test
}
