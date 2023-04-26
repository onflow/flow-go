package ephemeral_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage/ephemeral"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInMemoryStorage(t *testing.T) {

	t.Run("commit checks", func(t *testing.T) {
		var err error
		capacity := 10
		headers := unittest.BlockHeaderFixtures(13)
		genesis, batch1, _, batch3 := headers[0], headers[1:5], headers[5:9], headers[9:]

		store := ephemeral.NewStorage(capacity, genesis, nil)

		blk, err := store.LastCommittedBlock()
		require.NoError(t, err)
		require.Equal(t, blk, genesis)

		key := flow.RegisterID{Key: "key", Owner: "owner"}
		// add the first batch of headers
		for _, header := range batch1 {
			randValue := make([]byte, 10)
			_, err := rand.Read(randValue)
			require.NoError(t, err)

			update := map[flow.RegisterID]flow.RegisterValue{
				key: randValue,
			}

			err = store.CommitBlock(header, update)
			require.NoError(t, err)

			blk, err := store.LastCommittedBlock()
			require.NoError(t, err)
			require.Equal(t, blk, header)

			view, err := store.BlockView(header.Height, header.ID())
			require.NoError(t, err)

			ret, err := view.Get(key)
			require.NoError(t, err)
			require.Equal(t, randValue, ret)
		}

		// it should fail because of compliance
		for _, header := range batch3 {
			err = store.CommitBlock(header, nil)
			require.Error(t, err)

			view, err := store.BlockView(header.Height, header.ID())
			require.Error(t, err)
			require.Nil(t, view)
		}
	})

	t.Run("test min available height", func(t *testing.T) {
		var err error
		capacity := 4
		headers := unittest.BlockHeaderFixtures(9)
		genesis, batch1, batch2 := headers[0], headers[1:5], headers[5:9]

		key := flow.RegisterID{Key: "key", Owner: "owner"}
		value := []byte("random")
		data := map[flow.RegisterID]flow.RegisterValue{
			key: value,
		}

		store := ephemeral.NewStorage(capacity, genesis, data)

		view, err := store.BlockView(genesis.Height-1, flow.ZeroID)
		require.Error(t, err)
		require.Nil(t, view)

		// add the first batch of headers
		for _, header := range batch1 {
			err = store.CommitBlock(header, data)
			require.NoError(t, err)

			view, err := store.BlockView(header.Height, header.ID())
			require.NoError(t, err)

			ret, err := view.Get(key)
			require.NoError(t, err)
			require.Equal(t, value, ret)
		}

		// hold on to a view from batch one to test
		lastCommittedHeader := batch1[len(batch1)-1]
		viewToBeStale, err := store.BlockView(lastCommittedHeader.Height, lastCommittedHeader.ID())
		require.NoError(t, err)

		ret, err := viewToBeStale.Get(key)
		require.NoError(t, err)
		require.Equal(t, value, ret)

		// start batch2
		for _, header := range batch2 {
			err = store.CommitBlock(header, data)
			require.NoError(t, err)

			view, err := store.BlockView(header.Height, header.ID())
			require.NoError(t, err)

			ret, err := view.Get(key)
			require.NoError(t, err)
			require.Equal(t, value, ret)
		}

		// now batch one should not be available
		for _, header := range batch1 {
			view, err := store.BlockView(header.Height, header.ID())
			require.Error(t, err)
			require.Nil(t, view)
		}

		// check staleed view
		ret, err = viewToBeStale.Get(key)
		require.Error(t, err)
	})

	t.Run("test historic value return", func(t *testing.T) {
		var err error
		capacity := 10
		headers := unittest.BlockHeaderFixtures(1 + 10)
		genesis, batch1 := headers[0], headers[1:10]

		key := flow.RegisterID{Key: "key", Owner: "owner"}

		store := ephemeral.NewStorage(capacity, genesis, nil)

		view, err := store.BlockView(genesis.Height, genesis.ID())
		require.NoError(t, err)

		val, err := view.Get(key)
		require.NoError(t, err)
		require.Nil(t, val)

		for i, header := range batch1 {
			data := map[flow.RegisterID]flow.RegisterValue{
				key: []byte{byte(int8(i))},
			}
			err = store.CommitBlock(header, data)
			require.NoError(t, err)
		}

		for i, header := range batch1 {
			view, err := store.BlockView(header.Height, header.ID())
			require.NoError(t, err)

			val, err := view.Get(key)
			require.NoError(t, err)
			require.Equal(t, i, int(val[0]))
		}
	})

	t.Run("test update gaps", func(t *testing.T) {
		var err error
		capacity := 10
		headers := unittest.BlockHeaderFixtures(1 + 10)
		genesis, batch1 := headers[0], headers[1:10]

		key := flow.RegisterID{Key: "key", Owner: "owner"}

		store := ephemeral.NewStorage(capacity,
			genesis,
			map[flow.RegisterID]flow.RegisterValue{
				key: []byte{byte(int8(0))},
			})

		view, err := store.BlockView(genesis.Height, genesis.ID())
		require.NoError(t, err)

		val, err := view.Get(key)
		require.NoError(t, err)
		require.Equal(t, 0, int(val[0]))

		for i, header := range batch1 {
			data := make(map[flow.RegisterID][]byte)
			if i%5 == 0 {
				data[key] = []byte{byte(int8(i))}
			}
			err = store.CommitBlock(header, data)
			require.NoError(t, err)
		}

		lastUpdatedValue := 0
		for i, header := range batch1 {
			if i%5 == 0 {
				lastUpdatedValue = i
			}

			view, err := store.BlockView(header.Height, header.ID())
			require.NoError(t, err)

			val, err = view.Get(key)
			require.NoError(t, err)
			require.Equal(t, lastUpdatedValue, int(val[0]))
		}
	})
}
