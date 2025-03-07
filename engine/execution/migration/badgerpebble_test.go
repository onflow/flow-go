package migration

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMigrateLastSealedExecutedResultToPebble(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
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

		// read the saved results before migration
		badgerResults, badgerCommits := createStores(badgerimpl.ToDB(bdb))
		bresult, bcommit, err := readResultsForBlock(
			header.ID(), badgerResults, badgerCommits)
		require.NoError(t, err)

		// mock that the executed block is the last executed and sealed block
		ps := new(protocolmock.State)
		mockSnapshot := createSnapshot(header)
		params := new(protocolmock.Params)
		params.On("SporkID").Return(mainnet26SporkID)
		ps.On("Params").Return(params)
		ps.On("AtHeight", mock.Anything).Return(
			func(height uint64) protocol.Snapshot {
				if height == header.Height {
					return mockSnapshot
				}
				return invalid.NewSnapshot(fmt.Errorf("invalid height: %v", height))
			})
		ps.On("AtBlockID", mock.Anything).Return(
			func(blockID flow.Identifier) protocol.Snapshot {
				if blockID == header.ID() {
					return mockSnapshot
				} else if blockID == genesis.ID() {
					return createSnapshot(genesis)
				}
				return invalid.NewSnapshot(fmt.Errorf("invalid block: %v", blockID))
			})
		ps.On("Sealed", mock.Anything).Return(mockSnapshot)

		// run the migration
		require.NoError(t, MigrateLastSealedExecutedResultToPebble(unittest.Logger(), bdb, pdb, ps, rootSeal))

		// read the migrated results after migration
		pebbleResults, pebbleCommits := createStores(pebbleimpl.ToDB(pdb))
		presult, pcommit, err := readResultsForBlock(
			header.ID(), pebbleResults, pebbleCommits)
		require.NoError(t, err)

		// compare the migrated results
		require.Equal(t, bresult, presult)
		require.Equal(t, bcommit, pcommit)
	})
}

func createSnapshot(head *flow.Header) protocol.Snapshot {
	snapshot := &protocolmock.Snapshot{}
	snapshot.On("Head").Return(
		func() *flow.Header {
			return head
		},
		nil,
	)
	return snapshot
}
