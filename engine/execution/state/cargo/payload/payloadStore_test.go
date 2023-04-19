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
			err = pstore.Update(header, delta)
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
			found, err := pstore.Commit(header.ID(), header)
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

	// TODO test compliance
	// TODO test forks
}
