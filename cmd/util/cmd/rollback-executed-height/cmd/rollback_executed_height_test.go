package cmd

import (
	"context"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/dgraph-io/badger/v2"
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
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test save block execution related data, then remove it, and then
// save again should still work
func TestReExecuteBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(bdb *badger.DB) {
		unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {

			// bootstrap to init highest executed height
			bootstrapper := bootstrap.NewBootstrapper(unittest.Logger())
			genesis := unittest.BlockFixture()
			rootSeal := unittest.Seal.Fixture(unittest.Seal.WithBlock(genesis.Header))
			db := badgerimpl.ToDB(bdb)
			bootstrapLockManager := storage.NewTestingLockManager()
			err := bootstrapper.BootstrapExecutionDatabase(bootstrapLockManager, db, rootSeal)
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

<<<<<<< HEAD
			manager, lctx := unittest.LockManagerWithContext(t, storage.LockInsertBlock)
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, &genesis)
			})
			lctx.Release()
=======
			// By convention, root block has no proposer signature - implementation has to handle this edge case
			err = headers.Store(&flow.ProposalHeader{Header: genesis, ProposerSigData: nil})
>>>>>>> @{-1}
			require.NoError(t, err)

			getLatestFinalized := func() (uint64, error) {
				return genesis.Header.Height, nil
			}

			lockManager := storage.NewTestingLockManager()

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

<<<<<<< HEAD
			lctx2 := manager.NewContext()
			require.NoError(t, lctx2.AcquireLock(storage.LockInsertBlock))
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx2, rw, computationResult.Block)
			})
			lctx2.Release()
=======
			err = headers.Store(unittest.ProposalHeaderFromHeader(header))
>>>>>>> @{-1}
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
	})
}

func withLock(t *testing.T, manager lockctx.Manager, lockID string, fn func(lctx lockctx.Context) error) {
	t.Helper()
	lctx := manager.NewContext()
	require.NoError(t, lctx.AcquireLock(lockID))
	defer lctx.Release()
	require.NoError(t, fn(lctx))
}

// Test save block execution related data, then remove it, and then
// save again with different result should work
func TestReExecuteBlockWithDifferentResult(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {

		// bootstrap to init highest executed height
		bootstrapper := bootstrap.NewBootstrapper(unittest.Logger())
		genesis := unittest.BlockFixture()
		rootSeal := unittest.Seal.Fixture()
		unittest.Seal.WithBlock(genesis.Header)(rootSeal)

		db := pebbleimpl.ToDB(pdb)
		lockManager := storage.NewTestingLockManager()
		err := bootstrapper.BootstrapExecutionDatabase(lockManager, db, rootSeal)
		require.NoError(t, err)

		// create all modules
		metrics := &metrics.NoopCollector{}

		all := store.InitAll(metrics, db)
		headers := all.Headers
		blocks := all.Blocks

<<<<<<< HEAD
		txResults := store.NewTransactionResults(metrics, db, bstorage.DefaultCacheSize)
		commits := store.NewCommits(metrics, db)
		results := store.NewExecutionResults(metrics, db)
		receipts := store.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
		myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)
		events := store.NewEvents(metrics, db)
		serviceEvents := store.NewServiceEvents(metrics, db)
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)
		chunkDataPacks := store.NewChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), collections, bstorage.DefaultCacheSize)

		withLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, &genesis)
			})
=======
			// By convention, root block has no proposer signature - implementation has to handle this edge case
			err = headers.Store(&flow.ProposalHeader{Header: genesis, ProposerSigData: nil})
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
			)
			require.NotNil(t, es)

			executableBlock := unittest.ExecutableBlockFixtureWithParent(
				nil,
				genesis,
				&unittest.GenesisStateCommitment)
			header := executableBlock.Block.ToHeader()

			err = headers.Store(unittest.ProposalHeaderFromHeader(header))
			require.NoError(t, err)

			computationResult := testutil.ComputationResultFixture(t)
			computationResult.ExecutableBlock = executableBlock
			computationResult.ExecutionReceipt.ExecutionResult.BlockID = header.ID()

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
			require.NoError(t, chunkBatch.Commit())
			err2 := batch.Commit()
			require.NoError(t, err2)

			batch = db.NewBatch()
			defer batch.Close()

			chunkBatch = pebbleimpl.ToDB(pdb).NewBatch()
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
				header.ID(),
			)

			require.NoError(t, err)
			require.NoError(t, chunkBatch.Commit())

			err2 = batch.Commit()
			require.NoError(t, err2)

			computationResult2 := testutil.ComputationResultFixture(t)
			computationResult2.ExecutableBlock = executableBlock
			computationResult2.ExecutionReceipt.ExecutionResult.BlockID = header.ID()

			// re execute result
			err = es.SaveExecutionResults(context.Background(), computationResult2)
			require.NoError(t, err)
>>>>>>> @{-1}
		})

		getLatestFinalized := func() (uint64, error) {
			return genesis.Header.Height, nil
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
			genesis.Header,
			&unittest.GenesisStateCommitment)
		header := executableBlock.Block.Header

		withLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return blocks.BatchStore(lctx, rw, executableBlock.Block)
			})
		})

		computationResult := testutil.ComputationResultFixture(t)
		computationResult.ExecutableBlock = executableBlock
		computationResult.ExecutionReceipt.ExecutionResult.BlockID = header.ID()

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
			header.ID(),
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
			header.ID(),
		)

		require.NoError(t, err)
		require.NoError(t, chunkBatch.Commit())

		err2 = batch.Commit()
		require.NoError(t, err2)

		computationResult2 := testutil.ComputationResultFixture(t)
		computationResult2.ExecutableBlock = executableBlock
		computationResult2.ExecutionResult.BlockID = header.ID()

		// re execute result
		err = es.SaveExecutionResults(context.Background(), computationResult2)
		require.NoError(t, err)
	})
}
