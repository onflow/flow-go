package state_test

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	led "github.com/onflow/flow-go/ledger"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

func prepareStorehouseTest(f func(t *testing.T, es state.ExecutionState, l *ledger.Ledger, headers *storage.Headers, commits *storage.Commits, finalized *testutil.MockFinalizedReader)) func(*testing.T) {
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
			stateCommitments.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			headers := storage.NewHeaders(t)
			blocks := storage.NewBlocks(t)
			collections := storage.NewCollections(t)
			events := storage.NewEvents(t)
			events.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			serviceEvents := storage.NewServiceEvents(t)
			serviceEvents.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			txResults := storage.NewTransactionResults(t)
			txResults.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			chunkDataPacks := storage.NewChunkDataPacks(t)
			chunkDataPacks.On("Store", mock.Anything).Return(nil)
			results := storage.NewExecutionResults(t)
			results.On("BatchStore", mock.Anything, mock.Anything).Return(nil)
			results.On("BatchIndex", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			myReceipts := storage.NewMyExecutionReceipts(t)
			myReceipts.On("BatchStoreMyReceipt", mock.Anything, mock.Anything).Return(nil)

			withRegisterStore(t, func(t *testing.T,
				rs *storehouse.RegisterStore,
				diskStore execution.OnDiskRegisterStore,
				finalized *testutil.MockFinalizedReader,
				rootHeight uint64,
				endHeight uint64,
				finalizedHeaders map[uint64]*flow.Header,
			) {

				rootID, err := finalized.FinalizedBlockIDAtHeight(10)
				require.NoError(t, err)
				require.NoError(t,
					badgerDB.Update(operation.InsertExecutedBlock(rootID)),
				)

				metrics := metrics.NewNoopCollector()
				headersDB := badgerstorage.NewHeaders(metrics, badgerDB)
				require.NoError(t, headersDB.Store(finalizedHeaders[10]))

				es := state.NewExecutionState(
					ls, stateCommitments, blocks, headers, collections, chunkDataPacks, results, myReceipts, events, serviceEvents, txResults, badgerDB, trace.NewNoopTracer(),
					rs,
					true,
				)

				f(t, es, ls, headers, stateCommitments, finalized)

			})
		})
	}
}

func withRegisterStore(t *testing.T, fn func(
	t *testing.T,
	rs *storehouse.RegisterStore,
	diskStore execution.OnDiskRegisterStore,
	finalized *testutil.MockFinalizedReader,
	rootHeight uint64,
	endHeight uint64,
	headers map[uint64]*flow.Header,
)) {
	pebble.RunWithRegistersStorageAtInitialHeights(t, 10, 10, func(diskStore *pebble.Registers) {
		log := unittest.Logger()
		var wal execution.ExecutedFinalizedWAL
		finalized, headerByHeight, highest := testutil.NewMockFinalizedReader(10, 100)
		rs, err := storehouse.NewRegisterStore(diskStore, wal, finalized, log)
		require.NoError(t, err)
		fn(t, rs, diskStore, finalized, 10, highest, headerByHeight)
	})
}

func TestExecutionStateWithStorehouse(t *testing.T) {
	t.Run("commit write and read new state", prepareStorehouseTest(func(
		t *testing.T, es state.ExecutionState, l *ledger.Ledger, headers *storage.Headers, stateCommitments *storage.Commits, finalized *testutil.MockFinalizedReader) {

		// Block 1 height 11, start statecommitment sc1
		block1 := finalized.BlockAtHeight(11)
		header1 := block1.Header
		sc1 := flow.StateCommitment(l.InitialState())

		reg1 := unittest.MakeOwnerReg("fruit", "apple")
		reg2 := unittest.MakeOwnerReg("vegetable", "carrot")
		executionSnapshot := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: reg1.Value,
				reg2.Key: reg2.Value,
			},
			Meter: meter.NewMeter(meter.DefaultParameters()),
		}

		// create Block 1's end statecommitment
		sc2, update, sc2Snapshot, err := state.CommitDelta(l, executionSnapshot,
			storehouse.NewExecutingBlockSnapshot(state.NewLedgerStorageSnapshot(l, sc1), sc1))
		require.NoError(t, err)

		// validate new snapshot
		val, err := sc2Snapshot.Get(reg1.Key)
		require.NoError(t, err)
		require.Equal(t, reg1.Value, val)

		val, err = sc2Snapshot.Get(reg2.Key)
		require.NoError(t, err)
		require.Equal(t, reg2.Value, val)

		require.Equal(t, sc1[:], update.RootHash[:])
		require.Len(t, update.Paths, 2)
		require.Len(t, update.Payloads, 2)

		// validate sc2
		require.Equal(t, sc2, sc2Snapshot.Commitment())

		key1 := convert.RegisterIDToLedgerKey(reg1.Key)
		path1, err := pathfinder.KeyToPath(key1, ledger.DefaultPathFinderVersion)
		require.NoError(t, err)

		key2 := convert.RegisterIDToLedgerKey(reg2.Key)
		path2, err := pathfinder.KeyToPath(key2, ledger.DefaultPathFinderVersion)
		require.NoError(t, err)

		// validate update
		require.Equal(t, path1, update.Paths[0])
		require.Equal(t, path2, update.Paths[1])

		k1, err := update.Payloads[0].Key()
		require.NoError(t, err)

		k2, err := update.Payloads[1].Key()
		require.NoError(t, err)

		require.Equal(t, key1, k1)
		require.Equal(t, key2, k2)

		require.Equal(t, []byte("apple"), []byte(update.Payloads[0].Value()))
		require.Equal(t, []byte("carrot"), []byte(update.Payloads[1].Value()))

		// validate storage snapshot

		completeBlock := &entity.ExecutableBlock{
			Block:               block1,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{},
			StartState:          &sc1,
			Executing:           false,
		}

		computationResult := execution.NewEmptyComputationResult(completeBlock)
		numberOfChunks := 1
		ceds := make([]*execution_data.ChunkExecutionData, numberOfChunks)
		ceds[0] = unittest.ChunkExecutionDataFixture(t, 1024)
		computationResult.CollectionExecutionResultAt(0).UpdateExecutionSnapshot(executionSnapshot)
		computationResult.AppendCollectionAttestationResult(
			*completeBlock.StartState,
			sc2,
			nil,
			unittest.IdentifierFixture(),
			ceds[0],
		)

		bed := unittest.BlockExecutionDataFixture(
			unittest.WithBlockExecutionDataBlockID(completeBlock.Block.ID()),
			unittest.WithChunkExecutionDatas(ceds...),
		)

		executionDataID, err := execution_data.CalculateID(context.Background(), bed, execution_data.DefaultSerializer)
		require.NoError(t, err)

		executionResult := flow.NewExecutionResult(
			unittest.IdentifierFixture(),
			completeBlock.ID(),
			computationResult.AllChunks(),
			flow.ServiceEventList{},
			executionDataID)

		computationResult.BlockAttestationResult.BlockExecutionResult.ExecutionDataRoot = &flow.BlockExecutionDataRoot{
			BlockID:               completeBlock.ID(),
			ChunkExecutionDataIDs: []cid.Cid{flow.IdToCid(unittest.IdentifierFixture())},
		}

		computationResult.ExecutionReceipt = &flow.ExecutionReceipt{
			ExecutionResult:   *executionResult,
			Spocks:            make([]crypto.Signature, numberOfChunks),
			ExecutorSignature: crypto.Signature{},
		}

		// save result and store registers
		require.NoError(t, es.SaveExecutionResults(context.Background(), computationResult))

		storageSnapshot := es.NewStorageSnapshot(sc2, header1.ID(), header1.Height)

		// validate the storage snapshot has the registers
		b1, err := storageSnapshot.Get(reg1.Key)
		require.NoError(t, err)
		b2, err := storageSnapshot.Get(reg2.Key)
		require.NoError(t, err)

		require.Equal(t, flow.RegisterValue("apple"), b1)
		require.Equal(t, flow.RegisterValue("carrot"), b2)

		// verify has state
		require.True(t, l.HasState(led.State(sc2)))
		require.False(t, l.HasState(led.State(unittest.StateCommitmentFixture())))
	}))

}
