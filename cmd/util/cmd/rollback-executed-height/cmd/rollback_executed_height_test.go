package cmd

import (
	"context"
	"errors"
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

		all := store.InitAll(metrics, db, flow.Emulator)
		headers := all.Headers
		blocks := all.Blocks
		txResults, err := store.NewTransactionResults(metrics, db, store.DefaultCacheSize)
		require.NoError(t, err)
		commits := store.NewCommits(metrics, db)
		storedChunkDataPacks := store.NewStoredChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), store.DefaultCacheSize)
		chunkDataPacks := store.NewChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), storedChunkDataPacks, store.NewCollections(db, store.NewTransactions(metrics, db)), store.DefaultCacheSize)
		results := all.Results
		receipts := all.Receipts
		myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)
		events := store.NewEvents(metrics, db)
		serviceEvents := store.NewServiceEvents(metrics, db)

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
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

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, unittest.ProposalFromBlock(computationResult.Block))
			})
		})
		require.NoError(t, err)

		// save execution results
		err = es.SaveExecutionResults(context.Background(), computationResult)
		require.NoError(t, err)

		batch := db.NewBatch()
		defer batch.Close()

		// remove execution results
		var cdpIDs []flow.Identifier
		cdpIDs, err = removeForBlockID(
			batch,
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
		additionalCdpIDs, err := removeForBlockID(
			batch,
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
		// combine chunk data pack IDs from both calls
		cdpIDs = append(cdpIDs, additionalCdpIDs...)
		require.NoError(t, storedChunkDataPacks.Remove(cdpIDs))
		err2 := batch.Commit()

		require.NoError(t, err2)

		// verify that chunk data packs are no longer in stored chunk data pack database
		for _, cdpID := range cdpIDs {
			_, err := storedChunkDataPacks.ByID(cdpID)
			require.Error(t, err)
			require.True(t, errors.Is(err, storage.ErrNotFound))
		}

		batch = db.NewBatch()
		defer batch.Close()

		// remove again after flushing
		cdpIDs, err = removeForBlockID(
			batch,
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
		require.NoError(t, storedChunkDataPacks.Remove(cdpIDs))
		err2 = batch.Commit()

		require.NoError(t, err2)

		// verify that chunk data packs are no longer in stored chunk data pack database
		for _, cdpID := range cdpIDs {
			_, err := storedChunkDataPacks.ByID(cdpID)
			require.Error(t, err)
			require.True(t, errors.Is(err, storage.ErrNotFound))
		}

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
		all := store.InitAll(metrics, db, flow.Emulator)
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
		storedChunkDataPacks := store.NewStoredChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), bstorage.DefaultCacheSize)
		chunkDataPacks := store.NewChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), storedChunkDataPacks, collections, bstorage.DefaultCacheSize)
		txResults, err := store.NewTransactionResults(metrics, db, bstorage.DefaultCacheSize)
		require.NoError(t, err)

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
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

		executableBlock := unittest.ExecutableBlockFixtureWithParent(
			nil,
			genesis.ToHeader(),
			&unittest.GenesisStateCommitment)
		blockID := executableBlock.Block.ID()

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, unittest.ProposalFromBlock(executableBlock.Block))
			})
		})
		require.NoError(t, err)

		computationResult := testutil.ComputationResultFixture(t)
		computationResult.ExecutableBlock = executableBlock
		computationResult.ExecutionReceipt.ExecutionResult.BlockID = blockID

		// save execution results
		err = es.SaveExecutionResults(context.Background(), computationResult)
		require.NoError(t, err)

		batch := db.NewBatch()
		defer batch.Close()

		// remove execution results
		cdpIDs, err := removeForBlockID(
			batch,
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
		require.NoError(t, storedChunkDataPacks.Remove(cdpIDs))
		err2 := batch.Commit()
		require.NoError(t, err2)

		// verify that chunk data packs are no longer in stored chunk data pack database
		for _, cdpID := range cdpIDs {
			_, err := storedChunkDataPacks.ByID(cdpID)
			require.Error(t, err)
			require.True(t, errors.Is(err, storage.ErrNotFound))
		}

		batch = db.NewBatch()
		defer batch.Close()

		// remove again to test for duplicates handling
		additionalCdpIDs, err := removeForBlockID(
			batch,
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
		// combine chunk data pack IDs from both calls
		cdpIDs = append(cdpIDs, additionalCdpIDs...)
		require.NoError(t, storedChunkDataPacks.Remove(cdpIDs))

		err2 = batch.Commit()
		require.NoError(t, err2)

		// verify that chunk data packs are no longer in stored chunk data pack database
		for _, cdpID := range cdpIDs {
			_, err := storedChunkDataPacks.ByID(cdpID)
			require.Error(t, err)
			require.True(t, errors.Is(err, storage.ErrNotFound))
		}

		computationResult2 := testutil.ComputationResultFixture(t)
		computationResult2.ExecutableBlock = executableBlock
		computationResult2.ExecutionReceipt.ExecutionResult.BlockID = blockID

		// re execute result
		err = es.SaveExecutionResults(context.Background(), computationResult2)
		require.NoError(t, err)
	})
}
