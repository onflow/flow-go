package storage_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/cargo/storage"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInMemStorage(t *testing.T) {

	t.Run("commit checks", func(t *testing.T) {
		var err error
		capacity := 10
		headers := unittest.BlockHeaderFixtures(13)
		genesis, batch1, _, batch3 := headers[0], headers[1:5], headers[5:9], headers[9:]

		store := storage.NewInMemoryStorage(capacity, genesis, nil)

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

			ret, err := store.RegisterValueAt(header.Height, key)
			require.NoError(t, err)
			require.Equal(t, randValue, ret)
		}

		// it should fail because of compliance
		for _, header := range batch3 {
			err = store.CommitBlock(header, nil)
			require.Error(t, err)

			ret, err := store.RegisterValueAt(header.Height, key)
			require.Error(t, err)
			require.Nil(t, ret)
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

		store := storage.NewInMemoryStorage(capacity, genesis, data)

		val, err := store.RegisterValueAt(genesis.Height-1, key)
		require.Error(t, err)
		require.Nil(t, val)

		// add the first batch of headers
		for _, header := range batch1 {
			err = store.CommitBlock(header, data)
			require.NoError(t, err)

			ret, err := store.RegisterValueAt(header.Height, key)
			require.NoError(t, err)
			require.Equal(t, value, ret)
		}

		for _, header := range batch2 {
			err = store.CommitBlock(header, data)
			require.NoError(t, err)

			ret, err := store.RegisterValueAt(header.Height, key)
			require.NoError(t, err)
			require.Equal(t, value, ret)
		}

		// now batch one should not be available
		for _, header := range batch1 {
			ret, err := store.RegisterValueAt(header.Height, key)
			require.Error(t, err)
			require.Nil(t, ret)
		}
	})

	t.Run("test historic value return ", func(t *testing.T) {
		var err error
		capacity := 10
		headers := unittest.BlockHeaderFixtures(1 + 10)
		genesis, batch1 := headers[0], headers[1:10]

		key := flow.RegisterID{Key: "key", Owner: "owner"}

		store := storage.NewInMemoryStorage(capacity, genesis, nil)

		val, err := store.RegisterValueAt(genesis.Height, key)
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
			val, err = store.RegisterValueAt(header.Height, key)
			require.NoError(t, err)
			require.Equal(t, i, int(val[0]))
		}
	})
}
