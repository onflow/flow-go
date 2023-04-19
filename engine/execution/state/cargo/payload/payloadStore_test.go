package payload_test

import (
	"testing"

	"github.com/onflow/flow-go/engine/execution/state/cargo/payload"
	"github.com/onflow/flow-go/engine/execution/state/cargo/storage"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
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

		storage := storage.NewInMemoryStorage(10, genesis, data)
		pstore, err := payload.NewPayloadStore(storage)
		require.NoError(t, err)

		val, err := pstore.Get(genesis.Height, genesis.ID(), key)
		require.NoError(t, err)
		require.Equal(t, initValue, val)

		for i, header := range headers {
			delta := map[flow.RegisterID]flow.RegisterValue{
				key: []byte{byte(int8(i))},
			}
			err = pstore.BlockExecuted(header, delta)
			require.NoError(t, err)
		}

		// check without commit
		for i, header := range headers {
			val, err := pstore.Get(header.Height, header.ID(), key)
			require.NoError(t, err)
			require.Equal(t, i, int(val[0]))
		}

		// commit all
		for _, header := range headers {
			found, err := pstore.BlockFinalized(header.ID(), header)
			require.True(t, found)
			require.NoError(t, err)
		}

		// check the values after the commit
		for i, header := range headers {
			val, err := pstore.Get(header.Height, header.ID(), key)
			require.NoError(t, err)
			require.Equal(t, i, int(val[0]))
		}

	})

	t.Run("cleanup forks", func(t *testing.T) {
		headers := unittest.BlockHeaderFixtures(10)
		genesis, mainChain := headers[0], headers[1:]

		storage := storage.NewInMemoryStorage(10, genesis, nil)
		pstore, err := payload.NewPayloadStore(storage)
		require.NoError(t, err)

		for i, header := range mainChain {
			err = pstore.BlockExecuted(header, map[flow.RegisterID]flow.RegisterValue{
				key: []byte{byte(int8(i))},
			})
			require.NoError(t, err)
		}

		fork1 := unittest.BlockHeaderWithParentFixture(genesis)
		err = pstore.BlockExecuted(fork1, map[flow.RegisterID]flow.RegisterValue{
			key: []byte{byte(int8(11))},
		})
		require.NoError(t, err)

		val, err := pstore.Get(fork1.Height, fork1.ID(), key)
		require.NoError(t, err)
		require.Equal(t, 11, int(val[0]))

		fork11 := unittest.BlockHeaderWithParentFixture(fork1)
		err = pstore.BlockExecuted(fork11, map[flow.RegisterID]flow.RegisterValue{
			key: []byte{byte(int8(12))},
		})
		require.NoError(t, err)

		val, err = pstore.Get(fork11.Height, fork11.ID(), key)
		require.NoError(t, err)
		require.Equal(t, 12, int(val[0]))

		fork2 := unittest.BlockHeaderWithParentFixture(genesis)
		err = pstore.BlockExecuted(fork2, map[flow.RegisterID]flow.RegisterValue{
			key: []byte{byte(int8(13))},
		})
		require.NoError(t, err)

		val, err = pstore.Get(fork2.Height, fork2.ID(), key)
		require.NoError(t, err)

		fork22 := unittest.BlockHeaderWithParentFixture(fork2)
		err = pstore.BlockExecuted(fork22, map[flow.RegisterID]flow.RegisterValue{
			key: []byte{byte(int8(14))},
		})
		require.NoError(t, err)

		val, err = pstore.Get(fork22.Height, fork22.ID(), key)
		require.NoError(t, err)

		found, err := pstore.BlockFinalized(mainChain[0].ID(), mainChain[0])
		require.True(t, found)
		require.NoError(t, err)

		// prunned ones should not return results
		val, err = pstore.Get(fork1.Height, fork1.ID(), key)
		require.Error(t, err)

		val, err = pstore.Get(fork2.Height, fork2.ID(), key)
		require.Error(t, err)

		val, err = pstore.Get(fork11.Height, fork11.ID(), key)
		require.Error(t, err)

		val, err = pstore.Get(fork22.Height, fork22.ID(), key)
		require.Error(t, err)
	})

}
