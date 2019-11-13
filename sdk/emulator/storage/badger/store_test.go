package badger_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/dapperlabs/flow-go/sdk/emulator/storage"

	"github.com/dapperlabs/flow-go/sdk/emulator/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreTransaction(t *testing.T) {
	dir, err := ioutil.TempDir("", "badger-test")
	require.Nil(t, err)
	defer require.Nil(t, os.RemoveAll(dir))

	store, err := badger.New(&badger.Config{Path: dir})
	require.Nil(t, err)

	tx := unittest.TransactionFixture()

	t.Run("should return error for not found", func(t *testing.T) {
		_, err := store.GetTransaction(tx.Hash())
		if assert.Error(t, err) {
			assert.IsType(t, storage.ErrNotFound{}, err)
		}
	})

	t.Run("should be able to insert", func(t *testing.T) {
		err := store.InsertTransaction(tx)
		assert.Nil(t, err)
	})

	t.Run("should be able to get inserted tx", func(t *testing.T) {
		err := store.InsertTransaction(tx)
		assert.Nil(t, err)

		storedTx, err := store.GetTransaction(tx.Hash())
		require.Nil(t, err)
		assert.Equal(t, tx, storedTx)
	})
}

func TestStoreBlock(t *testing.T) {
	dir, err := ioutil.TempDir("", "badger-test")
	require.Nil(t, err)
	defer require.Nil(t, os.RemoveAll(dir))

	store, err := badger.New(&badger.Config{Path: dir})
	require.Nil(t, err)

	_ = store
}
