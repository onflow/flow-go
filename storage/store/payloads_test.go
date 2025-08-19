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

func TestPayloadStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()

		all := store.InitAll(metrics, db)
		payloads := all.Payloads
		blocks := all.Blocks

		expected := unittest.PayloadFixture(unittest.WithAllTheFixins)
		block := unittest.BlockWithParentAndPayload(unittest.BlockHeaderWithHeight(10), expected)
		require.Equal(t, &expected, block.Payload)
		blockID := block.ID()

		_, lctx := unittest.LockManagerWithContext(t, storage.LockInsertBlock)
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx, rw, block)
		})
		lctx.Release()
		require.NoError(t, err)

		// fetch payload
		payload, err := payloads.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, &expected, payload)
	})
}

func TestPayloadRetreiveWithoutStore(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()

		index := store.NewIndex(metrics, db)
		seals := store.NewSeals(metrics, db)
		guarantees := store.NewGuarantees(metrics, db, store.DefaultCacheSize, store.DefaultCacheSize)
		results := store.NewExecutionResults(metrics, db)
		receipts := store.NewExecutionReceipts(metrics, db, results, store.DefaultCacheSize)
		s := store.NewPayloads(db, index, guarantees, seals, receipts, results)

		blockID := unittest.IdentifierFixture()

		_, err := s.ByBlockID(blockID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}
