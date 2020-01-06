// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Entity struct {
	ID uint64
}

func TestInsertValid(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

		err := db.Update(insert(key, e))
		require.Nil(t, err)

		var act []byte
		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.Nil(t, err)
			act, err = item.ValueCopy(nil)
			require.Nil(t, err)
			return nil
		})

		assert.Equal(t, act, val)
	})
}

func TestInsertDuplicate(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		e := Entity{ID: 1337}
		e2 := Entity{ID: 1338}
		key := []byte{0x01, 0x02, 0x03}

		// persist first time
		err := db.Update(insert(key, e))
		require.NoError(t, err)

		// Insert again
		err = db.Update(insert(key, e))
		require.Error(t, err)
		require.Equal(t, err, storage.KeyAlreadyExistsErr)

		// persist again, but using different method
		err = db.Update(persist(key, e))
		require.NoError(t, err)

		// again with different data
		err = db.Update(persist(key, e2))
		require.Error(t, err)
		require.Equal(t, err, storage.DifferentDataErr)

	})
}

func TestUpdateValid(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, []byte{})
			require.Nil(t, err)
			return nil
		})

		err := db.Update(update(key, e))
		require.Nil(t, err)

		var act []byte
		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.Nil(t, err)
			act, err = item.ValueCopy(nil)
			require.Nil(t, err)
			return nil
		})

		assert.Equal(t, act, val)
	})
}

func TestUpdateMissing(t *testing.T) {
	// TODO
}

func TestRetrieveValid(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val := []byte(`{"ID":1337}`)

		_ = db.Update(func(tx *badger.Txn) error {
			err := tx.Set(key, val)
			require.Nil(t, err)
			return nil
		})

		var act Entity
		err := db.View(retrieve(key, &act))
		require.Nil(t, err)

		assert.Equal(t, act, e)

	})

}

func TestRetrieveMissing(t *testing.T) {
	// TODO
}

func TestScan(t *testing.T) {
	// TODO
}

func TestTraverse(t *testing.T) {
	// TODO
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
			db.Update(func(txn *badger.Txn) error {
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
			db.Update(func(txn *badger.Txn) error {
				err := remove(nonexistantKey)(txn)
				assert.Error(t, err)
				t.Log(err)
				return nil
			})
		})
	})

}
