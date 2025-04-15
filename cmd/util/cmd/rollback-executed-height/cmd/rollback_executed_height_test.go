package cmd

import (
	"context"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
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
			genesis := unittest.BlockHeaderFixture()
			rootSeal := unittest.Seal.Fixture(unittest.Seal.WithBlock(genesis))
			db := badgerimpl.ToDB(bdb)
			err := bootstrapper.BootstrapExecutionDatabase(db, rootSeal)
			require.NoError(t, err)

			// create all modules
			metrics := &metrics.NoopCollector{}

			headers := bstorage.NewHeaders(metrics, bdb)
			txResults := store.NewTransactionResults(metrics, db, bstorage.DefaultCacheSize)
			commits := store.NewCommits(metrics, db)
			chunkDataPacks := store.NewChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), bstorage.NewCollections(bdb, bstorage.NewTransactions(metrics, bdb)), bstorage.DefaultCacheSize)
			results := store.NewExecutionResults(metrics, db)
			receipts := store.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
			myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)
			events := store.NewEvents(metrics, db)
			serviceEvents := store.NewServiceEvents(metrics, db)

			err = headers.Store(genesis)
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

			computationResult := testutil.ComputationResultFixture(t)
			header := computationResult.Block.Header

			err = headers.Store(header)
			require.NoError(t, err)

			// save execution results
			err = es.SaveExecutionResults(context.Background(), computationResult)
			require.NoError(t, err)

			batch := db.NewBatch()
			chunkBatch := pebbleimpl.ToDB(pdb).NewBatch()

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
			chunkBatch = pebbleimpl.ToDB(pdb).NewBatch()

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

// Test save block execution related data, then remove it, and then
// save again with different result should work
func TestReExecuteBlockWithDifferentResult(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(bdb *badger.DB) {
		unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {

			// bootstrap to init highest executed height
			bootstrapper := bootstrap.NewBootstrapper(unittest.Logger())
			genesis := unittest.BlockHeaderFixture()
			rootSeal := unittest.Seal.Fixture()
			unittest.Seal.WithBlock(genesis)(rootSeal)

			db := badgerimpl.ToDB(bdb)
			err := bootstrapper.BootstrapExecutionDatabase(db, rootSeal)
			require.NoError(t, err)

			// create all modules
			metrics := &metrics.NoopCollector{}

			headers := bstorage.NewHeaders(metrics, bdb)
			txResults := store.NewTransactionResults(metrics, db, bstorage.DefaultCacheSize)
			commits := store.NewCommits(metrics, db)
			results := store.NewExecutionResults(metrics, db)
			receipts := store.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
			myReceipts := store.NewMyExecutionReceipts(metrics, db, receipts)
			events := store.NewEvents(metrics, db)
			serviceEvents := store.NewServiceEvents(metrics, db)
			transactions := bstorage.NewTransactions(metrics, bdb)
			collections := bstorage.NewCollections(bdb, transactions)
			chunkDataPacks := store.NewChunkDataPacks(metrics, pebbleimpl.ToDB(pdb), collections, bstorage.DefaultCacheSize)

			err = headers.Store(genesis)
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
			header := executableBlock.Block.Header

			err = headers.Store(header)
			require.NoError(t, err)

			computationResult := testutil.ComputationResultFixture(t)
			computationResult.ExecutableBlock = executableBlock
			computationResult.ExecutionReceipt.ExecutionResult.BlockID = header.ID()

			// save execution results
			err = es.SaveExecutionResults(context.Background(), computationResult)
			require.NoError(t, err)

			batch := db.NewBatch()
			chunkBatch := pebbleimpl.ToDB(pdb).NewBatch()

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
			chunkBatch = pebbleimpl.ToDB(pdb).NewBatch()

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
	})
}
