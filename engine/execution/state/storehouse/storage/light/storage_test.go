package light_test

import (
	"log"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage/light"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestStorage(t *testing.T) {

	unittest.RunWithTempDir(t, func(directory string) {
		headers := unittest.BlockHeaderFixtures(10)
		genesis, headers := headers[0], headers[1:]

		db, err := pebble.Open(directory, &pebble.Options{})
		if err != nil {
			log.Fatal(err)
		}

		store, err := light.NewStorage(db, genesis)
		require.NoError(t, err)

		ret, err := store.LastCommittedBlock()
		require.NoError(t, err)
		require.Equal(t, genesis, ret)

		key := flow.RegisterID{Owner: "A", Key: "B"}

		view, err := store.BlockView(genesis.Height, genesis.ID())
		require.NoError(t, err)

		val, err := view.Get(key)
		require.NoError(t, err)
		require.Nil(t, val)

		for i, header := range headers {
			data := map[flow.RegisterID]flow.RegisterValue{
				key: []byte{byte(int8(i))},
			}
			err = store.CommitBlock(header, data)
			require.NoError(t, err)

			// stale view
			_, err = view.Get(key)
			require.Error(t, err)

			view, err = store.BlockView(header.Height, header.ID())
			require.NoError(t, err)

			val, err := view.Get(key)
			require.NoError(t, err)
			require.Equal(t, i, int(val[0]))

			ret, err := store.LastCommittedBlock()
			require.NoError(t, err)
			require.Equal(t, header, ret)
		}
	})
}
