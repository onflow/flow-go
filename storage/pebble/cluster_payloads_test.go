package pebble_test

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

func TestStoreRetrieveClusterPayload(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewClusterPayloads(metrics, db)

		blockID := unittest.IdentifierFixture()
		expected := unittest.ClusterPayloadFixture(5)

		// store payload
		err := store.Store(blockID, expected)
		require.NoError(t, err)

		// fetch payload
		payload, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, payload)

		// storing again should error with key already exists
		err = store.Store(blockID, expected)
		require.True(t, errors.Is(err, storage.ErrAlreadyExists))
	})
}

func TestClusterPayloadRetrieveWithoutStore(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		store := pebblestorage.NewClusterPayloads(metrics, db)

		blockID := unittest.IdentifierFixture()

		_, err := store.ByBlockID(blockID)
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
