package state_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ledger2 "github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	st "github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"

	"github.com/onflow/flow-go/engine/execution/state"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/mocks"
	"github.com/onflow/flow-go/utils/unittest"
)

func prepareTest(f func(t *testing.T, es state.ExecutionState, l *ledger.Ledger)) func(*testing.T) {
	return func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(badgerDB *badger.DB) {
			metricsCollector := &metrics.NoopCollector{}
			diskWal := &fixtures.NoopWAL{}
			ls, err := ledger.NewLedger(diskWal, 100, metricsCollector, zerolog.Nop(), ledger.DefaultPathFinderVersion)
			require.NoError(t, err)

			ctrl := gomock.NewController(t)

			stateCommitments := mocks.NewMockCommits(ctrl)
			blocks := mocks.NewMockBlocks(ctrl)
			headers := mocks.NewMockHeaders(ctrl)
			collections := mocks.NewMockCollections(ctrl)
			events := mocks.NewMockEvents(ctrl)
			serviceEvents := mocks.NewMockServiceEvents(ctrl)
			txResults := mocks.NewMockTransactionResults(ctrl)

			stateCommitment := ls.InitialState()

			stateCommitments.EXPECT().ByBlockID(gomock.Any()).Return(flow.StateCommitment(stateCommitment), nil)

			chunkDataPacks := new(storage.ChunkDataPacks)

			results := new(storage.ExecutionResults)
			receipts := new(storage.ExecutionReceipts)
			myReceipts := new(storage.MyExecutionReceipts)

			es := state.NewExecutionState(
				ls, stateCommitments, blocks, headers, collections, chunkDataPacks, results, receipts, myReceipts, events, serviceEvents, txResults, badgerDB, trace.NewNoopTracer(),
			)

			f(t, es, ls)
		})
	}
}

func TestExecutionStateWithTrieStorage(t *testing.T) {
	registerID1 := "fruit"

	registerID2 := "vegetable"

	t.Run("commit write and read new state", prepareTest(func(t *testing.T, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(t, err)

		view1 := es.NewView(sc1)

		err = view1.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = view1.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		assert.NoError(t, err)

		sc2, update, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		assert.Equal(t, sc1[:], update.RootHash[:])
		assert.Len(t, update.Paths, 2)
		assert.Len(t, update.Payloads, 2)

		key1 := ledger2.NewKey([]ledger2.KeyPart{ledger2.NewKeyPart(0, []byte(registerID1)), ledger2.NewKeyPart(1, []byte("")), ledger2.NewKeyPart(2, []byte(""))})
		path1, err := pathfinder.KeyToPath(key1, ledger.DefaultPathFinderVersion)
		assert.NoError(t, err)

		key2 := ledger2.NewKey([]ledger2.KeyPart{ledger2.NewKeyPart(0, []byte(registerID2)), ledger2.NewKeyPart(1, []byte("")), ledger2.NewKeyPart(2, []byte(""))})
		path2, err := pathfinder.KeyToPath(key2, ledger.DefaultPathFinderVersion)
		assert.NoError(t, err)

		assert.Equal(t, path1, update.Paths[0])
		assert.Equal(t, path2, update.Paths[1])

		assert.Equal(t, key1, update.Payloads[0].Key)
		assert.Equal(t, key2, update.Payloads[1].Key)

		assert.Equal(t, []byte("apple"), []byte(update.Payloads[0].Value))
		assert.Equal(t, []byte("carrot"), []byte(update.Payloads[1].Value))

		view2 := es.NewView(sc2)

		b1, err := view2.Get(registerID1, "", "")
		assert.NoError(t, err)
		b2, err := view2.Get(registerID2, "", "")
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	}))

	t.Run("commit write and read previous state", prepareTest(func(t *testing.T, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(t, err)

		view1 := es.NewView(sc1)

		err = view1.Set(registerID1, "", "", []byte("apple"))
		assert.NoError(t, err)
		sc2, _, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		// update value and get resulting state commitment
		view2 := es.NewView(sc2)
		err = view2.Set(registerID1, "", "", []byte("orange"))
		assert.NoError(t, err)

		sc3, _, err := state.CommitDelta(l, view2.Delta(), sc2)
		assert.NoError(t, err)

		// create a view for previous state version
		view3 := es.NewView(sc2)

		// create a view for new state version
		view4 := es.NewView(sc3)

		// fetch the value at both versions
		b1, err := view3.Get(registerID1, "", "")
		assert.NoError(t, err)

		b2, err := view4.Get(registerID1, "", "")
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Equal(t, flow.RegisterValue("orange"), b2)
	}))

	t.Run("commit delete and read new state", prepareTest(func(t *testing.T, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(t, err)

		// set initial value
		view1 := es.NewView(sc1)
		err = view1.Set(registerID1, "", "", []byte("apple"))
		assert.NoError(t, err)
		err = view1.Set(registerID2, "", "", []byte("apple"))
		assert.NoError(t, err)

		sc2, _, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		// update value and get resulting state commitment
		view2 := es.NewView(sc2)
		err = view2.Delete(registerID1, "", "")
		assert.NoError(t, err)

		sc3, _, err := state.CommitDelta(l, view2.Delta(), sc2)
		assert.NoError(t, err)

		// create a view for previous state version
		view3 := es.NewView(sc2)

		// create a view for new state version
		view4 := es.NewView(sc3)

		// fetch the value at both versions
		b1, err := view3.Get(registerID1, "", "")
		assert.NoError(t, err)

		b2, err := view4.Get(registerID1, "", "")
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Empty(t, b2)
	}))

	t.Run("commit delta and persist state commit for the second time should be OK", prepareTest(func(t *testing.T, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(t, err)

		// set initial value
		view1 := es.NewView(sc1)
		err = view1.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = view1.Set(registerID2, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		sc2, _, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		// committing for the second time should be OK
		sc2Same, _, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		require.Equal(t, sc2, sc2Same)
	}))

}

func TestChunkDataPackStorageAndPruge(t *testing.T) {

	ls, err := ledger.NewLedger(&fixtures.NoopWAL{}, 100, &metrics.NoopCollector{}, zerolog.Nop(), ledger.DefaultPathFinderVersion)
	require.NoError(t, err)

	db, _ := unittest.TempBadgerDB(t)

	blks := unittest.BlockFixtures(2)
	prevBlock := blks[0]
	prevBlock.Header.View = 1
	block := blks[1]
	block.Header.View = 2
	header := block.Header

	headers := badgerstorage.NewHeaders(&metrics.NoopCollector{}, db)
	index := badgerstorage.NewIndex(&metrics.NoopCollector{}, db)
	guarantees := badgerstorage.NewGuarantees(&metrics.NoopCollector{}, db, 100)
	seals := badgerstorage.NewSeals(&metrics.NoopCollector{}, db)
	results := badgerstorage.NewExecutionResults(&metrics.NoopCollector{}, db)
	receipts := badgerstorage.NewExecutionReceipts(&metrics.NoopCollector{}, db, results, 100)
	payloads := badgerstorage.NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := badgerstorage.NewBlocks(db, headers, payloads)
	stateCommitments := badgerstorage.NewCommits(&metrics.NoopCollector{}, db)
	transactions := badgerstorage.NewTransactions(&metrics.NoopCollector{}, db)
	collections := badgerstorage.NewCollections(db, transactions)
	chunkDataPacks := badgerstorage.NewChunkDataPacks(&metrics.NoopCollector{}, db, collections, 100)
	myReceipts := badgerstorage.NewMyExecutionReceipts(&metrics.NoopCollector{}, db, receipts)
	events := badgerstorage.NewEvents(&metrics.NoopCollector{}, db)
	serEvents := badgerstorage.NewServiceEvents(&metrics.NoopCollector{}, db)
	txResults := badgerstorage.NewTransactionResults(&metrics.NoopCollector{}, db, 100)

	es := state.NewExecutionState(
		ls, stateCommitments, blocks, headers, collections, chunkDataPacks, results, receipts, myReceipts, events, serEvents, txResults, db, trace.NewNoopTracer(),
	)

	err = blocks.Store(block)
	assert.NoError(t, err)

	operation.RetryOnConflict(db.Update, func(txn *badger.Txn) error {
		err = operation.InsertExecutedBlock(block.ID())(txn)
		assert.NoError(t, err)
		return nil
	})

	stateCommit := unittest.StateCommitmentFixture()

	result := unittest.ExecutionResultFixture(unittest.WithBlock(block))

	chdps := unittest.ChunkDataPacksFixture(result.Chunks.Len())
	for i, ch := range result.Chunks {
		chdps[i].ChunkID = ch.ID()
		err = collections.Store(chdps[i].Collection)
		assert.NoError(t, err)
	}

	receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(result))

	err = es.SaveExecutionResults(context.Background(), header, stateCommit, chdps, receipt, nil, nil, nil)
	assert.NoError(t, err)

	for _, ch := range result.Chunks {
		_, err := es.ChunkDataPackByChunkID(context.Background(), ch.ID())
		assert.NoError(t, err)
	}

	err = es.PurgeChunkDataPacksByBlockID(context.Background(), header.ID())
	assert.NoError(t, err)

	for _, ch := range result.Chunks {
		_, err := es.ChunkDataPackByChunkID(context.Background(), ch.ID())
		assert.True(t, errors.Is(err, st.ErrNotFound))
	}

}
