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

func TestPayloadStoreRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()

		index := pebblestorage.NewIndex(metrics, db)
		seals := pebblestorage.NewSeals(metrics, db)
		guarantees := pebblestorage.NewGuarantees(metrics, db, pebblestorage.DefaultCacheSize)
		results := pebblestorage.NewExecutionResults(metrics, db)
		receipts := pebblestorage.NewExecutionReceipts(metrics, db, results, pebblestorage.DefaultCacheSize)
		store := pebblestorage.NewPayloads(db, index, guarantees, seals, receipts, results)

		blockID := unittest.IdentifierFixture()
		expected := unittest.PayloadFixture(unittest.WithAllTheFixins)

		// store payload
		err := store.Store(blockID, &expected)
		require.NoError(t, err)

		// fetch payload
		payload, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, &expected, payload)
	})
}

func TestPayloadRetreiveWithoutStore(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()

		index := pebblestorage.NewIndex(metrics, db)
		seals := pebblestorage.NewSeals(metrics, db)
		guarantees := pebblestorage.NewGuarantees(metrics, db, pebblestorage.DefaultCacheSize)
		results := pebblestorage.NewExecutionResults(metrics, db)
		receipts := pebblestorage.NewExecutionReceipts(metrics, db, results, pebblestorage.DefaultCacheSize)
		store := pebblestorage.NewPayloads(db, index, guarantees, seals, receipts, results)

		blockID := unittest.IdentifierFixture()

		_, err := store.ByBlockID(blockID)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
