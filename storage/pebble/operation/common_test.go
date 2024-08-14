package operation

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

var upsert = insert
var update = insert

type Entity struct {
	ID uint64
}

type UnencodeableEntity Entity

var errCantEncode = fmt.Errorf("encoding not supported")
var errCantDecode = fmt.Errorf("decoding not supported")

func (a UnencodeableEntity) MarshalJSON() ([]byte, error) {
	return nil, errCantEncode
}

func (a *UnencodeableEntity) UnmarshalJSON(b []byte) error {
	return errCantDecode
}

func (a UnencodeableEntity) MarshalMsgpack() ([]byte, error) {
	return nil, errCantEncode
}

func (a UnencodeableEntity) UnmarshalMsgpack(b []byte) error {
	return errCantDecode
}

func TestInsertValid(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := msgpack.Marshal(e)

		err := insert(key, e)(db)
		require.NoError(t, err)

		var act []byte
		act, closer, err := db.Get(key)
		require.NoError(t, err)
		defer require.NoError(t, closer.Close())

		assert.Equal(t, val, act)
	})
}

func TestInsertDuplicate(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}

		// persist first time
		err := insert(key, e)(db)
		require.NoError(t, err)

		e2 := Entity{ID: 1338}
		val, _ := msgpack.Marshal(e2)

		// persist again will override
		err = insert(key, e2)(db)
		require.NoError(t, err)

		// ensure old value did not insert
		act, closer, err := db.Get(key)
		require.NoError(t, err)
		defer require.NoError(t, closer.Close())

		assert.Equal(t, val, act)
	})
}

func TestInsertEncodingError(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}

		err := insert(key, UnencodeableEntity(e))(db)
		require.Error(t, err, errCantEncode)
		require.NotErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestUpdateValid(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := msgpack.Marshal(e)

		err := db.Set(key, []byte{}, nil)
		require.NoError(t, err)

		err = insert(key, e)(db)
		require.NoError(t, err)

		act, closer, err := db.Get(key)
		require.NoError(t, err)
		defer require.NoError(t, closer.Close())

		assert.Equal(t, val, act)
	})
}

func TestUpdateEncodingError(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := msgpack.Marshal(e)

		err := db.Set(key, val, nil)
		require.NoError(t, err)

		err = insert(key, UnencodeableEntity(e))(db)
		require.Error(t, err)
		require.NotErrorIs(t, err, storage.ErrNotFound)

		// ensure value did not change
		act, closer, err := db.Get(key)
		require.NoError(t, err)
		defer require.NoError(t, closer.Close())

		assert.Equal(t, val, act)
	})
}

func TestUpsertEntry(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := msgpack.Marshal(e)

		// first upsert an non-existed entry
		err := insert(key, e)(db)
		require.NoError(t, err)

		act, closer, err := db.Get(key)
		require.NoError(t, err)
		defer require.NoError(t, closer.Close())
		require.NoError(t, err)

		assert.Equal(t, val, act)

		// next upsert the value with the same key
		newEntity := Entity{ID: 1338}
		newVal, _ := msgpack.Marshal(newEntity)
		err = upsert(key, newEntity)(db)
		require.NoError(t, err)

		act, closer, err = db.Get(key)
		require.NoError(t, err)
		defer require.NoError(t, closer.Close())

		assert.Equal(t, newVal, act)
	})
}

func TestRetrieveValid(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := msgpack.Marshal(e)

		err := db.Set(key, val, nil)
		require.NoError(t, err)

		var act Entity
		err = retrieve(key, &act)(db)
		require.NoError(t, err)

		assert.Equal(t, e, act)
	})
}

func TestRetrieveMissing(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		key := []byte{0x01, 0x02, 0x03}

		var act Entity
		err := retrieve(key, &act)(db)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestRetrieveUnencodeable(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := msgpack.Marshal(e)

		err := db.Set(key, val, nil)
		require.NoError(t, err)

		var act *UnencodeableEntity
		err = retrieve(key, &act)(db)
		require.Error(t, err)
		require.NotErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestExists verifies that `exists` returns correct results in different scenarios.
func TestExists(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		t.Run("non-existent key", func(t *testing.T) {
			key := unittest.RandomBytes(32)
			var _exists bool
			err := exists(key, &_exists)(db)
			require.NoError(t, err)
			assert.False(t, _exists)
		})

		t.Run("existent key", func(t *testing.T) {
			key := unittest.RandomBytes(32)
			err := insert(key, unittest.RandomBytes(256))(db)
			require.NoError(t, err)

			var _exists bool
			err = exists(key, &_exists)(db)
			require.NoError(t, err)
			assert.True(t, _exists)
		})

		t.Run("removed key", func(t *testing.T) {
			key := unittest.RandomBytes(32)
			// insert, then remove the key
			err := insert(key, unittest.RandomBytes(256))(db)
			require.NoError(t, err)
			err = remove(key)(db)
			require.NoError(t, err)

			var _exists bool
			err = exists(key, &_exists)(db)
			require.NoError(t, err)
			assert.False(t, _exists)
		})
	})
}

func TestRemove(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := msgpack.Marshal(e)

		err := db.Set(key, val, nil)
		require.NoError(t, err)

		t.Run("should be able to remove", func(t *testing.T) {
			err := remove(key)(db)
			assert.NoError(t, err)

			_, _, err = db.Get(key)
			assert.ErrorIs(t, convertNotFoundError(err), storage.ErrNotFound)
		})

		t.Run("should ok when removing non-existing value", func(t *testing.T) {
			nonexistantKey := append(key, 0x01)
			err := remove(nonexistantKey)(db)
			assert.NoError(t, err)
		})
	})
}
