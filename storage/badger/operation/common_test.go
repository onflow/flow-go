// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Entity struct {
	ID uint64
}

type UnencodeableEntity bool

var errCantEncode = fmt.Errorf("MarshalJSON not supported")
var errCantDecode = fmt.Errorf("UnmarshalJSON not supported")

func (a UnencodeableEntity) MarshalJSON() ([]byte, error) {
	return nil, errCantEncode
}

func (a *UnencodeableEntity) UnmarshalJSON(b []byte) error {
	return errCantDecode
}

func TestInsertValid(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

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
		val := []byte(`{"ID":1337}`)

		// persist first time
		err := db.Update(insert(key, e))
		require.NoError(t, err)

		e2 := Entity{ID: 1338}

		// persist again
		err = db.Update(insert(key, e2))
		require.Error(t, err)
		require.Equal(t, err, storage.ErrAlreadyExists)

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
		key := []byte{0x01, 0x02, 0x03}

		err := db.Update(insert(key, UnencodeableEntity(true)))

		require.True(t, errors.Is(err, errCantEncode))
	})
}

func TestCheckTrue(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, val)
			require.NoError(t, err)
			return nil
		})

		var exists bool
		err := db.View(check(key, &exists))
		require.NoError(t, err)
		require.True(t, exists)
	})
}

func TestCheckFalse(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		key := []byte{0x01, 0x02, 0x03}

		var exists bool
		err := db.View(check(key, &exists))
		require.NoError(t, err)
		require.False(t, exists)
	})
}

func TestUpdateValid(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

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
		require.Equal(t, storage.ErrNotFound, err)

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
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, val)
			require.NoError(t, err)
			return nil
		})

		err := db.Update(update(key, UnencodeableEntity(true)))
		require.True(t, errors.Is(err, errCantEncode))

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

func TestRetrieveValid(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

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
		require.Equal(t, storage.ErrNotFound, err)
	})
}

func TestRetrieveUnencodeable(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, val)
			require.NoError(t, err)
			return nil
		})

		var act UnencodeableEntity
		err := db.View(retrieve(key, &act))
		require.True(t, errors.Is(err, errCantDecode))
	})
}

func TestLookup(t *testing.T) {
	expected := []flow.Identifier{
		flow.Identifier{0x01},
		flow.Identifier{0x02},
		flow.Identifier{0x03},
		flow.Identifier{0x04},
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
		keys := [][]byte{[]byte{0x00}, []byte{0x12}, []byte{0xf0}, []byte{0xff}}
		vals := []bool{false, false, true, false}
		expected := []bool{false, true}

		_ = db.Update(func(tx *badger.Txn) error {
			for i, key := range keys {
				enc, err := json.Marshal(vals[i])
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
		keys := [][]byte{[]byte{0x42, 0x00}, []byte{0xff}, []byte{0x42, 0x56}, []byte{0x00}, []byte{0x42, 0xff}}
		vals := []bool{false, false, true, false, true}
		expected := []bool{false, true}

		_ = db.Update(func(tx *badger.Txn) error {
			for i, key := range keys {
				enc, err := json.Marshal(vals[i])
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
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

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
				assert.Error(t, err)
				assert.IsType(t, badger.ErrKeyNotFound, err)

				return nil
			})
		})

		t.Run("should error when removing non-existant value", func(t *testing.T) {
			nonexistantKey := append(key, 0x01)
			_ = db.Update(func(txn *badger.Txn) error {
				err := remove(nonexistantKey)(txn)
				assert.Error(t, err)
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
		{0x09, 0xff},
		{0x10, 0x00},
		{0x10, 0xff},
		{0x11, 0x00},
		{0x19, 0xff},
		{0x20, 0x00},
		{0x20, 0xff},
		{0x21, 0x00},
	}

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
		require.NoError(t, err, "should iterate forward without error")
		require.ElementsMatch(t, keys[1:7], found, "forward iteration should go over correct keys")

		// iterate backward and check boundaries are included correctly
		found = nil
		err = db.View(iterate(end, start, iteration))
		require.NoError(t, err, "should iterate backward without error")
		require.ElementsMatch(t, keys[1:7], found, "backward iteration should go over correct keys")
	})
}
