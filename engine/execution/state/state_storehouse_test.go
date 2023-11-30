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

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	led "github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
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
	// block 10 is executed block
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

		// block 11 is the block to be executed
		block11 := finalized.BlockAtHeight(11)
		header11 := block11.Header
		sc10 := flow.StateCommitment(l.InitialState())

		reg1 := unittest.MakeOwnerReg("fruit", "apple")
		reg2 := unittest.MakeOwnerReg("vegetable", "carrot")
		executionSnapshot := &snapshot.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				reg1.Key: reg1.Value,
				reg2.Key: reg2.Value,
			},
			Meter: meter.NewMeter(meter.DefaultParameters()),
		}

		// create Block 11's end statecommitment
		sc2, update, sc2Snapshot, err := state.CommitDelta(l, executionSnapshot,
			storehouse.NewExecutingBlockSnapshot(state.NewLedgerStorageSnapshot(l, sc10), sc10))
		require.NoError(t, err)

		// validate new snapshot
		val, err := sc2Snapshot.Get(reg1.Key)
		require.NoError(t, err)
		require.Equal(t, reg1.Value, val)

		val, err = sc2Snapshot.Get(reg2.Key)
		require.NoError(t, err)
		require.Equal(t, reg2.Value, val)

		validateUpdate(t, update, sc10, executionSnapshot)

		// validate storage snapshot
		completeBlock := &entity.ExecutableBlock{
			Block:               block11,
			CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{},
			StartState:          &sc10,
			Executing:           false,
		}

		computationResult := makeComputationResult(t, completeBlock, executionSnapshot, sc2)

		// save result and store registers
		require.NoError(t, es.SaveExecutionResults(context.Background(), computationResult))

		storageSnapshot := es.NewStorageSnapshot(sc2, header11.ID(), header11.Height)

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

func validateUpdate(t *testing.T, update *led.TrieUpdate, commit flow.StateCommitment, executionSnapshot *snapshot.ExecutionSnapshot) {
	require.Equal(t, commit[:], update.RootHash[:])
	require.Len(t, update.Paths, len(executionSnapshot.WriteSet))
	require.Len(t, update.Payloads, len(executionSnapshot.WriteSet))

	regs := executionSnapshot.UpdatedRegisters()
	for i, reg := range regs {
		key := convert.RegisterIDToLedgerKey(reg.Key)
		path, err := pathfinder.KeyToPath(key, ledger.DefaultPathFinderVersion)
		require.NoError(t, err)

		require.Equal(t, path, update.Paths[i])
		require.Equal(t, led.Value(reg.Value), update.Payloads[i].Value())
	}
}

func makeComputationResult(
	t *testing.T,
	completeBlock *entity.ExecutableBlock,
	executionSnapshot *snapshot.ExecutionSnapshot,
	commit flow.StateCommitment,
) *execution.ComputationResult {

	computationResult := execution.NewEmptyComputationResult(completeBlock)
	numberOfChunks := 1
	ceds := make([]*execution_data.ChunkExecutionData, numberOfChunks)
	ceds[0] = unittest.ChunkExecutionDataFixture(t, 1024)
	computationResult.CollectionExecutionResultAt(0).UpdateExecutionSnapshot(executionSnapshot)
	computationResult.AppendCollectionAttestationResult(
		*completeBlock.StartState,
		commit,
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
	return computationResult
}
