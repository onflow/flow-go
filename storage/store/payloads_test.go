package store_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPayloadStoreRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()

		all, err := store.InitAll(metrics, db, flow.Emulator)
		require.NoError(t, err)
		payloads := all.Payloads
		blocks := all.Blocks

		expected := unittest.PayloadFixture(unittest.WithAllTheFixins)
		block := unittest.BlockWithParentAndPayload(unittest.BlockHeaderWithHeight(10), expected)
		proposal := unittest.ProposalFromBlock(block)
		require.Equal(t, expected, block.Payload)
		blockID := block.ID()

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, proposal)
			})
		})
		require.NoError(t, err)

		// fetch payload
		payload, err := payloads.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, *payload)
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
