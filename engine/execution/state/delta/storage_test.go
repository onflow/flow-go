package delta_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBadgerStorage(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store, err := delta.NewBadgerStore(db)
		require.NoError(t, err)

		h, err := store.BlockHeight()
		require.NoError(t, err)
		require.Equal(t, uint64(0), h)

		key1 := flow.RegisterID{
			Owner: "owner1",
			Key:   "ABC",
		}
		value1 := []byte("123")

		key2 := flow.RegisterID{
			Owner: "owner1",
			Key:   "DEF",
		}
		value2 := []byte("456")

		data := []flow.RegisterEntry{
			{
				Key:   key1,
				Value: value1,
			},
			{
				Key:   key2,
				Value: value2,
			},
		}
		err = store.Bootstrap(11, data)
		require.NoError(t, err)

		h, err = store.BlockHeight()
		require.NoError(t, err)
		require.Equal(t, uint64(11), h)

		retVal, found, err := store.GetRegister(key1)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value1, retVal)

		retVal, found, err = store.GetRegister(key2)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value2, retVal)

		key3 := flow.RegisterID{
			Owner: "owner2",
			Key:   "ABC",
		}
		value3 := []byte("789")

		retVal, found, err = store.GetRegister(key3)
		require.NoError(t, err)
		require.False(t, found)

		del := delta.NewDelta()
		del.Set(key3.Owner, key3.Key, value3)
		err = store.CommitBlockDelta(12, del)
		require.NoError(t, err)

		retVal, found, err = store.GetRegister(key3)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value3, retVal)

		retVal, found, err = store.GetRegister(key2)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value2, retVal)

		h, err = store.BlockHeight()
		require.NoError(t, err)
		require.Equal(t, uint64(12), h)
	})
}
