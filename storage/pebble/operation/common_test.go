package operation

import (
	"bytes"
	"encoding/hex"
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

func TestGetStartEndKeys(t *testing.T) {
	tests := []struct {
		prefix        []byte
		expectedStart []byte
		expectedEnd   []byte
	}{
		{[]byte("a"), []byte("a"), []byte("b")},
		{[]byte("abc"), []byte("abc"), []byte("abd")},
		{[]byte("prefix"), []byte("prefix"), []byte("prefiy")},
	}

	for _, test := range tests {
		start, end := getStartEndKeys(test.prefix)
		if !bytes.Equal(start, test.expectedStart) {
			t.Errorf("getStartEndKeys(%q) start = %q; want %q", test.prefix, start, test.expectedStart)
		}
		if !bytes.Equal(end, test.expectedEnd) {
			t.Errorf("getStartEndKeys(%q) end = %q; want %q", test.prefix, end, test.expectedEnd)
		}
	}
}

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

func TestIterate(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		keys := [][]byte{{0x00}, {0x12}, {0xf0}, {0xff}}
		vals := []bool{false, false, true, true}
		expected := []bool{false, true}

		require.NoError(t, WithReaderBatchWriter(db, func(tx storage.PebbleReaderBatchWriter) error {
			_, w := tx.ReaderWriter()
			for i, key := range keys {
				enc, err := msgpack.Marshal(vals[i])
				if err != nil {
					return err
				}

				err = w.Set(key, enc, nil)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		actual := make([]bool, 0, len(keys))
		iterationFunc := func() (checkFunc, createFunc, handleFunc) {
			check := func(key []byte) bool {
				return !bytes.Equal(key, []byte{0x12})
			}
			var val bool
			create := func() interface{} {
				return &val
			}
			handle := func() error {
				actual = append(actual, val)
				return nil
			}
			return check, create, handle
		}

		err := iterate(keys[0], keys[2], iterationFunc, true)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestTraverse(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		keys := [][]byte{{0x42, 0x00}, {0xff}, {0x42, 0x56}, {0x00}, {0x42, 0xff}}
		vals := []bool{false, false, true, false, true}
		expected := []bool{false, true}

		require.NoError(t, WithReaderBatchWriter(db, func(tx storage.PebbleReaderBatchWriter) error {
			_, w := tx.ReaderWriter()
			for i, key := range keys {
				enc, err := msgpack.Marshal(vals[i])
				require.NoError(t, err)
				err = w.Set(key, enc, nil)
				require.NoError(t, err)
			}
			return nil
		}))

		actual := make([]bool, 0, len(keys))
		iterationFunc := func() (checkFunc, createFunc, handleFunc) {
			check := func(key []byte) bool {
				return !bytes.Equal(key, []byte{0x42, 0x56})
			}
			var val bool
			create := func() interface{} {
				return &val
			}
			handle := func() error {
				actual = append(actual, val)
				return nil
			}
			return check, create, handle
		}

		err := traverse([]byte{0x42}, iterationFunc)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
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

func TestRemoveByPrefix(t *testing.T) {
	t.Run("should no-op when removing non-existing value", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			e := Entity{ID: 1337}
			key := []byte{0x01, 0x02, 0x03}
			val, _ := msgpack.Marshal(e)

			err := db.Set(key, val, nil)
			assert.NoError(t, err)

			nonexistantKey := append(key, 0x01)
			err = removeByPrefix(nonexistantKey)(db)
			assert.NoError(t, err)

			var act Entity
			err = retrieve(key, &act)(db)
			require.NoError(t, err)

			assert.Equal(t, e, act)
		})
	})

	t.Run("should be able to remove", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			e := Entity{ID: 1337}
			key := []byte{0x01, 0x02, 0x03}
			val, _ := msgpack.Marshal(e)

			err := db.Set(key, val, nil)
			assert.NoError(t, err)

			prefix := []byte{0x01, 0x02}
			err = removeByPrefix(prefix)(db)
			assert.NoError(t, err)

			_, _, err = db.Get(key)
			assert.Error(t, err)
			assert.ErrorIs(t, convertNotFoundError(err), storage.ErrNotFound)
		})
	})

	t.Run("should be able to remove by key", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			e := Entity{ID: 1337}
			key := []byte{0x01, 0x02, 0x03}
			val, _ := msgpack.Marshal(e)

			err := db.Set(key, val, nil)
			assert.NoError(t, err)

			err = removeByPrefix(key)(db)
			assert.NoError(t, err)

			_, _, err = db.Get(key)
			assert.Error(t, err)
			assert.ErrorIs(t, convertNotFoundError(err), storage.ErrNotFound)
		})
	})
}

func TestIterateBoundaries(t *testing.T) {

	// create range of keys covering all boundaries around our start/end values
	start := []byte{0x10}
	end := []byte{0x20}
	keys := [][]byte{
		// before start -> not included in range
		{0x09, 0xff},
		// shares prefix with start -> included in range
		{0x10},
		{0x10, 0x00},
		{0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x10, 0xff},
		// prefix with a shared
		{0x10,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		},
		{0x10,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00,
		},
		{0x10,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff,
		},
		// prefix between start and end -> included in range
		{0x11, 0x00},
		{0x19, 0xff},
		// shares prefix with end -> included in range
		{0x20, 0x00},
		{0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x20, 0xff},
		{0x20,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		},
		{0x20,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00,
		},
		{0x20,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff,
		},
		// after end -> not included in range
		{0x21},
		{0x21, 0x00},
	}

	// keys within the expected range
	keysInRange := make([]string, 0)

	firstNToExclude := 1
	lastNToExclude := 2
	for i := firstNToExclude; i < len(keys)-lastNToExclude; i++ {
		key := keys[i]
		keysInRange = append(keysInRange, hex.EncodeToString(key))
	}

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		// insert the keys into the database
		require.NoError(t, WithReaderBatchWriter(db, func(tx storage.PebbleReaderBatchWriter) error {
			_, w := tx.ReaderWriter()
			for _, key := range keys {
				err := w.Set(key, []byte{0x00}, nil)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		// define iteration function that simply appends all traversed keys
		found := make([]string, 0)

		iteration := func() (checkFunc, createFunc, handleFunc) {
			check := func(key []byte) bool {
				found = append(found, hex.EncodeToString(key))
				return false
			}
			create := func() interface{} {
				return nil
			}
			handle := func() error {
				return fmt.Errorf("shouldn't handle anything")
			}
			return check, create, handle
		}

		// iterate forward and check boundaries are included correctly
		err := iterate(start, end, iteration, false)(db)
		for i, f := range found {
			t.Logf("forward %d: %x", i, f)
		}
		require.NoError(t, err, "should iterate forward without error")
		assert.ElementsMatch(t, keysInRange, found, "forward iteration should go over correct keys")

		// iterate backward and check boundaries are not supported
	})
}

// to test the case where the end key is 0xffff..ff
func TestIterateBoundariesOverflow(t *testing.T) {

	// create range of keys covering all boundaries around our start/end values
	start := []byte{0x10}
	end := []byte{0xff}
	keys := [][]byte{
		// before start -> not included in range
		{0x09, 0xff},
		// shares prefix with start -> included in range
		{0x10},
		{0x10, 0x00},
		{0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x10, 0xff},
		// prefix with a shared
		{0x10,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		},
		{0x10,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff,
		},
		// prefix between start and end -> included in range
		{0x11, 0x00},
		{0x19, 0xff},
		// shares prefix with end -> included in range
		{0xff, 0x00},
		{0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0xff, 0xff},
		{0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		},
	}

	// keys within the expected range
	keysInRange := make([]string, 0)
	firstNToExclude := 1
	lastNToExclude := 0
	for i := firstNToExclude; i < len(keys)-lastNToExclude; i++ {
		key := keys[i]
		keysInRange = append(keysInRange, hex.EncodeToString(key))
	}

	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		// insert the keys into the database
		require.NoError(t, WithReaderBatchWriter(db, func(tx storage.PebbleReaderBatchWriter) error {
			_, w := tx.ReaderWriter()
			for _, key := range keys {
				err := w.Set(key, []byte{0x00}, nil)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		// define iteration function that simply appends all traversed keys
		found := make([]string, 0)

		iteration := func() (checkFunc, createFunc, handleFunc) {
			check := func(key []byte) bool {
				found = append(found, hex.EncodeToString(key))
				return false
			}
			create := func() interface{} {
				return nil
			}
			handle := func() error {
				return fmt.Errorf("shouldn't handle anything")
			}
			return check, create, handle
		}

		// iterate forward and check boundaries are included correctly
		err := iterate(start, end, iteration, false)(db)
		for i, f := range found {
			t.Logf("forward %d: %x", i, f)
		}
		require.NoError(t, err, "should iterate forward without error")
		assert.ElementsMatch(t, keysInRange, found, "forward iteration should go over correct keys")

		// iterate backward and check boundaries are not supported
	})
}

func TestFindHighestAtOrBelow(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		prefix := []byte("test_prefix")

		type Entity struct {
			Value uint64
		}

		entity1 := Entity{Value: 41}
		entity2 := Entity{Value: 42}
		entity3 := Entity{Value: 43}

		require.NoError(t, WithReaderBatchWriter(db, func(tx storage.PebbleReaderBatchWriter) error {
			_, w := tx.ReaderWriter()
			key := append(prefix, b(uint64(15))...)
			val, err := msgpack.Marshal(entity3)
			if err != nil {
				return err
			}
			err = w.Set(key, val, nil)
			if err != nil {
				return err
			}

			key = append(prefix, b(uint64(5))...)
			val, err = msgpack.Marshal(entity1)
			if err != nil {
				return err
			}
			err = w.Set(key, val, nil)
			if err != nil {
				return err
			}

			key = append(prefix, b(uint64(10))...)
			val, err = msgpack.Marshal(entity2)
			if err != nil {
				return err
			}
			err = w.Set(key, val, nil)
			if err != nil {
				return err
			}
			return nil
		}))

		var entity Entity

		t.Run("target height exists", func(t *testing.T) {
			err := findHighestAtOrBelow(
				prefix,
				10,
				&entity)(db)
			require.NoError(t, err)
			require.Equal(t, uint64(42), entity.Value)
		})

		t.Run("target height above", func(t *testing.T) {
			err := findHighestAtOrBelow(
				prefix,
				11,
				&entity)(db)
			require.NoError(t, err)
			require.Equal(t, uint64(42), entity.Value)
		})

		t.Run("target height above highest", func(t *testing.T) {
			err := findHighestAtOrBelow(
				prefix,
				20,
				&entity)(db)
			require.NoError(t, err)
			require.Equal(t, uint64(43), entity.Value)
		})

		t.Run("target height below lowest", func(t *testing.T) {
			err := findHighestAtOrBelow(
				prefix,
				4,
				&entity)(db)
			require.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("empty prefix", func(t *testing.T) {
			err := findHighestAtOrBelow(
				[]byte{},
				5,
				&entity)(db)
			require.Error(t, err)
			require.Contains(t, err.Error(), "prefix must not be empty")
		})
	})
}
