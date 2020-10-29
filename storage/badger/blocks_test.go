package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"

	stoerr "github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlocks(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		store := bstorage.NewBlocks(db, nil, nil)

		// check retrieval of non-existing key
		_, err := store.GetLastFullBlockHeight()
		assert.Error(t, err)
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))

		// insert a value for height
		var height1 = uint64(1234)
		err = store.UpdateLastFullBlockHeight(height1)
		assert.NoError(t, err)

		// check value can be retrieved
		actual, err := store.GetLastFullBlockHeight()
		assert.NoError(t, err)
		assert.Equal(t, height1, actual)

		// update the value for height
		var height2 = uint64(1234)
		err = store.UpdateLastFullBlockHeight(height2)
		assert.NoError(t, err)

		// check that the new value can be retrieved
		actual, err = store.GetLastFullBlockHeight()
		assert.NoError(t, err)
		assert.Equal(t, height2, actual)

	})
}
