package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/procedure"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestStoreRetrieveClusterPayload(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewClusterPayloads(metrics, db)

		blockID := unittest.IdentifierFixture()
		expected := unittest.ClusterPayloadFixture(5)

		// store payload
		manager := storage.NewTestingLockManager()
		lctx := manager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.InsertClusterPayload(lctx, rw, blockID, expected)
		})
		lctx.Release()
		require.NoError(t, err)

		// fetch payload
		payload, err := store1.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, payload)

		// storing again should error with key already exists
		lctx = manager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.InsertClusterPayload(lctx, rw, blockID, expected)
		})
		lctx.Release()
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestClusterPayloadRetrieveWithoutStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store1 := store.NewClusterPayloads(metrics, db)

		blockID := unittest.IdentifierFixture()

		_, err := store1.ByBlockID(blockID)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}
