package store_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
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
		payloads := store.NewClusterPayloads(metrics, db)

		blockID := unittest.IdentifierFixture()
		expected := unittest.ClusterPayloadFixture(5)

		// store payload
		manager := storage.NewTestingLockManager()
		err := unittest.WithLock(t, manager, storage.LockInsertOrFinalizeClusterBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.InsertClusterPayload(lctx, rw, blockID, expected)
			})
		})
		require.NoError(t, err)

		// fetch payload
		payload, err := payloads.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, payload)
	})
}

func TestClusterPayloadRetrieveWithoutStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		payloads := store.NewClusterPayloads(metrics, db)

		_, err := payloads.ByBlockID(unittest.IdentifierFixture()) // attempt to retrieve block for random ID
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}
