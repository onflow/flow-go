package forest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage/ephemeral"
	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage/forest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPayloadStore(t *testing.T) {
	key := flow.RegisterID{Owner: "A", Key: "B"}
	initValue := []byte("rand value")

	t.Run("happy path functionality", func(t *testing.T) {
		headers := unittest.BlockHeaderFixtures(10)
		genesis, headers := headers[0], headers[1:]

		data := map[flow.RegisterID]flow.RegisterValue{
			key: initValue,
		}

		storage := ephemeral.NewStorage(10, genesis, data)
		pstore, err := forest.NewStorage(storage)
		require.NoError(t, err)

		view, err := pstore.BlockView(genesis.Height, genesis.ID())
		require.NoError(t, err)

		val, err := view.Get(key)
		require.NoError(t, err)
		require.Equal(t, initValue, val)

		for i, header := range headers {
			delta := map[flow.RegisterID]flow.RegisterValue{
				key: []byte{byte(int8(i))},
			}
			err = pstore.CommitBlock(header, delta)
			require.NoError(t, err)
		}

		// check without commit
		for i, header := range headers {
			view, err := pstore.BlockView(header.Height, header.ID())
			require.NoError(t, err)

			val, err := view.Get(key)
			require.NoError(t, err)
			require.Equal(t, i, int(val[0]))
		}

		// commit all
		for _, header := range headers {
			found, err := pstore.BlockFinalized(header)
			require.True(t, found)
			require.NoError(t, err)
		}

		// check the values after the commit
		for i, header := range headers {
			view, err := pstore.BlockView(header.Height, header.ID())
			require.NoError(t, err)

			val, err := view.Get(key)
			require.NoError(t, err)
			require.Equal(t, i, int(val[0]))
		}
	})

	t.Run("test fork cleanup", func(t *testing.T) {
		headers := unittest.BlockHeaderFixtures(10)
		genesis, mainChain := headers[0], headers[1:]

		storage := ephemeral.NewStorage(10, genesis, nil)
		pstore, err := forest.NewStorage(storage)
		require.NoError(t, err)

		for i, header := range mainChain {
			err = pstore.CommitBlock(header, map[flow.RegisterID]flow.RegisterValue{
				key: []byte{byte(int8(i))},
			})
			require.NoError(t, err)
		}

		fork1 := unittest.BlockHeaderWithParentFixture(genesis)
		err = pstore.CommitBlock(fork1, map[flow.RegisterID]flow.RegisterValue{
			key: []byte{byte(int8(11))},
		})
		require.NoError(t, err)

		view, err := pstore.BlockView(fork1.Height, fork1.ID())
		require.NoError(t, err)

		val, err := view.Get(key)
		require.NoError(t, err)
		require.Equal(t, 11, int(val[0]))

		fork11 := unittest.BlockHeaderWithParentFixture(fork1)
		err = pstore.CommitBlock(fork11, map[flow.RegisterID]flow.RegisterValue{
			key: []byte{byte(int8(12))},
		})
		require.NoError(t, err)

		view, err = pstore.BlockView(fork11.Height, fork11.ID())
		require.NoError(t, err)

		val, err = view.Get(key)
		require.NoError(t, err)
		require.Equal(t, 12, int(val[0]))

		fork2 := unittest.BlockHeaderWithParentFixture(genesis)
		err = pstore.CommitBlock(fork2, map[flow.RegisterID]flow.RegisterValue{
			key: []byte{byte(int8(13))},
		})
		require.NoError(t, err)

		view, err = pstore.BlockView(fork2.Height, fork2.ID())
		require.NoError(t, err)

		val, err = view.Get(key)
		require.NoError(t, err)

		fork22 := unittest.BlockHeaderWithParentFixture(fork2)
		err = pstore.CommitBlock(fork22, map[flow.RegisterID]flow.RegisterValue{
			key: []byte{byte(int8(14))},
		})
		require.NoError(t, err)

		view, err = pstore.BlockView(fork22.Height, fork22.ID())
		require.NoError(t, err)

		val, err = view.Get(key)
		require.NoError(t, err)

		// first block is finalized
		found, err := pstore.BlockFinalized(mainChain[0])
		require.True(t, found)
		require.NoError(t, err)

		// prunned ones should not return block view
		view, err = pstore.BlockView(fork1.Height, fork1.ID())
		require.Error(t, err)

		view, err = pstore.BlockView(fork2.Height, fork2.ID())
		require.Error(t, err)

		view, err = pstore.BlockView(fork11.Height, fork11.ID())
		require.Error(t, err)

		view, err = pstore.BlockView(fork22.Height, fork22.ID())
		require.Error(t, err)
	})

}
