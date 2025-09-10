package cmd

import (
	"context"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test save block execution related data, then remove it, and then
// save again should still work
func TestReExecuteBlock(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()

		// bootstrap to init highest executed height
		bootstrapper := bootstrap.NewBootstrapper(unittest.Logger())
		genesis := unittest.BlockFixture()
		rootSeal := unittest.Seal.Fixture(unittest.Seal.WithBlock(genesis.ToHeader()))
		db := pebbleimpl.ToDB(pdb)
		err := bootstrapper.BootstrapExecutionDatabase(lockManager, db, rootSeal)
		require.NoError(t, err)

		// create all modules
		metrics := &metrics.NoopCollector{}

		all := store.InitAll(metrics, db)
		headers := all.Headers
		blocks := all.Blocks
		txResults := store.NewTransactionResults(metrics, db, store.DefaultCacheSize)
		commits := store.NewCommits(metrics, db)
		chunkDataPacks := store.NewChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), store.NewCollections(db, store.NewTransactions(metrics, db)), store.DefaultCacheSize)
		results := all.Results
		receipts := all.Receipts
		myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)
		events := store.NewEvents(metrics, db)
		serviceEvents := store.NewServiceEvents(metrics, db)

		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				// By convention, root block has no proposer signature - implementation has to handle this edge case
				return blocks.BatchStore(lctx, rw, &flow.Proposal{Block: *genesis, ProposerSigData: nil})
			})
		})
		require.NoError(t, err)

		getLatestFinalized := func() (uint64, error) {
			return genesis.Height, nil
		}

		// create execution state module
		es := state.NewExecutionState(
			nil,
			commits,
			nil,
			headers,
			chunkDataPacks,
			results,
			myReceipts,
			events,
			serviceEvents,
			txResults,
			db,
			getLatestFinalized,
			trace.NewNoopTracer(),
			nil,
			false,
			lockManager,
		)
		require.NotNil(t, es)

		computationResult := testutil.ComputationResultFixture(t)
		header := computationResult.Block.ToHeader()

		lctx2 := lockManager.NewContext()
		require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return blocks.BatchStore(lctx2, rw, unittest.ProposalFromBlock(computationResult.Block))
		})
		lctx2.Release()
		require.NoError(t, err)

		// save execution results
		err = es.SaveExecutionResults(context.Background(), computationResult)
		require.NoError(t, err)

		batch := db.NewBatch()
		defer batch.Close()

		chunkBatch := pebbleimpl.ToDB(pdb).NewBatch()
		defer chunkBatch.Close()

		// remove execution results
		err = removeForBlockID(
			batch,
			chunkBatch,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			header.ID(),
		)

		require.NoError(t, err)

		// remove again, to make sure missing entires are handled properly
		err = removeForBlockID(
			batch,
			chunkBatch,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			header.ID(),
		)

		require.NoError(t, err)
		require.NoError(t, chunkBatch.Commit())
		err2 := batch.Commit()

		require.NoError(t, err2)

		batch = db.NewBatch()
		defer batch.Close()

		chunkBatch = pebbleimpl.ToDB(pdb).NewBatch()
		defer chunkBatch.Close()

		// remove again after flushing
		err = removeForBlockID(
			batch,
			chunkBatch,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			header.ID(),
		)
		require.NoError(t, err)

		require.NoError(t, chunkBatch.Commit())
		err2 = batch.Commit()

		require.NoError(t, err2)

		// re execute result
		err = es.SaveExecutionResults(context.Background(), computationResult)
		require.NoError(t, err)
	})
}

// Test save block execution related data, then remove it, and then
// save again with different result should work
func TestReExecuteBlockWithDifferentResult(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {

		// bootstrap to init highest executed height
		bootstrapper := bootstrap.NewBootstrapper(unittest.Logger())
		genesis := unittest.BlockFixture()
		rootSeal := unittest.Seal.Fixture()
		unittest.Seal.WithBlock(genesis.ToHeader())(rootSeal)

		db := pebbleimpl.ToDB(pdb)
		err := bootstrapper.BootstrapExecutionDatabase(lockManager, db, rootSeal)
		require.NoError(t, err)

		// create all modules
		metrics := &metrics.NoopCollector{}
		all := store.InitAll(metrics, db)
		headers := all.Headers
		blocks := all.Blocks
		commits := store.NewCommits(metrics, db)
		results := store.NewExecutionResults(metrics, db)
		receipts := store.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
		myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)
		events := store.NewEvents(metrics, db)
		serviceEvents := store.NewServiceEvents(metrics, db)
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)
		chunkDataPacks := store.NewChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), collections, bstorage.DefaultCacheSize)
		txResults := store.NewTransactionResults(metrics, db, bstorage.DefaultCacheSize)

		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				// By convention, root block has no proposer signature - implementation has to handle this edge case
				return blocks.BatchStore(lctx, rw, &flow.Proposal{Block: *genesis, ProposerSigData: nil})
			})
		})

		getLatestFinalized := func() (uint64, error) {
			return genesis.Height, nil
		}

		// create execution state module
		es := state.NewExecutionState(
			nil,
			commits,
			nil,
			headers,
			chunkDataPacks,
			results,
			myReceipts,
			events,
			serviceEvents,
			txResults,
			db,
			getLatestFinalized,
			trace.NewNoopTracer(),
			nil,
			false,
			lockManager,
		)
		require.NotNil(t, es)

		executableBlock := unittest.ExecutableBlockFixtureWithParent(
			nil,
			genesis.ToHeader(),
			&unittest.GenesisStateCommitment)
		blockID := executableBlock.Block.ID()

		unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, unittest.ProposalFromBlock(executableBlock.Block))
			})
		})

		computationResult := testutil.ComputationResultFixture(t)
		computationResult.ExecutableBlock = executableBlock
		computationResult.ExecutionReceipt.ExecutionResult.BlockID = blockID

		// save execution results
		err = es.SaveExecutionResults(context.Background(), computationResult)
		require.NoError(t, err)

		batch := db.NewBatch()
		defer batch.Close()

		chunkBatch := db.NewBatch()
		defer chunkBatch.Close()

		// remove execution results
		err = removeForBlockID(
			batch,
			chunkBatch,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			blockID,
		)

		require.NoError(t, err)
		require.NoError(t, chunkBatch.Commit())
		err2 := batch.Commit()
		require.NoError(t, err2)

		batch = db.NewBatch()
		defer batch.Close()

		chunkBatch = db.NewBatch()
		defer chunkBatch.Close()

		// remove again to test for duplicates handling
		err = removeForBlockID(
			batch,
			chunkBatch,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			blockID,
		)

		require.NoError(t, err)
		require.NoError(t, chunkBatch.Commit())

		err2 = batch.Commit()
		require.NoError(t, err2)

		computationResult2 := testutil.ComputationResultFixture(t)
		computationResult2.ExecutableBlock = executableBlock
		computationResult2.ExecutionReceipt.ExecutionResult.BlockID = blockID

		// re execute result
		err = es.SaveExecutionResults(context.Background(), computationResult2)
		require.NoError(t, err)
	})
}
