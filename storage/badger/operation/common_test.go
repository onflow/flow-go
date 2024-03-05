// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

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
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := encodeEntity(e)

		err := db.Update(insert(key, e))
		require.NoError(t, err)

		var act []byte
		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.NoError(t, err)
			act, err = item.ValueCopy(nil)
			require.NoError(t, err)
			return nil
		})

		assert.Equal(t, val, act)
	})
}

func TestInsertDuplicate(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := encodeEntity(e)

		// persist first time
		err := db.Update(insert(key, e))
		require.NoError(t, err)

		e2 := Entity{ID: 1338}

		// persist again
		err = db.Update(insert(key, e2))
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)

		// ensure old value did not update
		var act []byte
		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.NoError(t, err)
			act, err = item.ValueCopy(nil)
			require.NoError(t, err)
			return nil
		})

		assert.Equal(t, val, act)
	})
}

func TestInsertEncodingError(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}

		err := db.Update(insert(key, UnencodeableEntity(e)))
		require.Error(t, err, errCantEncode)
		require.NotErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestUpdateValid(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := encodeEntity(e)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, []byte{})
			require.NoError(t, err)
			return nil
		})

		err := db.Update(update(key, e))
		require.NoError(t, err)

		var act []byte
		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.NoError(t, err)
			act, err = item.ValueCopy(nil)
			require.NoError(t, err)
			return nil
		})

		assert.Equal(t, val, act)
	})
}

func TestUpdateMissing(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}

		err := db.Update(update(key, e))
		require.ErrorIs(t, err, storage.ErrNotFound)

		// ensure nothing was written
		_ = db.View(func(tx *badger.Txn) error {
			_, err := tx.Get(key)
			require.Equal(t, badger.ErrKeyNotFound, err)
			return nil
		})
	})
}

func TestUpdateEncodingError(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := encodeEntity(e)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, val)
			require.NoError(t, err)
			return nil
		})

		err := db.Update(update(key, UnencodeableEntity(e)))
		require.Error(t, err)
		require.NotErrorIs(t, err, storage.ErrNotFound)

		// ensure value did not change
		var act []byte
		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.NoError(t, err)
			act, err = item.ValueCopy(nil)
			require.NoError(t, err)
			return nil
		})

		assert.Equal(t, val, act)
	})
}

func TestUpsertEntry(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := encodeEntity(e)

		// first upsert an non-existed entry
		err := db.Update(insert(key, e))
		require.NoError(t, err)

		var act []byte
		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.NoError(t, err)
			act, err = item.ValueCopy(nil)
			require.NoError(t, err)
			return nil
		})

		assert.Equal(t, val, act)

		// next upsert the value with the same key
		newEntity := Entity{ID: 1338}
		newVal, _ := encodeEntity(newEntity)
		err = db.Update(upsert(key, newEntity))
		require.NoError(t, err)

		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.NoError(t, err)
			act, err = item.ValueCopy(nil)
			require.NoError(t, err)
			return nil
		})

		assert.Equal(t, newVal, act)
	})
}

func TestRetrieveValid(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := encodeEntity(e)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, val)
			require.NoError(t, err)
			return nil
		})

		var act Entity
		err := db.View(retrieve(key, &act))
		require.NoError(t, err)

		assert.Equal(t, e, act)
	})
}

func TestRetrieveMissing(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		key := []byte{0x01, 0x02, 0x03}

		var act Entity
		err := db.View(retrieve(key, &act))
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestRetrieveUnencodeable(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := encodeEntity(e)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, val)
			require.NoError(t, err)
			return nil
		})

		var act *UnencodeableEntity
		err := db.View(retrieve(key, &act))
		require.Error(t, err)
		require.NotErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestExists verifies that `exists` returns correct results in different scenarios.
func TestExists(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("non-existent key", func(t *testing.T) {
			key := unittest.RandomBytes(32)
			var _exists bool
			err := db.View(exists(key, &_exists))
			require.NoError(t, err)
			assert.False(t, _exists)
		})

		t.Run("existent key", func(t *testing.T) {
			key := unittest.RandomBytes(32)
			err := db.Update(insert(key, unittest.RandomBytes(256)))
			require.NoError(t, err)

			var _exists bool
			err = db.View(exists(key, &_exists))
			require.NoError(t, err)
			assert.True(t, _exists)
		})

		t.Run("removed key", func(t *testing.T) {
			key := unittest.RandomBytes(32)
			// insert, then remove the key
			err := db.Update(insert(key, unittest.RandomBytes(256)))
			require.NoError(t, err)
			err = db.Update(remove(key))
			require.NoError(t, err)

			var _exists bool
			err = db.View(exists(key, &_exists))
			require.NoError(t, err)
			assert.False(t, _exists)
		})
	})
}

func TestLookup(t *testing.T) {
	expected := []flow.Identifier{
		{0x01},
		{0x02},
		{0x03},
		{0x04},
	}
	actual := []flow.Identifier{}

	iterationFunc := lookup(&actual)

	for _, e := range expected {
		checkFunc, createFunc, handleFunc := iterationFunc()
		assert.True(t, checkFunc([]byte{0x00}))
		target := createFunc()
		assert.IsType(t, &flow.Identifier{}, target)

		// set the value to target. Need to use reflection here since target is not strongly typed
		reflect.ValueOf(target).Elem().Set(reflect.ValueOf(e))

		assert.NoError(t, handleFunc())
	}

	assert.Equal(t, expected, actual)
}

func TestIterate(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		keys := [][]byte{{0x00}, {0x12}, {0xf0}, {0xff}}
		vals := []bool{false, false, true, true}
		expected := []bool{false, true}

		_ = db.Update(func(tx *badger.Txn) error {
			for i, key := range keys {
				enc, err := encodeEntity(vals[i])
				require.NoError(t, err)
				err = tx.Set(key, enc)
				require.NoError(t, err)
			}
			return nil
		})

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

		err := db.View(iterate(keys[0], keys[2], iterationFunc))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestTraverse(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		keys := [][]byte{{0x42, 0x00}, {0xff}, {0x42, 0x56}, {0x00}, {0x42, 0xff}}
		vals := []bool{false, false, true, false, true}
		expected := []bool{false, true}

		_ = db.Update(func(tx *badger.Txn) error {
			for i, key := range keys {
				enc, err := encodeEntity(vals[i])
				require.NoError(t, err)
				err = tx.Set(key, enc)
				require.NoError(t, err)
			}
			return nil
		})

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

		err := db.View(traverse([]byte{0x42}, iterationFunc))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestRemove(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := encodeEntity(e)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, val)
			require.NoError(t, err)
			return nil
		})

		t.Run("should be able to remove", func(t *testing.T) {
			_ = db.Update(func(txn *badger.Txn) error {
				err := remove(key)(txn)
				assert.NoError(t, err)

				_, err = txn.Get(key)
				assert.ErrorIs(t, err, badger.ErrKeyNotFound)

				return nil
			})
		})

		t.Run("should error when removing non-existing value", func(t *testing.T) {
			nonexistantKey := append(key, 0x01)
			_ = db.Update(func(txn *badger.Txn) error {
				err := remove(nonexistantKey)(txn)
				assert.ErrorIs(t, err, storage.ErrNotFound)
				assert.Error(t, err)
				return nil
			})
		})
	})
}

func TestRemoveByPrefix(t *testing.T) {
	t.Run("should no-op when removing non-existing value", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			e := Entity{ID: 1337}
			key := []byte{0x01, 0x02, 0x03}
			val, _ := encodeEntity(e)

			_ = db.Update(func(tx *badger.Txn) error {
				err := tx.Set(key, val)
				assert.NoError(t, err)
				return nil
			})

			nonexistantKey := append(key, 0x01)
			err := db.Update(removeByPrefix(nonexistantKey))
			assert.NoError(t, err)

			var act Entity
			err = db.View(retrieve(key, &act))
			require.NoError(t, err)

			assert.Equal(t, e, act)
		})
	})

	t.Run("should be able to remove", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			e := Entity{ID: 1337}
			key := []byte{0x01, 0x02, 0x03}
			val, _ := encodeEntity(e)

			_ = db.Update(func(tx *badger.Txn) error {
				err := tx.Set(key, val)
				assert.NoError(t, err)
				return nil
			})

			_ = db.Update(func(txn *badger.Txn) error {
				prefix := []byte{0x01, 0x02}
				err := removeByPrefix(prefix)(txn)
				assert.NoError(t, err)

				_, err = txn.Get(key)
				assert.Error(t, err)
				assert.IsType(t, badger.ErrKeyNotFound, err)

				return nil
			})
		})
	})

	t.Run("should be able to remove by key", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			e := Entity{ID: 1337}
			key := []byte{0x01, 0x02, 0x03}
			val, _ := encodeEntity(e)

			_ = db.Update(func(tx *badger.Txn) error {
				err := tx.Set(key, val)
				assert.NoError(t, err)
				return nil
			})

			_ = db.Update(func(txn *badger.Txn) error {
				err := removeByPrefix(key)(txn)
				assert.NoError(t, err)

				_, err = txn.Get(key)
				assert.Error(t, err)
				assert.IsType(t, badger.ErrKeyNotFound, err)

				return nil
			})
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
		{0x10, 0x00},
		{0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x10, 0xff},
		{0x10, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		// prefix between start and end -> included in range
		{0x11, 0x00},
		{0x19, 0xff},
		// shares prefix with end -> included in range
		{0x20, 0x00},
		{0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x20, 0xff},
		{0x20, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		// after end -> not included in range
		{0x21, 0x00},
	}

	// set the maximum current DB key range
	for _, key := range keys {
		if uint32(len(key)) > max {
			max = uint32(len(key))
		}
	}

	// keys within the expected range
	keysInRange := keys[1:11]

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		// insert the keys into the database
		_ = db.Update(func(tx *badger.Txn) error {
			for _, key := range keys {
				err := tx.Set(key, []byte{0x00})
				if err != nil {
					return err
				}
			}
			return nil
		})

		// define iteration function that simply appends all traversed keys
		var found [][]byte
		iteration := func() (checkFunc, createFunc, handleFunc) {
			check := func(key []byte) bool {
				found = append(found, key)
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
		found = nil
		err := db.View(iterate(start, end, iteration))
		for i, f := range found {
			t.Logf("forward %d: %x", i, f)
		}
		require.NoError(t, err, "should iterate forward without error")
		assert.ElementsMatch(t, keysInRange, found, "forward iteration should go over correct keys")

		// iterate backward and check boundaries are included correctly
		found = nil
		err = db.View(iterate(end, start, iteration))
		for i, f := range found {
			t.Logf("backward %d: %x", i, f)
		}
		require.NoError(t, err, "should iterate backward without error")
		assert.ElementsMatch(t, keysInRange, found, "backward iteration should go over correct keys")
	})
}

func TestFindHighestAtOrBelow(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		prefix := []byte("test_prefix")

		type Entity struct {
			Value uint64
		}

		entity1 := Entity{Value: 41}
		entity2 := Entity{Value: 42}
		entity3 := Entity{Value: 43}

		err := db.Update(func(tx *badger.Txn) error {
			key := append(prefix, b(uint64(15))...)
			val, err := encodeEntity(entity3)
			if err != nil {
				return err
			}
			err = tx.Set(key, val)
			if err != nil {
				return err
			}

			key = append(prefix, b(uint64(5))...)
			val, err = encodeEntity(entity1)
			if err != nil {
				return err
			}
			err = tx.Set(key, val)
			if err != nil {
				return err
			}

			key = append(prefix, b(uint64(10))...)
			val, err = encodeEntity(entity2)
			if err != nil {
				return err
			}
			err = tx.Set(key, val)
			if err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		var entity Entity

		t.Run("target height exists", func(t *testing.T) {
			err = findHighestAtOrBelow(
				prefix,
				10,
				&entity)(db.NewTransaction(false))
			require.NoError(t, err)
			require.Equal(t, uint64(42), entity.Value)
		})

		t.Run("target height above", func(t *testing.T) {
			err = findHighestAtOrBelow(
				prefix,
				11,
				&entity)(db.NewTransaction(false))
			require.NoError(t, err)
			require.Equal(t, uint64(42), entity.Value)
		})

		t.Run("target height above highest", func(t *testing.T) {
			err = findHighestAtOrBelow(
				prefix,
				20,
				&entity)(db.NewTransaction(false))
			require.NoError(t, err)
			require.Equal(t, uint64(43), entity.Value)
		})

		t.Run("target height below lowest", func(t *testing.T) {
			err = findHighestAtOrBelow(
				prefix,
				4,
				&entity)(db.NewTransaction(false))
			require.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("empty prefix", func(t *testing.T) {
			err = findHighestAtOrBelow(
				[]byte{},
				5,
				&entity)(db.NewTransaction(false))
			require.Error(t, err)
			require.Contains(t, err.Error(), "prefix must not be empty")
		})
	})
}
