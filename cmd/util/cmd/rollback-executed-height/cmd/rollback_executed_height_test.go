package cmd

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/model/flow"
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
		err := bootstrapper.BootstrapExecutionDatabase(db, unittest.StateCommitmentFixture(), &genesis)
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

		err = headers.Store(&genesis)
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
		)
		require.NotNil(t, es)

		// prepare data
		header := unittest.BlockHeaderWithParentFixture(&genesis) // make sure the height is higher than genesis
		executionReceipt := unittest.ExecutionReceiptFixture()
		executionReceipt.ExecutionResult.BlockID = header.ID()
		cdp := make([]*flow.ChunkDataPack, 0, len(executionReceipt.ExecutionResult.Chunks))
		for _, chunk := range executionReceipt.ExecutionResult.Chunks {
			cdp = append(cdp, unittest.ChunkDataPackFixture(chunk.ID()))
		}
		endState, err := executionReceipt.ExecutionResult.FinalStateCommitment()
		require.NoError(t, err)
		blockEvents := unittest.BlockEventsFixture(header, 3)
		// se := unittest.ServiceEventsFixture(2)
		se := unittest.BlockEventsFixture(header, 8)
		tes := unittest.TransactionResultsFixture(4)

		err = headers.Store(&header)
		require.NoError(t, err)

		// save execution results
		err = es.SaveExecutionResults(
			context.Background(),
			&header,
			endState,
			cdp,
			executionReceipt,
			[]flow.EventsList{blockEvents.Events},
			se.Events,
			tes,
		)

		require.NoError(t, err)

		// remove execution results
		err = removeForBlockID(
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

		// re execute result
		err = es.SaveExecutionResults(
			context.Background(),
			&header,
			endState,
			cdp,
			executionReceipt,
			[]flow.EventsList{blockEvents.Events},
			se.Events,
			tes,
		)

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
		err := bootstrapper.BootstrapExecutionDatabase(db, unittest.StateCommitmentFixture(), &genesis)
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

		err = headers.Store(&genesis)
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
		)
		require.NotNil(t, es)

		// prepare data
		header := unittest.BlockHeaderWithParentFixture(&genesis) // make sure the height is higher than genesis
		executionReceipt := unittest.ExecutionReceiptFixture()
		executionReceipt.ExecutionResult.BlockID = header.ID()
		cdp := make([]*flow.ChunkDataPack, 0, len(executionReceipt.ExecutionResult.Chunks))
		for _, chunk := range executionReceipt.ExecutionResult.Chunks {
			cdp = append(cdp, unittest.ChunkDataPackFixture(chunk.ID()))
		}
		endState, err := executionReceipt.ExecutionResult.FinalStateCommitment()
		require.NoError(t, err)
		blockEvents := unittest.BlockEventsFixture(header, 3)
		// se := unittest.ServiceEventsFixture(2)
		se := unittest.BlockEventsFixture(header, 8)
		tes := unittest.TransactionResultsFixture(4)

		err = headers.Store(&header)
		require.NoError(t, err)

		// save execution results
		err = es.SaveExecutionResults(
			context.Background(),
			&header,
			endState,
			cdp,
			executionReceipt,
			[]flow.EventsList{blockEvents.Events},
			se.Events,
			tes,
		)

		require.NoError(t, err)

		// remove execution results
		err = removeForBlockID(
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

		executionReceipt2 := unittest.ExecutionReceiptFixture()
		executionReceipt2.ExecutionResult.BlockID = header.ID()
		cdp2 := make([]*flow.ChunkDataPack, 0, len(executionReceipt2.ExecutionResult.Chunks))
		for _, chunk := range executionReceipt.ExecutionResult.Chunks {
			cdp2 = append(cdp2, unittest.ChunkDataPackFixture(chunk.ID()))
		}
		endState2, err := executionReceipt2.ExecutionResult.FinalStateCommitment()
		require.NoError(t, err)

		// re execute result
		err = es.SaveExecutionResults(
			context.Background(),
			&header,
			endState2,
			cdp2,
			executionReceipt2,
			[]flow.EventsList{blockEvents.Events},
			se.Events,
			tes,
		)

		require.NoError(t, err)
	})
}
