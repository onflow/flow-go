package pebble_test

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

func TestGuaranteeStoreRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewGuarantees(metrics, db, 1000)

		// abiturary guarantees
		expected := unittest.CollectionGuaranteeFixture()

		// retrieve guarantee without stored
		_, err := store.ByCollectionID(expected.ID())
		require.True(t, errors.Is(err, storage.ErrNotFound))

		// store guarantee
		err = store.Store(expected)
		require.NoError(t, err)

		// retreive by coll idx
		actual, err := store.ByCollectionID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}
