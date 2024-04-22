package badger_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMonotonousConsumer(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store := badgerstorage.NewMonotonousConsumerProgress(db, module.ConsumeProgressLastFullBlockHeight)

		// get value of non-initialized
		_, err := store.ProcessedIndex()
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound))

		// update value of non-initialized
		var height1 = uint64(1234)
		err = store.SetProcessedIndex(height1)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound))

		// initialize & update
		err = store.InitProcessedIndex(0)
		require.NoError(t, err)
		err = store.SetProcessedIndex(height1)
		require.NoError(t, err)

		// check value can be retrieved
		actual, err := store.ProcessedIndex()
		require.NoError(t, err)
		require.Equal(t, height1, actual)

		// update value to less than current
		var lessHeight = uint64(1233)
		err = store.SetProcessedIndex(lessHeight)
		require.Error(t, err)
		require.Equal(t, err, fmt.Errorf("could not update to height that is lower than the current height"))

		// update the value for bigger height
		var height2 = uint64(1235)
		err = store.SetProcessedIndex(height2)
		require.NoError(t, err)

		// check that the new value can be retrieved
		actual, err = store.ProcessedIndex()
		require.NoError(t, err)
		require.Equal(t, height2, actual)
	})
}
