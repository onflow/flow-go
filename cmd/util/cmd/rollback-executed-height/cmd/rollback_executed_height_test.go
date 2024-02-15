package cmd

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test save block execution related data, then remove it, and then
// save again should still work
func TestReExecuteBlock(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		// bootstrap to init highest executed height
		bootstrapper := bootstrap.NewBootstrapper(unittest.Logger())
		genesis := unittest.BlockHeaderFixture()
		rootSeal := unittest.Seal.Fixture(unittest.Seal.WithBlock(genesis))
		err := bootstrapper.BootstrapExecutionDatabase(db, rootSeal)
		require.NoError(t, err)

		// create all modules
		metrics := &metrics.NoopCollector{}

		headers := bstorage.NewHeaders(metrics, db)
		txResults := bstorage.NewTransactionResults(metrics, db, bstorage.DefaultCacheSize)
		commits := bstorage.NewCommits(metrics, db)
		chunkDataPacks := bstorage.NewChunkDataPacks(metrics, db, bstorage.NewCollections(db, bstorage.NewTransactions(metrics, db)), bstorage.DefaultCacheSize)
		results := bstorage.NewExecutionResults(metrics, db)
		receipts := bstorage.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
		myReceipts := bstorage.NewMyExecutionReceipts(metrics, db, receipts)
		events := bstorage.NewEvents(metrics, db)
		serviceEvents := bstorage.NewServiceEvents(metrics, db)
		transactions := bstorage.NewTransactions(metrics, db)
		collections := bstorage.NewCollections(db, transactions)

		err = headers.Store(genesis)
		require.NoError(t, err)

		// create execution state module
		es := state.NewExecutionState(
			nil,
			commits,
			nil,
			headers,
			collections,
			chunkDataPacks,
			results,
			myReceipts,
			events,
			serviceEvents,
			txResults,
			db,
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

		batch := bstorage.NewBatch(db)

		// remove execution results
		err = removeForBlockID(
			batch,
			headers,
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
			headers,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			header.ID(),
		)

		err2 := batch.Flush()

		require.NoError(t, err)
		require.NoError(t, err2)

		batch = bstorage.NewBatch(db)

		// remove again after flushing
		err = removeForBlockID(
			batch,
			headers,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			header.ID(),
		)
		err2 = batch.Flush()

		require.NoError(t, err)
		require.NoError(t, err2)

		// re execute result
		err = es.SaveExecutionResults(context.Background(), computationResult)
		require.NoError(t, err)
	})
}

// Test save block execution related data, then remove it, and then
// save again with different result should work
func TestReExecuteBlockWithDifferentResult(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		// bootstrap to init highest executed height
		bootstrapper := bootstrap.NewBootstrapper(unittest.Logger())
		genesis := unittest.BlockHeaderFixture()
		rootSeal := unittest.Seal.Fixture()
		unittest.Seal.WithBlock(genesis)(rootSeal)
		err := bootstrapper.BootstrapExecutionDatabase(db, rootSeal)
		require.NoError(t, err)

		// create all modules
		metrics := &metrics.NoopCollector{}

		headers := bstorage.NewHeaders(metrics, db)
		txResults := bstorage.NewTransactionResults(metrics, db, bstorage.DefaultCacheSize)
		commits := bstorage.NewCommits(metrics, db)
		chunkDataPacks := bstorage.NewChunkDataPacks(metrics, db, bstorage.NewCollections(db, bstorage.NewTransactions(metrics, db)), bstorage.DefaultCacheSize)
		results := bstorage.NewExecutionResults(metrics, db)
		receipts := bstorage.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
		myReceipts := bstorage.NewMyExecutionReceipts(metrics, db, receipts)
		events := bstorage.NewEvents(metrics, db)
		serviceEvents := bstorage.NewServiceEvents(metrics, db)
		transactions := bstorage.NewTransactions(metrics, db)
		collections := bstorage.NewCollections(db, transactions)

		err = headers.Store(genesis)
		require.NoError(t, err)

		// create execution state module
		es := state.NewExecutionState(
			nil,
			commits,
			nil,
			headers,
			collections,
			chunkDataPacks,
			results,
			myReceipts,
			events,
			serviceEvents,
			txResults,
			db,
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

		batch := bstorage.NewBatch(db)

		// remove execution results
		err = removeForBlockID(
			batch,
			headers,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			header.ID(),
		)

		err2 := batch.Flush()

		require.NoError(t, err)
		require.NoError(t, err2)

		batch = bstorage.NewBatch(db)

		// remove again to test for duplicates handling
		err = removeForBlockID(
			batch,
			headers,
			commits,
			txResults,
			results,
			chunkDataPacks,
			myReceipts,
			events,
			serviceEvents,
			header.ID(),
		)

		err2 = batch.Flush()

		require.NoError(t, err)
		require.NoError(t, err2)

		computationResult2 := testutil.ComputationResultFixture(t)
		computationResult2.ExecutableBlock = executableBlock
		computationResult2.ExecutionResult.BlockID = header.ID()

		// re execute result
		err = es.SaveExecutionResults(context.Background(), computationResult2)
		require.NoError(t, err)
	})
}
