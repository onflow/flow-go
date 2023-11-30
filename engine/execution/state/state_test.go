package state_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	led "github.com/onflow/flow-go/ledger"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	storageerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func prepareTest(f func(t *testing.T, es state.ExecutionState, l *ledger.Ledger, headers *storage.Headers, commits *storage.Commits)) func(*testing.T) {
	return func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(badgerDB *badger.DB) {
			metricsCollector := &metrics.NoopCollector{}
			diskWal := &fixtures.NoopWAL{}
			ls, err := ledger.NewLedger(diskWal, 100, metricsCollector, zerolog.Nop(), ledger.DefaultPathFinderVersion)
			require.NoError(t, err)
			compactor := fixtures.NewNoopCompactor(ls)
			<-compactor.Ready()
			defer func() {
				<-ls.Done()
				<-compactor.Done()
			}()

			stateCommitments := storage.NewCommits(t)
			headers := storage.NewHeaders(t)
			blocks := storage.NewBlocks(t)
			collections := storage.NewCollections(t)
			events := storage.NewEvents(t)
			serviceEvents := storage.NewServiceEvents(t)
			txResults := storage.NewTransactionResults(t)
			chunkDataPacks := storage.NewChunkDataPacks(t)
			results := storage.NewExecutionResults(t)
			myReceipts := storage.NewMyExecutionReceipts(t)

			es := state.NewExecutionState(
				ls, stateCommitments, blocks, headers, collections, chunkDataPacks, results, myReceipts, events, serviceEvents, txResults, badgerDB, trace.NewNoopTracer(),
				nil,
				false,
			)

			f(t, es, ls, headers, stateCommitments)
		})
	}
}

func TestExecutionStateWithTrieStorage(t *testing.T) {
	t.Run("commit write and read new state", prepareTest(func(
		t *testing.T, es state.ExecutionState, l *ledger.Ledger, headers *storage.Headers, stateCommitments *storage.Commits) {
		header1 := unittest.BlockHeaderFixture()
		sc1 := flow.StateCommitment(l.InitialState())

		reg1 := unittest.MakeOwnerReg("fruit", "apple")
		reg2 := unittest.MakeOwnerReg("vegetable", "carrot")
		executionSnapshot := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: reg1.Value,
				reg2.Key: reg2.Value,
			},
		}

		sc2, update, sc2Snapshot, err := state.CommitDelta(l, executionSnapshot,
			storehouse.NewExecutingBlockSnapshot(state.NewLedgerStorageSnapshot(l, sc1), sc1))
		assert.NoError(t, err)

		// validate new snapshot
		val, err := sc2Snapshot.Get(reg1.Key)
		require.NoError(t, err)
		require.Equal(t, reg1.Value, val)

		val, err = sc2Snapshot.Get(reg2.Key)
		require.NoError(t, err)
		require.Equal(t, reg2.Value, val)

		assert.Equal(t, sc1[:], update.RootHash[:])
		assert.Len(t, update.Paths, 2)
		assert.Len(t, update.Payloads, 2)

		// validate sc2
		require.Equal(t, sc2, sc2Snapshot.Commitment())

		key1 := convert.RegisterIDToLedgerKey(reg1.Key)
		path1, err := pathfinder.KeyToPath(key1, ledger.DefaultPathFinderVersion)
		assert.NoError(t, err)

		key2 := convert.RegisterIDToLedgerKey(reg2.Key)
		path2, err := pathfinder.KeyToPath(key2, ledger.DefaultPathFinderVersion)
		assert.NoError(t, err)

		// validate update
		assert.Equal(t, path1, update.Paths[0])
		assert.Equal(t, path2, update.Paths[1])

		k1, err := update.Payloads[0].Key()
		require.NoError(t, err)

		k2, err := update.Payloads[1].Key()
		require.NoError(t, err)

		assert.Equal(t, key1, k1)
		assert.Equal(t, key2, k2)

		assert.Equal(t, []byte("apple"), []byte(update.Payloads[0].Value()))
		assert.Equal(t, []byte("carrot"), []byte(update.Payloads[1].Value()))

		header2 := unittest.BlockHeaderWithParentFixture(header1)
		storageSnapshot := es.NewStorageSnapshot(sc2, header2.ID(), header2.Height)

		b1, err := storageSnapshot.Get(reg1.Key)
		assert.NoError(t, err)
		b2, err := storageSnapshot.Get(reg2.Key)
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)

		// verify has state
		require.True(t, l.HasState(led.State(sc2)))
		require.False(t, l.HasState(led.State(unittest.StateCommitmentFixture())))
	}))

	t.Run("commit write and read previous state", prepareTest(func(
		t *testing.T, es state.ExecutionState, l *ledger.Ledger, headers *storage.Headers, stateCommitments *storage.Commits) {
		header1 := unittest.BlockHeaderFixture()
		sc1 := flow.StateCommitment(l.InitialState())

		reg1 := unittest.MakeOwnerReg("fruit", "apple")
		executionSnapshot1 := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: reg1.Value,
			},
		}

		sc2, _, sc2Snapshot, err := state.CommitDelta(l, executionSnapshot1,
			storehouse.NewExecutingBlockSnapshot(state.NewLedgerStorageSnapshot(l, sc1), sc1),
		)
		assert.NoError(t, err)

		// update value and get resulting state commitment
		executionSnapshot2 := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: flow.RegisterValue("orange"),
			},
		}

		sc3, _, _, err := state.CommitDelta(l, executionSnapshot2, sc2Snapshot)
		assert.NoError(t, err)

		header2 := unittest.BlockHeaderWithParentFixture(header1)
		// create a view for previous state version
		storageSnapshot3 := es.NewStorageSnapshot(sc2, header2.ID(), header2.Height)

		header3 := unittest.BlockHeaderWithParentFixture(header1)
		// create a view for new state version
		storageSnapshot4 := es.NewStorageSnapshot(sc3, header3.ID(), header3.Height)

		// header2 and header3 are different blocks
		assert.True(t, header2.ID() != (header3.ID()))

		// fetch the value at both versions
		b1, err := storageSnapshot3.Get(reg1.Key)
		assert.NoError(t, err)

		b2, err := storageSnapshot4.Get(reg1.Key)
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Equal(t, flow.RegisterValue("orange"), b2)
	}))

	t.Run("commit delta and read new state", prepareTest(func(
		t *testing.T, es state.ExecutionState, l *ledger.Ledger, headers *storage.Headers, stateCommitments *storage.Commits) {
		header1 := unittest.BlockHeaderFixture()
		sc1 := flow.StateCommitment(l.InitialState())

		reg1 := unittest.MakeOwnerReg("fruit", "apple")
		reg2 := unittest.MakeOwnerReg("vegetable", "carrot")
		// set initial value
		executionSnapshot1 := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: reg1.Value,
				reg2.Key: reg2.Value,
			},
		}

		sc2, _, sc2Snapshot, err := state.CommitDelta(l, executionSnapshot1,
			storehouse.NewExecutingBlockSnapshot(state.NewLedgerStorageSnapshot(l, sc1), sc1),
		)
		assert.NoError(t, err)

		// update value and get resulting state commitment
		executionSnapshot2 := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: nil,
			},
		}

		sc3, _, _, err := state.CommitDelta(l, executionSnapshot2, sc2Snapshot)
		assert.NoError(t, err)

		header2 := unittest.BlockHeaderWithParentFixture(header1)
		// create a view for previous state version
		storageSnapshot3 := es.NewStorageSnapshot(sc2, header2.ID(), header2.Height)

		header3 := unittest.BlockHeaderWithParentFixture(header2)
		// create a view for new state version
		storageSnapshot4 := es.NewStorageSnapshot(sc3, header3.ID(), header3.Height)

		// fetch the value at both versions
		b1, err := storageSnapshot3.Get(reg1.Key)
		assert.NoError(t, err)

		b2, err := storageSnapshot4.Get(reg1.Key)
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Empty(t, b2)
	}))

	t.Run("commit delta and persist state commit for the second time should be OK", prepareTest(func(
		t *testing.T, es state.ExecutionState, l *ledger.Ledger, headers *storage.Headers, stateCommitments *storage.Commits) {
		sc1 := flow.StateCommitment(l.InitialState())

		reg1 := unittest.MakeOwnerReg("fruit", "apple")
		reg2 := unittest.MakeOwnerReg("vegetable", "carrot")
		// set initial value
		executionSnapshot1 := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: reg1.Value,
				reg2.Key: reg2.Value,
			},
		}

		sc2, _, _, err := state.CommitDelta(l, executionSnapshot1,
			storehouse.NewExecutingBlockSnapshot(state.NewLedgerStorageSnapshot(l, sc1), sc1),
		)
		assert.NoError(t, err)

		// committing for the second time should be OK
		sc2Same, _, _, err := state.CommitDelta(l, executionSnapshot1,
			storehouse.NewExecutingBlockSnapshot(state.NewLedgerStorageSnapshot(l, sc1), sc1),
		)
		assert.NoError(t, err)

		require.Equal(t, sc2, sc2Same)
	}))

	t.Run("commit write and create snapshot", prepareTest(func(
		t *testing.T, es state.ExecutionState, l *ledger.Ledger, headers *storage.Headers, stateCommitments *storage.Commits) {
		header1 := unittest.BlockHeaderFixture()
		header2 := unittest.BlockHeaderWithParentFixture(header1)
		sc1 := flow.StateCommitment(l.InitialState())

		reg1 := unittest.MakeOwnerReg("fruit", "apple")
		reg2 := unittest.MakeOwnerReg("vegetable", "carrot")
		executionSnapshot := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: reg1.Value,
				reg2.Key: reg2.Value,
			},
		}

		sc2, _, _, err := state.CommitDelta(l, executionSnapshot,
			storehouse.NewExecutingBlockSnapshot(state.NewLedgerStorageSnapshot(l, sc1), sc1))
		assert.NoError(t, err)

		// test CreateStorageSnapshot for known and executed block
		headers.On("ByBlockID", header2.ID()).Return(header2, nil)
		stateCommitments.On("ByBlockID", header2.ID()).Return(sc2, nil)
		snapshot2, h2, err := es.CreateStorageSnapshot(header2.ID())
		require.NoError(t, err)
		require.Equal(t, header2.ID(), h2.ID())

		val, err := snapshot2.Get(reg1.Key)
		require.NoError(t, err)
		require.Equal(t, val, reg1.Value)

		val, err = snapshot2.Get(reg2.Key)
		require.NoError(t, err)
		require.Equal(t, val, reg2.Value)

		// test CreateStorageSnapshot for unknown block
		unknown := unittest.BlockHeaderFixture()
		headers.On("ByBlockID", unknown.ID()).Return(nil, fmt.Errorf("unknown: %w", storageerr.ErrNotFound))
		_, _, err = es.CreateStorageSnapshot(unknown.ID())
		require.Error(t, err)
		require.True(t, errors.Is(err, storageerr.ErrNotFound))

		// test CreateStorageSnapshot for known and unexecuted block
		unexecuted := unittest.BlockHeaderFixture()
		headers.On("ByBlockID", unexecuted.ID()).Return(unexecuted, nil)
		stateCommitments.On("ByBlockID", unexecuted.ID()).Return(nil, fmt.Errorf("not found: %w", storageerr.ErrNotFound))
		_, _, err = es.CreateStorageSnapshot(unexecuted.ID())
		require.Error(t, err)
		require.True(t, errors.Is(err, state.ErrNotExecuted))

		// test CreateStorageSnapshot for pruned block
		pruned := unittest.BlockHeaderFixture()
		prunedState := unittest.StateCommitmentFixture()
		headers.On("ByBlockID", pruned.ID()).Return(pruned, nil)
		stateCommitments.On("ByBlockID", pruned.ID()).Return(prunedState, nil)
		_, _, err = es.CreateStorageSnapshot(pruned.ID())
		require.Error(t, err)
		require.True(t, errors.Is(err, state.ErrExecutionStatePruned))
	}))

}
