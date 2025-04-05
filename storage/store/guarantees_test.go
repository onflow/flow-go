package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGuaranteeStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		s := store.NewGuarantees(metrics, db, 1000)

		// abiturary guarantees
		expected := unittest.CollectionGuaranteeFixture()

		// retrieve guarantee without stored
		_, err := s.ByCollectionID(expected.ID())
		require.ErrorIs(t, err, storage.ErrNotFound)

		// store guarantee
		err = s.Store(expected)
		require.NoError(t, err)

		// retreive by coll idx
		actual, err := s.ByCollectionID(expected.ID())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}
