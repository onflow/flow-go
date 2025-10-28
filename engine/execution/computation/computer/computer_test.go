package computer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	computermock "github.com/onflow/flow-go/engine/execution/computation/computer/mock"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	fvmmock "github.com/onflow/flow-go/fvm/mock"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	testMaxConcurrency = 1
)

func incStateCommitment(startState flow.StateCommitment) flow.StateCommitment {
	endState := flow.StateCommitment(startState)
	endState[0] += 1
	return endState
}

type fakeCommitter struct {
	callCount int
}

func (committer *fakeCommitter) CommitView(
	view *snapshot.ExecutionSnapshot,
	baseStorageSnapshot execution.ExtendableStorageSnapshot,
) (
	flow.StateCommitment,
	[]byte,
	*ledger.TrieUpdate,
	execution.ExtendableStorageSnapshot,
	error,
) {
	// Add proof generation timing logs
	fmt.Printf("🔒 PROOF GENERATION for Collection %d STARTED at %v\n",
		committer.callCount, time.Now())

	committer.callCount++

	// Simulate proof generation work to make timing visible
	time.Sleep(5 * time.Millisecond)

	startState := baseStorageSnapshot.Commitment()
	endState := incStateCommitment(startState)

	reg := unittest.MakeOwnerReg("key", fmt.Sprintf("%v", committer.callCount))
	regKey := convert.RegisterIDToLedgerKey(reg.Key)
	path, err := pathfinder.KeyToPath(
		regKey,
		complete.DefaultPathFinderVersion,
	)
	if err != nil {
		return flow.DummyStateCommitment, nil, nil, nil, err
	}
	trieUpdate := &ledger.TrieUpdate{
		RootHash: ledger.RootHash(startState),
		Paths: []ledger.Path{
			path,
		},
		Payloads: []*ledger.Payload{
			ledger.NewPayload(regKey, reg.Value),
		},
	}

	newStorageSnapshot := baseStorageSnapshot.Extend(endState, map[flow.RegisterID]flow.RegisterValue{
		reg.Key: reg.Value,
	})

	// Add proof generation completion log
	fmt.Printf("🔓 PROOF GENERATION for Collection %d COMPLETED at %v\n",
		committer.callCount, time.Now())

	return newStorageSnapshot.Commitment(),
		[]byte{byte(committer.callCount)},
		trieUpdate,
		newStorageSnapshot,
		nil
}

func TestBlockExecutor_ExecuteBlock(t *testing.T) {
	rag := &RandomAddressGenerator{}

	executorID := unittest.IdentifierFixture()

	me := new(modulemock.Local)
	me.On("NodeID").Return(executorID)
	me.On("Sign", mock.Anything, mock.Anything).Return(unittest.SignatureFixture(), nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	t.Run("single collection", func(t *testing.T) {

		execCtx := fvm.NewContext()

		vm := &testVM{
			t:                    t,
			eventsPerTransaction: 1,
		}

		committer := &fakeCommitter{
			callCount: 0,
		}

		exemetrics := new(modulemock.ExecutionMetrics)
		exemetrics.On("ExecutionBlockExecuted",
			mock.Anything,
			mock.Anything).
			Return(nil).
			Times(1)

		exemetrics.On("ExecutionCollectionExecuted",
			mock.Anything,
			mock.Anything).
			Return(nil).
			Times(2) // 1 collection + system collection

		exemetrics.On("ExecutionTransactionExecuted",
			mock.Anything,
			mock.MatchedBy(func(arg module.TransactionExecutionResultStats) bool {
				return !arg.Failed // only successful transactions
			}),
			mock.Anything).
			Return(nil).
			Times(2 + 1) // 2 txs in collection + system chunk tx

		exemetrics.On(
			"ExecutionChunkDataPackGenerated",
			mock.Anything,
			mock.Anything).
			Return(nil).
			Times(2) // 1 collection + system collection

		expectedProgramsInCache := 1 // we set one program in the cache
		exemetrics.On(
			"ExecutionBlockCachedPrograms",
			expectedProgramsInCache).
			Return(nil).
			Times(1) // 1 block

		bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
		trackerStorage := mocktracker.NewMockStorage()

		prov := provider.NewProvider(
			zerolog.Nop(),
			metrics.NewNoopCollector(),
			execution_data.DefaultSerializer,
			bservice,
			trackerStorage,
		)

		exe, err := computer.NewBlockComputer(
			vm,
			execCtx,
			exemetrics,
			trace.NewNoopTracer(),
			zerolog.Nop(),
			committer,
			me,
			prov,
			nil,
			testutil.ProtocolStateWithSourceFixture(nil),
			testMaxConcurrency)
		require.NoError(t, err)

		// create a block with 1 collection with 2 transactions
		block := generateBlock(1, 2, rag)

		parentBlockExecutionResultID := unittest.IdentifierFixture()
		result, err := exe.ExecuteBlock(
			context.Background(),
			parentBlockExecutionResultID,
			block,
			nil,
			derived.NewEmptyDerivedBlockData(0))
		assert.NoError(t, err)
		assert.Len(t, result.AllExecutionSnapshots(), 1+1) // +1 system chunk

		require.Equal(t, 2, committer.callCount)

		assert.Equal(t, block.BlockID(), result.BlockExecutionData.BlockID)

		expectedChunk1EndState := incStateCommitment(*block.StartState)
		expectedChunk2EndState := incStateCommitment(expectedChunk1EndState)

		assert.Equal(t, expectedChunk2EndState, result.CurrentEndState())

		assertEventHashesMatch(t, 1+1, result)

		// Verify ExecutionReceipt
		receipt := result.ExecutionReceipt

		assert.Equal(t, executorID, receipt.ExecutorID)
		assert.Equal(
			t,
			parentBlockExecutionResultID,
			receipt.PreviousResultID)
		assert.Equal(t, block.BlockID(), receipt.BlockID)
		assert.NotEqual(t, flow.ZeroID, receipt.ExecutionDataID)

		assert.Len(t, receipt.Chunks, 1+1) // +1 system chunk

		chunk1 := receipt.Chunks[0]

		eventCommits := result.AllEventCommitments()
		assert.Equal(t, block.BlockID(), chunk1.BlockID)
		assert.Equal(t, uint(0), chunk1.CollectionIndex)
		assert.Equal(t, uint64(2), chunk1.NumberOfTransactions)
		assert.Equal(t, eventCommits[0], chunk1.EventCollection)

		assert.Equal(t, *block.StartState, chunk1.StartState)

		assert.NotEqual(t, *block.StartState, chunk1.EndState)
		assert.NotEqual(t, flow.DummyStateCommitment, chunk1.EndState)
		assert.Equal(t, expectedChunk1EndState, chunk1.EndState)

		chunk2 := receipt.Chunks[1]
		assert.Equal(t, block.BlockID(), chunk2.BlockID)
		assert.Equal(t, uint(1), chunk2.CollectionIndex)
		assert.Equal(t, uint64(1), chunk2.NumberOfTransactions)
		assert.Equal(t, eventCommits[1], chunk2.EventCollection)

		assert.Equal(t, expectedChunk1EndState, chunk2.StartState)

		assert.NotEqual(t, *block.StartState, chunk2.EndState)
		assert.NotEqual(t, flow.DummyStateCommitment, chunk2.EndState)
		assert.NotEqual(t, expectedChunk1EndState, chunk2.EndState)
		assert.Equal(t, expectedChunk2EndState, chunk2.EndState)

		// Verify ChunkDataPacks

		chunkDataPacks, err := result.AllChunkDataPacks()
		require.NoError(t, err)
		assert.Len(t, chunkDataPacks, 1+1) // +1 system chunk

		chunkDataPack1 := chunkDataPacks[0]

		assert.Equal(t, chunk1.ID(), chunkDataPack1.ChunkID)
		assert.Equal(t, *block.StartState, chunkDataPack1.StartState)
		assert.Equal(t, []byte{1}, chunkDataPack1.Proof)
		assert.NotNil(t, chunkDataPack1.Collection)

		chunkDataPack2 := chunkDataPacks[1]

		assert.Equal(t, chunk2.ID(), chunkDataPack2.ChunkID)
		assert.Equal(t, chunk2.StartState, chunkDataPack2.StartState)
		assert.Equal(t, []byte{2}, chunkDataPack2.Proof)
		assert.Nil(t, chunkDataPack2.Collection)

		// Verify BlockExecutionData

		assert.Len(t, result.ChunkExecutionDatas, 1+1) // +1 system chunk

		chunkExecutionData1 := result.ChunkExecutionDatas[0]
		assert.Equal(
			t,
			chunkDataPack1.Collection,
			chunkExecutionData1.Collection)
		assert.NotNil(t, chunkExecutionData1.TrieUpdate)
		assert.Equal(t, ledger.RootHash(chunk1.StartState), chunkExecutionData1.TrieUpdate.RootHash)

		chunkExecutionData2 := result.ChunkExecutionDatas[1]
		assert.NotNil(t, chunkExecutionData2.Collection)
		assert.NotNil(t, chunkExecutionData2.TrieUpdate)
		assert.Equal(t, ledger.RootHash(chunk2.StartState), chunkExecutionData2.TrieUpdate.RootHash)

		assert.GreaterOrEqual(t, vm.CallCount(), 3)
		// if every transaction is retried once, then the call count should be
		// (1+totalTransactionCount) /2 * totalTransactionCount
		assert.LessOrEqual(t, vm.CallCount(), (1+3)/2*3)
	})

	t.Run("empty block still computes system chunk", func(t *testing.T) {

		execCtx := fvm.NewContext()

		vm := new(fvmmock.VM)
		committer := new(computermock.ViewCommitter)

		bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
		trackerStorage := mocktracker.NewMockStorage()

		prov := provider.NewProvider(
			zerolog.Nop(),
			metrics.NewNoopCollector(),
			execution_data.DefaultSerializer,
			bservice,
			trackerStorage,
		)

		exe, err := computer.NewBlockComputer(
			vm,
			execCtx,
			metrics.NewNoopCollector(),
			trace.NewNoopTracer(),
			zerolog.Nop(),
			committer,
			me,
			prov,
			nil,
			testutil.ProtocolStateWithSourceFixture(nil),
			testMaxConcurrency)
		require.NoError(t, err)

		// create an empty block
		block := generateBlock(0, 0, rag)
		derivedBlockData := derived.NewEmptyDerivedBlockData(0)

		vm.On("NewExecutor", mock.Anything, mock.Anything, mock.Anything).
			Return(noOpExecutor{}).
			Once() // just system chunk

		snapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{},
			unittest.StateCommitmentFixture(),
		)

		committer.On("CommitView", mock.Anything, mock.Anything).
			Return(nil, nil, nil, snapshot, nil).
			Once() // just system chunk

		result, err := exe.ExecuteBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			block,
			nil,
			derivedBlockData)
		assert.NoError(t, err)
		assert.Len(t, result.AllExecutionSnapshots(), 1)
		assert.Len(t, result.AllTransactionResults(), 1)
		assert.Len(t, result.ChunkExecutionDatas, 1)

		assertEventHashesMatch(t, 1, result)

		vm.AssertExpectations(t)
	})

	t.Run("system chunk transaction should not fail", func(t *testing.T) {
		// include all fees. System chunk should ignore them
		contextOptions := []fvm.Option{
			fvm.WithEVMEnabled(true),
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
			fvm.WithBlocks(&environment.NoopBlockFinder{}),
		}
		// set 0 clusters to pass n_collectors >= n_clusters check
		epochConfig := epochs.DefaultEpochConfig()
		epochConfig.NumCollectorClusters = 0
		bootstrapOptions := []fvm.BootstrapProcedureOption{
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
			fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
			fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
			fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
			fvm.WithEpochConfig(epochConfig),
		}

		chain := flow.Localnet.Chain()
		vm := fvm.NewVirtualMachine()
		derivedBlockData := derived.NewEmptyDerivedBlockData(0)
		baseOpts := []fvm.Option{
			fvm.WithChain(chain),
			fvm.WithDerivedBlockData(derivedBlockData),
		}

		opts := append(baseOpts, contextOptions...)
		ctx := fvm.NewContext(opts...)
		snapshotTree := snapshot.NewSnapshotTree(nil)

		baseBootstrapOpts := []fvm.BootstrapProcedureOption{
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		}
		bootstrapOpts := append(baseBootstrapOpts, bootstrapOptions...)
		executionSnapshot, _, err := vm.Run(
			ctx,
			fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
			snapshotTree)
		require.NoError(t, err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		comm := new(computermock.ViewCommitter)

		bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
		trackerStorage := mocktracker.NewMockStorage()

		prov := provider.NewProvider(
			zerolog.Nop(),
			metrics.NewNoopCollector(),
			execution_data.DefaultSerializer,
			bservice,
			trackerStorage,
		)

		exe, err := computer.NewBlockComputer(
			vm,
			ctx,
			metrics.NewNoopCollector(),
			trace.NewNoopTracer(),
			zerolog.Nop(),
			comm,
			me,
			prov,
			nil,
			testutil.ProtocolStateWithSourceFixture(nil),
			testMaxConcurrency)
		require.NoError(t, err)

		// create an empty block
		block := generateBlock(0, 0, rag)

		snapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{},
			unittest.StateCommitmentFixture(),
		)

		comm.On("CommitView", mock.Anything, mock.Anything).
			Return(nil, nil, nil, snapshot, nil).
			Once() // just system chunk

		result, err := exe.ExecuteBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			block,
			snapshotTree,
			derivedBlockData.NewChildDerivedBlockData())
		assert.NoError(t, err)
		assert.Len(t, result.AllExecutionSnapshots(), 1)
		assert.Len(t, result.AllTransactionResults(), 1)
		assert.Len(t, result.ChunkExecutionDatas, 1)

		assert.Empty(t, result.AllTransactionResults()[0].ErrorMessage)
	})

	t.Run("multiple collections", func(t *testing.T) {
		execCtx := fvm.NewContext()

		committer := &fakeCommitter{
			callCount: 0,
		}

		bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
		trackerStorage := mocktracker.NewMockStorage()

		prov := provider.NewProvider(
			zerolog.Nop(),
			metrics.NewNoopCollector(),
			execution_data.DefaultSerializer,
			bservice,
			trackerStorage,
		)

		eventsPerTransaction := 2
		vm := &testVM{
			t:                    t,
			eventsPerTransaction: eventsPerTransaction,
			err: fvmErrors.NewInvalidAddressErrorf(
				flow.EmptyAddress,
				"no payer address provided"),
		}

		exe, err := computer.NewBlockComputer(
			vm,
			execCtx,
			metrics.NewNoopCollector(),
			trace.NewNoopTracer(),
			zerolog.Nop(),
			committer,
			me,
			prov,
			nil,
			testutil.ProtocolStateWithSourceFixture(nil),
			testMaxConcurrency)
		require.NoError(t, err)

		collectionCount := 2
		transactionsPerCollection := 2
		eventsPerCollection := eventsPerTransaction * transactionsPerCollection
		totalTransactionCount := (collectionCount * transactionsPerCollection) + 1 // +1 for system chunk
		// totalEventCount := eventsPerTransaction * totalTransactionCount

		// create a block with 2 collections with 2 transactions each
		block := generateBlock(collectionCount, transactionsPerCollection, rag)
		derivedBlockData := derived.NewEmptyDerivedBlockData(0)

		// No mock expectations needed - using real fakeCommitter with timing logs

		result, err := exe.ExecuteBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			block,
			nil,
			derivedBlockData)
		assert.NoError(t, err)

		// chunk count should match collection count
		assert.Equal(t, result.BlockExecutionResult.Size(), collectionCount+1) // system chunk

		// all events should have been collected
		for i := 0; i < collectionCount; i++ {
			events := result.CollectionExecutionResultAt(i).Events()
			assert.Len(t, events, eventsPerCollection)
		}

		// system chunk
		assert.Len(t, result.CollectionExecutionResultAt(collectionCount).Events(), eventsPerTransaction)

		events := result.AllEvents()

		// events should have been indexed by transaction and event
		k := 0
		for expectedTxIndex := 0; expectedTxIndex < totalTransactionCount; expectedTxIndex++ {
			for expectedEventIndex := 0; expectedEventIndex < eventsPerTransaction; expectedEventIndex++ {
				e := events[k]
				assert.EqualValues(t, expectedEventIndex, int(e.EventIndex))
				assert.EqualValues(t, expectedTxIndex, e.TransactionIndex)
				k++
			}
		}

		expectedResults := make([]flow.TransactionResult, 0)
		for _, c := range block.CompleteCollections {
			for _, t := range c.Collection.Transactions {
				txResult := flow.TransactionResult{
					TransactionID: t.ID(),
					ErrorMessage: fvmErrors.NewInvalidAddressErrorf(
						flow.EmptyAddress,
						"no payer address provided").Error(),
				}
				expectedResults = append(expectedResults, txResult)
			}
		}
		txResults := result.AllTransactionResults()
		assert.ElementsMatch(t, expectedResults, txResults[0:len(txResults)-1]) // strip system chunk

		assertEventHashesMatch(t, collectionCount+1, result)

		assert.GreaterOrEqual(t, vm.CallCount(), totalTransactionCount)
		// if every transaction is retried once, then the call count should be
		// (1+totalTransactionCount) /2 * totalTransactionCount
		assert.LessOrEqual(t, vm.CallCount(), (1+totalTransactionCount)/2*totalTransactionCount)
	})

	// TODO: this test is flaky with a low probability of failing
	t.Run(
		"service events are emitted", func(t *testing.T) {
			execCtx := fvm.NewContext(
				fvm.WithAuthorizationChecksEnabled(false),
				fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			)

			collectionCount := 2
			transactionsPerCollection := 2

			// create a block with 2 collections with 2 transactions each
			block := generateBlock(collectionCount, transactionsPerCollection, rag)

			chainID := execCtx.Chain.ChainID()
			serviceEvents := systemcontracts.ServiceEventsForChain(chainID)

			randomSource := unittest.EpochSetupRandomSourceFixture()
			payload, err := ccf.Decode(nil, unittest.EpochSetupFixtureCCF(randomSource))
			require.NoError(t, err)

			serviceEventA, ok := payload.(cadence.Event)
			require.True(t, ok)

			serviceEventA.EventType.Location = common.AddressLocation{
				Address: common.Address(serviceEvents.EpochSetup.Address),
			}
			serviceEventA.EventType.QualifiedIdentifier = serviceEvents.EpochSetup.QualifiedIdentifier()

			payload, err = ccf.Decode(nil, unittest.EpochCommitFixtureCCF)
			require.NoError(t, err)

			serviceEventB, ok := payload.(cadence.Event)
			require.True(t, ok)

			serviceEventB.EventType.Location = common.AddressLocation{
				Address: common.Address(serviceEvents.EpochCommit.Address),
			}
			serviceEventB.EventType.QualifiedIdentifier = serviceEvents.EpochCommit.QualifiedIdentifier()

			payload, err = ccf.Decode(nil, unittest.VersionBeaconFixtureCCF)
			require.NoError(t, err)

			serviceEventC, ok := payload.(cadence.Event)
			require.True(t, ok)

			serviceEventC.EventType.Location = common.AddressLocation{
				Address: common.Address(serviceEvents.VersionBeacon.Address),
			}
			serviceEventC.EventType.QualifiedIdentifier = serviceEvents.VersionBeacon.QualifiedIdentifier()

			transactions := []*flow.TransactionBody{}
			for _, col := range block.Collections() {
				transactions = append(transactions, col.Collection.Transactions...)
			}

			// events to emit for each iteration/transaction
			events := map[common.Location][]cadence.Event{
				common.TransactionLocation(transactions[0].ID()): nil,
				common.TransactionLocation(transactions[1].ID()): {
					serviceEventA,
					{
						EventType: &cadence.EventType{
							Location:            stdlib.FlowLocation{},
							QualifiedIdentifier: "what.ever",
						},
					},
				},
				common.TransactionLocation(transactions[2].ID()): {
					{
						EventType: &cadence.EventType{
							Location:            stdlib.FlowLocation{},
							QualifiedIdentifier: "what.ever",
						},
					},
				},
				common.TransactionLocation(transactions[3].ID()): nil,
			}

			systemTransactionEvents := []cadence.Event{
				serviceEventB,
				serviceEventC,
			}

			emittingRuntime := &testRuntime{
				executeTransaction: func(
					script runtime.Script,
					context runtime.Context,
				) error {
					scriptEvents, ok := events[context.Location]
					if !ok {
						scriptEvents = systemTransactionEvents
					}

					for _, e := range scriptEvents {
						err := context.Interface.EmitEvent(e)
						if err != nil {
							return err
						}
					}
					return nil
				},
				readStored: func(
					address common.Address,
					path cadence.Path,
					r runtime.Context,
				) (cadence.Value, error) {
					return nil, nil
				},
			}

			execCtx = fvm.NewContextFromParent(
				execCtx,
				fvm.WithReusableCadenceRuntimePool(
					reusableRuntime.NewCustomReusableCadenceRuntimePool(
						0,
						runtime.Config{},
						func(_ runtime.Config) runtime.Runtime {
							return emittingRuntime
						},
					),
				),
			)

			vm := fvm.NewVirtualMachine()

			bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
			trackerStorage := mocktracker.NewMockStorage()

			prov := provider.NewProvider(
				zerolog.Nop(),
				metrics.NewNoopCollector(),
				execution_data.DefaultSerializer,
				bservice,
				trackerStorage,
			)

			exe, err := computer.NewBlockComputer(
				vm,
				execCtx,
				metrics.NewNoopCollector(),
				trace.NewNoopTracer(),
				zerolog.Nop(),
				committer.NewNoopViewCommitter(),
				me,
				prov,
				nil,
				testutil.ProtocolStateWithSourceFixture(nil),
				testMaxConcurrency)
			require.NoError(t, err)

			result, err := exe.ExecuteBlock(
				context.Background(),
				unittest.IdentifierFixture(),
				block,
				nil,
				derived.NewEmptyDerivedBlockData(0),
			)
			require.NoError(t, err)

			// make sure event index sequence are valid
			for i := 0; i < result.BlockExecutionResult.Size(); i++ {
				collectionResult := result.CollectionExecutionResultAt(i)
				unittest.EnsureEventsIndexSeq(t, collectionResult.Events(), chainID)
			}

			sEvents := result.AllServiceEvents() // all events should have been collected
			require.Len(t, sEvents, 3)

			// events are ordered
			require.Equal(
				t,
				serviceEventA.EventType.ID(),
				string(sEvents[0].Type),
			)
			require.Equal(
				t,
				serviceEventB.EventType.ID(),
				string(sEvents[1].Type),
			)

			require.Equal(
				t,
				serviceEventC.EventType.ID(),
				string(sEvents[2].Type),
			)

			assertEventHashesMatch(t, collectionCount+1, result)
		},
	)

	t.Run("succeeding transactions store programs", func(t *testing.T) {

		execCtx := fvm.NewContext()

		address := common.Address{0x1}
		contractLocation := common.AddressLocation{
			Address: address,
			Name:    "Test",
		}

		contractProgram := &runtime.Program{}

		rt := &testRuntime{
			executeTransaction: func(script runtime.Script, r runtime.Context) error {

				_, err := r.Interface.GetOrLoadProgram(
					contractLocation,
					func() (*runtime.Program, error) {
						return contractProgram, nil
					},
				)
				require.NoError(t, err)

				return nil
			},
			readStored: func(
				address common.Address,
				path cadence.Path,
				r runtime.Context,
			) (cadence.Value, error) {
				return nil, nil
			},
		}

		execCtx = fvm.NewContextFromParent(
			execCtx,
			fvm.WithReusableCadenceRuntimePool(
				reusableRuntime.NewCustomReusableCadenceRuntimePool(
					0,
					runtime.Config{},
					func(_ runtime.Config) runtime.Runtime {
						return rt
					})),
		)

		vm := fvm.NewVirtualMachine()

		bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
		trackerStorage := mocktracker.NewMockStorage()

		prov := provider.NewProvider(
			zerolog.Nop(),
			metrics.NewNoopCollector(),
			execution_data.DefaultSerializer,
			bservice,
			trackerStorage,
		)

		exe, err := computer.NewBlockComputer(
			vm,
			execCtx,
			metrics.NewNoopCollector(),
			trace.NewNoopTracer(),
			zerolog.Nop(),
			committer.NewNoopViewCommitter(),
			me,
			prov,
			nil,
			testutil.ProtocolStateWithSourceFixture(nil),
			testMaxConcurrency)
		require.NoError(t, err)

		const collectionCount = 2
		const transactionCount = 2
		block := generateBlock(collectionCount, transactionCount, rag)

		key := flow.AccountStatusRegisterID(
			flow.BytesToAddress(address.Bytes()))
		value := environment.NewAccountStatus().ToBytes()

		result, err := exe.ExecuteBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			block,
			snapshot.MapStorageSnapshot{key: value},
			derived.NewEmptyDerivedBlockData(0))
		assert.NoError(t, err)
		assert.Len(t, result.AllExecutionSnapshots(), collectionCount+1) // +1 system chunk
	})

	t.Run("failing transactions do not store programs", func(t *testing.T) {
		execCtx := fvm.NewContext(
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		)

		address := common.Address{0x1}

		contractLocation := common.AddressLocation{
			Address: address,
			Name:    "Test",
		}

		contractProgram := &runtime.Program{}

		const collectionCount = 2
		const transactionCount = 2
		block := generateBlock(collectionCount, transactionCount, rag)

		normalTransactions := map[common.Location]struct{}{}
		for _, col := range block.Collections() {
			for _, txn := range col.Collection.Transactions {
				loc := common.TransactionLocation(txn.ID())
				normalTransactions[loc] = struct{}{}
			}
		}

		rt := &testRuntime{
			executeTransaction: func(script runtime.Script, r runtime.Context) error {

				// NOTE: set a program and revert all transactions but the
				// system chunk transaction
				_, err := r.Interface.GetOrLoadProgram(
					contractLocation,
					func() (*runtime.Program, error) {
						return contractProgram, nil
					},
				)
				require.NoError(t, err)

				_, ok := normalTransactions[r.Location]
				if ok {
					return runtime.Error{
						Err: fmt.Errorf("TX reverted"),
					}
				}

				return nil
			},
			readStored: func(
				address common.Address,
				path cadence.Path,
				r runtime.Context,
			) (cadence.Value, error) {
				return nil, nil
			},
		}

		execCtx = fvm.NewContextFromParent(
			execCtx,
			fvm.WithReusableCadenceRuntimePool(
				reusableRuntime.NewCustomReusableCadenceRuntimePool(
					0,
					runtime.Config{},
					func(_ runtime.Config) runtime.Runtime {
						return rt
					})),
		)

		vm := fvm.NewVirtualMachine()

		bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
		trackerStorage := mocktracker.NewMockStorage()

		prov := provider.NewProvider(
			zerolog.Nop(),
			metrics.NewNoopCollector(),
			execution_data.DefaultSerializer,
			bservice,
			trackerStorage,
		)

		exe, err := computer.NewBlockComputer(
			vm,
			execCtx,
			metrics.NewNoopCollector(),
			trace.NewNoopTracer(),
			zerolog.Nop(),
			committer.NewNoopViewCommitter(),
			me,
			prov,
			nil,
			testutil.ProtocolStateWithSourceFixture(nil),
			testMaxConcurrency)
		require.NoError(t, err)

		key := flow.AccountStatusRegisterID(
			flow.BytesToAddress(address.Bytes()))
		value := environment.NewAccountStatus().ToBytes()

		result, err := exe.ExecuteBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			block,
			snapshot.MapStorageSnapshot{key: value},
			derived.NewEmptyDerivedBlockData(0))
		require.NoError(t, err)
		assert.Len(t, result.AllExecutionSnapshots(), collectionCount+1) // +1 system chunk
	})

	t.Run("internal error", func(t *testing.T) {
		execCtx := fvm.NewContext()

		committer := new(computermock.ViewCommitter)

		bservice := requesterunit.MockBlobService(
			blockstore.NewBlockstore(
				dssync.MutexWrap(datastore.NewMapDatastore())))
		trackerStorage := mocktracker.NewMockStorage()

		prov := provider.NewProvider(
			zerolog.Nop(),
			metrics.NewNoopCollector(),
			execution_data.DefaultSerializer,
			bservice,
			trackerStorage)

		exe, err := computer.NewBlockComputer(
			errorVM{errorAt: 5},
			execCtx,
			metrics.NewNoopCollector(),
			trace.NewNoopTracer(),
			zerolog.Nop(),
			committer,
			me,
			prov,
			nil,
			testutil.ProtocolStateWithSourceFixture(nil),
			testMaxConcurrency)
		require.NoError(t, err)

		collectionCount := 5
		transactionsPerCollection := 3
		block := generateBlock(collectionCount, transactionsPerCollection, rag)

		snapshot := storehouse.NewExecutingBlockSnapshot(
			snapshot.MapStorageSnapshot{},
			unittest.StateCommitmentFixture(),
		)

		committer.On("CommitView", mock.Anything, mock.Anything).
			Return(nil, nil, nil, snapshot, nil).
			Times(collectionCount + 1)

		_, err = exe.ExecuteBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			block,
			nil,
			derived.NewEmptyDerivedBlockData(0))
		assert.ErrorContains(t, err, "boom - internal error")
	})

}

func assertEventHashesMatch(
	t *testing.T,
	expectedNoOfChunks int,
	result *execution.ComputationResult,
) {
	execResSize := result.BlockExecutionResult.Size()
	attestResSize := result.BlockAttestationResult.Size()
	require.Equal(t, execResSize, expectedNoOfChunks)
	require.Equal(t, execResSize, attestResSize)

	for i := 0; i < expectedNoOfChunks; i++ {
		events := result.CollectionExecutionResultAt(i).Events()
		calculatedHash, err := flow.EventsMerkleRootHash(events)
		require.NoError(t, err)
		require.Equal(t, calculatedHash, result.CollectionAttestationResultAt(i).EventCommitment())
	}
}

type testTransactionExecutor struct {
	executeTransaction func(runtime.Script, runtime.Context) error

	script  runtime.Script
	context runtime.Context
}

func (executor *testTransactionExecutor) Preprocess() error {
	// Do nothing.
	return nil
}

func (executor *testTransactionExecutor) Execute() error {
	return executor.executeTransaction(executor.script, executor.context)
}

func (executor *testTransactionExecutor) Result() (cadence.Value, error) {
	panic("Result not expected")
}

type testRuntime struct {
	executeScript      func(runtime.Script, runtime.Context) (cadence.Value, error)
	executeTransaction func(runtime.Script, runtime.Context) error
	readStored         func(common.Address, cadence.Path, runtime.Context) (
		cadence.Value,
		error,
	)
}

var _ runtime.Runtime = &testRuntime{}

func (e *testRuntime) Config() runtime.Config {
	panic("Config not expected")
}

func (e *testRuntime) NewScriptExecutor(
	script runtime.Script,
	c runtime.Context,
) runtime.Executor {
	panic("NewScriptExecutor not expected")
}

func (e *testRuntime) NewTransactionExecutor(
	script runtime.Script,
	c runtime.Context,
) runtime.Executor {
	return &testTransactionExecutor{
		executeTransaction: e.executeTransaction,
		script:             script,
		context:            c,
	}
}

func (e *testRuntime) NewContractFunctionExecutor(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	context runtime.Context,
) runtime.Executor {
	panic("NewContractFunctionExecutor not expected")
}

func (e *testRuntime) SetInvalidatedResourceValidationEnabled(_ bool) {
	panic("SetInvalidatedResourceValidationEnabled not expected")
}

func (e *testRuntime) SetTracingEnabled(_ bool) {
	panic("SetTracingEnabled not expected")
}

func (e *testRuntime) SetResourceOwnerChangeHandlerEnabled(_ bool) {
	panic("SetResourceOwnerChangeHandlerEnabled not expected")
}

func (e *testRuntime) InvokeContractFunction(
	_ common.AddressLocation,
	_ string,
	_ []cadence.Value,
	_ []sema.Type,
	_ runtime.Context,
) (cadence.Value, error) {
	panic("InvokeContractFunction not expected")
}

func (e *testRuntime) ExecuteScript(
	script runtime.Script,
	context runtime.Context,
) (cadence.Value, error) {
	return e.executeScript(script, context)
}

func (e *testRuntime) ExecuteTransaction(
	script runtime.Script,
	context runtime.Context,
) error {
	return e.executeTransaction(script, context)
}

func (*testRuntime) ParseAndCheckProgram(
	_ []byte,
	_ runtime.Context,
) (*interpreter.Program, error) {
	panic("ParseAndCheckProgram not expected")
}

func (*testRuntime) SetCoverageReport(_ *runtime.CoverageReport) {
	panic("SetCoverageReport not expected")
}

func (*testRuntime) SetContractUpdateValidationEnabled(_ bool) {
	panic("SetContractUpdateValidationEnabled not expected")
}

func (*testRuntime) SetAtreeValidationEnabled(_ bool) {
	panic("SetAtreeValidationEnabled not expected")
}

func (e *testRuntime) ReadStored(
	a common.Address,
	p cadence.Path,
	c runtime.Context,
) (cadence.Value, error) {
	return e.readStored(a, p, c)
}

func (*testRuntime) SetDebugger(_ *interpreter.Debugger) {
	panic("SetDebugger not expected")
}

type RandomAddressGenerator struct{}

func (r *RandomAddressGenerator) NextAddress() (flow.Address, error) {
	return flow.HexToAddress(fmt.Sprintf("0%d", rand.Intn(1000))), nil
}

func (r *RandomAddressGenerator) CurrentAddress() flow.Address {
	return flow.HexToAddress(fmt.Sprintf("0%d", rand.Intn(1000)))
}

func (r *RandomAddressGenerator) Bytes() []byte {
	panic("not implemented")
}

func (r *RandomAddressGenerator) AddressCount() uint64 {
	panic("not implemented")
}

func (testRuntime) Storage(runtime.Context) (
	*runtime.Storage,
	*interpreter.Interpreter,
	error,
) {
	panic("Storage not expected")
}

type FixedAddressGenerator struct {
	Address flow.Address
}

func (f *FixedAddressGenerator) NextAddress() (flow.Address, error) {
	return f.Address, nil
}

func (f *FixedAddressGenerator) CurrentAddress() flow.Address {
	return f.Address
}

func (f *FixedAddressGenerator) Bytes() []byte {
	panic("not implemented")
}

func (f *FixedAddressGenerator) AddressCount() uint64 {
	panic("not implemented")
}

func Test_ExecutingSystemCollection(t *testing.T) {

	execCtx := fvm.NewContext(
		fvm.WithEVMEnabled(true),
		fvm.WithChain(flow.Localnet.Chain()),
		fvm.WithBlocks(&environment.NoopBlockFinder{}),
	)

	vm := fvm.NewVirtualMachine()

	rag := &RandomAddressGenerator{}

	ledger := testutil.RootBootstrappedLedger(vm, execCtx)

	committer := new(computermock.ViewCommitter)
	snapshot := storehouse.NewExecutingBlockSnapshot(
		snapshot.MapStorageSnapshot{},
		unittest.StateCommitmentFixture(),
	)

	committer.On("CommitView", mock.Anything, mock.Anything).
		Return(nil, nil, nil, snapshot, nil).
		Times(1) // only system chunk

	noopCollector := metrics.NewNoopCollector()

	expectedNumberOfEvents := 4
	expectedMinEventSize := 1000

	metrics := new(modulemock.ExecutionMetrics)
	metrics.On("ExecutionBlockExecuted",
		mock.Anything,
		mock.Anything).
		Return(nil).
		Times(1)

	metrics.On("ExecutionCollectionExecuted",
		mock.Anything,
		mock.Anything).
		Return(nil).
		Times(1) // system collection

	metrics.On("ExecutionTransactionExecuted",
		mock.Anything, // duration
		mock.MatchedBy(func(arg module.TransactionExecutionResultStats) bool {
			return arg.EventCounts == expectedNumberOfEvents &&
				arg.EventSize >= expectedMinEventSize &&
				!arg.Failed
		}),
		mock.Anything).
		Return(nil).
		Times(1) // system chunk tx

	metrics.On(
		"ExecutionChunkDataPackGenerated",
		mock.Anything,
		mock.Anything).
		Return(nil).
		Times(1) // system collection

	metrics.On(
		"ExecutionBlockCachedPrograms",
		mock.Anything).
		Run(func(args mock.Arguments) {
			actual := args[0].(int)
			// bootstrapping already caches some programs
			require.Greater(t, actual, 0)
		}).
		Return(nil).
		Times(1) // block

	metrics.On(
		"ExecutionBlockExecutionEffortVectorComponent",
		mock.Anything,
		mock.Anything).
		Return(nil)

	metrics.On("RuntimeTransactionParsed", mock.Anything)
	metrics.On("RuntimeTransactionProgramsCacheMiss")
	metrics.On("RuntimeTransactionProgramsCacheHit")
	metrics.On("RuntimeTransactionChecked", mock.Anything)
	metrics.On("RuntimeTransactionInterpreted", mock.Anything)

	metrics.On("EVMBlockExecuted",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		noopCollector,
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	me := new(modulemock.Local)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(unittest.SignatureFixture(), nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	constRandomSource := make([]byte, 32)

	exe, err := computer.NewBlockComputer(
		vm,
		execCtx,
		metrics,
		trace.NewNoopTracer(),
		zerolog.Nop(),
		committer,
		me,
		prov,
		nil,
		testutil.ProtocolStateWithSourceFixture(constRandomSource),
		testMaxConcurrency)
	require.NoError(t, err)

	// create empty block, it will have system collection attached while executing
	block := generateBlock(0, 0, rag)

	result, err := exe.ExecuteBlock(
		context.Background(),
		unittest.IdentifierFixture(),
		block,
		ledger,
		derived.NewEmptyDerivedBlockData(0))
	assert.NoError(t, err)
	assert.Len(t, result.AllExecutionSnapshots(), 1) // +1 system chunk
	assert.Len(t, result.AllTransactionResults(), 1)

	assert.Empty(t, result.AllTransactionResults()[0].ErrorMessage)

	committer.AssertExpectations(t)
}

func Test_ScheduledCallback(t *testing.T) {
	chain := flow.Testnet.Chain()

	t.Run("process with no scheduled callback", func(t *testing.T) {
		testScheduledCallback(t, chain, []cadence.Event{}, 2) // process callback + system chunk
	})

	t.Run("process with 2 scheduled callbacks", func(t *testing.T) {
		// create callback events that process callback will return
		env := systemcontracts.SystemContractsForChain(chain.ChainID())
		location := common.NewAddressLocation(nil, common.Address(env.FlowCallbackScheduler.Address), "FlowTransactionScheduler")

		eventType := cadence.NewEventType(
			location,
			"PendingExecution",
			[]cadence.Field{
				{Identifier: "id", Type: cadence.UInt64Type},
				{Identifier: "priority", Type: cadence.UInt8Type},
				{Identifier: "executionEffort", Type: cadence.UInt64Type},
				{Identifier: "fees", Type: cadence.UFix64Type},
				{Identifier: "callbackOwner", Type: cadence.AddressType},
			},
			nil,
		)

		callbackID1 := uint64(1)
		callbackID2 := uint64(2)

		fees, err := cadence.NewUFix64("0.0")
		require.NoError(t, err)

		callbackEvent1 := cadence.NewEvent(
			[]cadence.Value{
				cadence.NewUInt64(callbackID1),
				cadence.NewUInt8(1),
				cadence.NewUInt64(1000), // execution effort
				fees,
				cadence.NewAddress(env.FlowServiceAccount.Address),
			},
		).WithType(eventType)

		callbackEvent2 := cadence.NewEvent(
			[]cadence.Value{
				cadence.NewUInt64(callbackID2),
				cadence.NewUInt8(1),
				cadence.NewUInt64(2000), // execution effort
				fees,
				cadence.NewAddress(env.FlowServiceAccount.Address),
			},
		).WithType(eventType)

		testScheduledCallback(t, chain, []cadence.Event{callbackEvent1, callbackEvent2}, 4) // process callback + 2 callbacks + system chunk
	})

	t.Run("process callback transaction execution error", func(t *testing.T) {
		processCallbackError := fvmErrors.NewInvalidAddressErrorf(flow.EmptyAddress, "process callback execution failed")
		testScheduledCallbackWithError(t, chain, []cadence.Event{}, 0, processCallbackError, nil)
	})

	t.Run("process callback transaction output error", func(t *testing.T) {
		processCallbackError := fvmErrors.NewInvalidAddressErrorf(flow.EmptyAddress, "process callback output error")
		testScheduledCallbackWithError(t, chain, []cadence.Event{}, 2, nil, processCallbackError)
	})
}

func testScheduledCallback(t *testing.T, chain flow.Chain, callbackEvents []cadence.Event, expectedTransactionCount int) {
	testScheduledCallbackWithError(t, chain, callbackEvents, expectedTransactionCount, nil, nil)
}

func testScheduledCallbackWithError(
	t *testing.T,
	chain flow.Chain,
	callbackEvents []cadence.Event,
	expectedTransactionCount int,
	processExecuteError fvmErrors.CodedError,
	processOutputError fvmErrors.CodedError,
) {
	rag := &RandomAddressGenerator{}
	executorID := unittest.IdentifierFixture()

	testLogger := NewTestLogger()

	execCtx := fvm.NewContext(
		fvm.WithScheduleCallbacksEnabled(true), // Enable callbacks
		fvm.WithChain(chain),
		fvm.WithLogger(testLogger.Logger),
	)

	// track which transactions were executed and their details
	executedTransactions := make(map[string]string)
	var executedTransactionsMutex sync.Mutex

	// encode events to create flow event payloads
	eventPayloads := make([][]byte, len(callbackEvents))
	callbackIDs := make([]uint64, len(callbackEvents))
	for i, event := range callbackEvents {
		payload, err := ccf.Encode(event)
		require.NoError(t, err)
		eventPayloads[i] = payload

		// extract callback ID from event for later comparison
		if len(callbackEvents) > 0 {
			decodedEvent, err := ccf.Decode(nil, payload)
			require.NoError(t, err)
			if cadenceEvent, ok := decodedEvent.(cadence.Event); ok {
				// search for the ID field in the event
				idField := cadence.SearchFieldByName(cadenceEvent, "id")
				if idValue, ok := idField.(cadence.UInt64); ok {
					callbackIDs[i] = uint64(idValue)
				}
			}
		}
	}

	// create a VM that will track execution and return appropriate events
	vm := &callbackTestVM{
		testVM: testVM{
			t:                    t,
			eventsPerTransaction: 0,                   // we'll handle events manually
			err:                  processExecuteError, // inject error if provided
		},
		processOutputErr:     processOutputError,
		executedTransactions: executedTransactions,
		executedMutex:        &executedTransactionsMutex,
		eventPayloads:        eventPayloads,
		callbackIDs:          callbackIDs,
	}

	committer := &fakeCommitter{
		callCount: 0,
	}

	me := new(modulemock.Local)
	me.On("NodeID").Return(executorID)
	me.On("Sign", mock.Anything, mock.Anything).Return(unittest.SignatureFixture(), nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(unittest.SignatureFixture(), nil)

	exemetrics := new(modulemock.ExecutionMetrics)
	exemetrics.On("ExecutionBlockExecuted",
		mock.Anything,
		mock.Anything).
		Return(nil).
		Times(1)

	// expect 1 system collection execution
	exemetrics.On("ExecutionCollectionExecuted",
		mock.Anything,
		mock.Anything).
		Return(nil).
		Times(1)

	if processOutputError != nil {
		// expect 1 failed transaction (process callback) + 1 successful transaction (system chunk)
		exemetrics.On("ExecutionTransactionExecuted",
			mock.Anything,
			mock.MatchedBy(func(arg module.TransactionExecutionResultStats) bool {
				return arg.Failed && (arg.SystemTransaction || arg.ScheduledTransaction)
			}),
			mock.Anything).
			Return(nil).
			Times(1)
		exemetrics.On("ExecutionTransactionExecuted",
			mock.Anything,
			mock.MatchedBy(func(arg module.TransactionExecutionResultStats) bool {
				return !arg.Failed && (arg.SystemTransaction || arg.ScheduledTransaction)
			}),
			mock.Anything).
			Return(nil).
			Times(expectedTransactionCount - 1)
	} else {
		exemetrics.On("ExecutionTransactionExecuted",
			mock.Anything,
			mock.MatchedBy(func(arg module.TransactionExecutionResultStats) bool {
				return !arg.Failed && (arg.SystemTransaction || arg.ScheduledTransaction)
			}),
			mock.Anything).
			Return(nil).
			Times(expectedTransactionCount)
	}

	exemetrics.On(
		"ExecutionChunkDataPackGenerated",
		mock.Anything,
		mock.Anything).
		Return(nil).
		Times(1) // system collection

	exemetrics.On(
		"ExecutionBlockCachedPrograms",
		mock.Anything).
		Return(nil).
		Times(1)

	// expect callback execution metrics if there are callbacks
	if len(callbackEvents) > 0 {
		exemetrics.On("ExecutionCallbacksExecuted",
			mock.Anything,
			mock.Anything,
			mock.Anything).
			Return(nil).
			Times(1)
	}

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	exe, err := computer.NewBlockComputer(
		vm,
		execCtx,
		exemetrics,
		trace.NewNoopTracer(),
		testLogger.Logger,
		committer,
		me,
		prov,
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		testMaxConcurrency)
	require.NoError(t, err)

	// create empty block (no user collections)
	block := generateBlock(0, 0, rag)

	parentBlockExecutionResultID := unittest.IdentifierFixture()
	result, err := exe.ExecuteBlock(
		context.Background(),
		parentBlockExecutionResultID,
		block,
		nil,
		derived.NewEmptyDerivedBlockData(0))

	// If we expect an error, verify it and return early
	if processExecuteError != nil {
		require.Error(t, err)
		require.Contains(t, err.Error(), "system process transaction")
		return
	}

	if processOutputError != nil {
		require.NoError(t, err)
		require.Truef(
			t,
			testLogger.HasLogWithField("system process transaction output error", "critical_error", true),
			"expected critical error log not found",
		)

		// verify the process callback transaction failed as expected
		require.Len(t, result.AllTransactionResults(), expectedTransactionCount)
		processCallbackResult := result.AllTransactionResults()[0]
		require.NotEmpty(t, processCallbackResult.ErrorMessage, "process callback transaction should have failed")
		require.Contains(t, processCallbackResult.ErrorMessage, "process callback output error")

		// verify system chunk transaction succeeded
		systemChunkResult := result.AllTransactionResults()[1]
		require.Empty(t, systemChunkResult.ErrorMessage, "system chunk transaction should not have failed")
		return
	}

	require.NoError(t, err)

	// verify execution results
	assert.Len(t, result.AllExecutionSnapshots(), 1) // Only system chunk
	assert.Len(t, result.AllTransactionResults(), expectedTransactionCount)

	// verify correct number of commits (1 for system collection)
	assert.Equal(t, 1, committer.callCount)
	assert.Equal(t, expectedTransactionCount, len(executedTransactions))

	// verify we executed each type of transaction
	hasProcessCallback := false
	hasSystemChunk := false
	callbackNames := make(map[string]bool)

	for _, txType := range executedTransactions {
		switch txType {
		case "process_callback":
			hasProcessCallback = true
		case "system_chunk":
			hasSystemChunk = true
		default:
			if strings.HasPrefix(txType, "callback") {
				// add unique callbacks to the map
				callbackNames[txType] = true
			}
		}
	}

	assert.True(t, hasProcessCallback, "process callback transaction should have been executed")
	assert.True(t, hasSystemChunk, "system chunk transaction should have been executed")
	assert.Equal(t, len(callbackEvents), len(callbackNames), "should have executed the expected number of callback transactions")

	// verify no transaction errors
	for _, txResult := range result.AllTransactionResults() {
		assert.Empty(t, txResult.ErrorMessage, "transaction should not have failed")
	}

	// verify receipt structure
	receipt := result.ExecutionReceipt
	assert.Equal(t, executorID, receipt.ExecutorID)
	assert.Equal(t, parentBlockExecutionResultID, receipt.PreviousResultID)
	assert.Equal(t, block.BlockID(), receipt.BlockID)
	assert.Len(t, receipt.Chunks, 1) // Only system chunk

	// verify system chunk details
	systemChunk := receipt.Chunks[0]
	assert.Equal(t, block.BlockID(), systemChunk.BlockID)
	assert.Equal(t, uint(0), systemChunk.CollectionIndex) // System collection is at index 0 for empty block
	assert.Equal(t, uint64(expectedTransactionCount), systemChunk.NumberOfTransactions)

	// verify all mocks were called as expected
	exemetrics.AssertExpectations(t)
}

// callbackTestVM is a custom VM for testing callback execution
type callbackTestVM struct {
	testVM
	processOutputErr     fvmErrors.CodedError
	executedTransactions map[string]string
	executedMutex        *sync.Mutex
	eventPayloads        [][]byte
	callbackIDs          []uint64
}

func (vm *callbackTestVM) NewExecutor(
	ctx fvm.Context,
	proc fvm.Procedure,
	txnState storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	// Create a custom executor that tracks execution and returns proper events
	return &callbackTestExecutor{
		testExecutor: testExecutor{
			testVM:   &vm.testVM,
			ctx:      ctx,
			proc:     proc,
			txnState: txnState,
		},
		vm: vm,
	}
}

// callbackTestExecutor is a custom executor for testing callback execution
type callbackTestExecutor struct {
	testExecutor
	vm *callbackTestVM
}

func (c *callbackTestExecutor) Execute() error {
	// Return error if one was injected for process callback transaction
	if c.vm.err != nil {
		txProc, ok := c.proc.(*fvm.TransactionProcedure)
		if ok {
			script := string(txProc.Transaction.Script)
			if strings.Contains(script, "scheduler.process") {
				return c.vm.err
			}
		}
	}

	return c.testExecutor.Execute()
}

// we need to reimplement this Output since the events are consumed in the block computer
// from the output of the procedure executor
func (c *callbackTestExecutor) Output() fvm.ProcedureOutput {
	// Return error if one was injected for process callback transaction
	if c.vm.processOutputErr != nil {
		txProc, ok := c.proc.(*fvm.TransactionProcedure)
		if ok {
			script := string(txProc.Transaction.Script)
			if strings.Contains(script, "scheduler.process") {
				return fvm.ProcedureOutput{
					Err: c.vm.processOutputErr,
				}
			}
		}
	}

	c.vm.executedMutex.Lock()
	defer c.vm.executedMutex.Unlock()

	txProc, ok := c.proc.(*fvm.TransactionProcedure)
	if !ok {
		return fvm.ProcedureOutput{}
	}

	const callbackSchedulerImport = `import "FlowTransactionScheduler"`
	txBody := txProc.Transaction
	txID := fmt.Sprintf("tx_%d", txProc.TxIndex)

	switch {
	// scheduled callbacks process transaction
	case strings.Contains(string(txBody.Script), "scheduler.process"):
		c.vm.executedTransactions[txID] = "process_callback"
		env := systemcontracts.SystemContractsForChain(c.ctx.Chain.ChainID()).AsTemplateEnv()
		eventTypeString := fmt.Sprintf("A.%v.FlowTransactionScheduler.PendingExecution", env.FlowTransactionSchedulerAddress)

		// return events for each scheduled callback
		events := make([]flow.Event, len(c.vm.eventPayloads))
		for i, payload := range c.vm.eventPayloads {
			events[i] = flow.Event{
				Type:             flow.EventType(eventTypeString),
				TransactionID:    txProc.ID,
				TransactionIndex: txProc.TxIndex,
				EventIndex:       uint32(i),
				Payload:          payload,
			}
		}

		return fvm.ProcedureOutput{
			Events: events,
		}
	// scheduled callbacks execute transaction
	case strings.Contains(string(txBody.Script), "scheduler.executeTransaction"):
		// extract the callback ID from the arguments
		if len(txBody.Arguments) == 0 {
			return fvm.ProcedureOutput{}
		}

		// decode the argument to check which callback it is
		argValue, err := jsoncdc.Decode(nil, txBody.Arguments[0])
		if err == nil {
			if idValue, ok := argValue.(cadence.UInt64); ok {
				// find which callback this is
				callbackIndex := -1
				for i, callbackID := range c.vm.callbackIDs {
					if uint64(idValue) == callbackID {
						callbackIndex = i
						break
					}
				}

				if callbackIndex >= 0 {
					c.vm.executedTransactions[txID] = fmt.Sprintf("callback%d", callbackIndex+1)
				} else {
					c.vm.executedTransactions[txID] = "unknown_callback"
				}
			}
		}

		return fvm.ProcedureOutput{}
	// system chunk transaction
	default:
		c.vm.executedTransactions[txID] = "system_chunk"
		return fvm.ProcedureOutput{}
	}
}

func generateBlock(
	collectionCount, transactionCount int,
	addressGenerator flow.AddressGenerator,
) *entity.ExecutableBlock {
	return generateBlockWithVisitor(collectionCount, transactionCount, addressGenerator, nil)
}

func generateBlockWithVisitor(
	collectionCount, transactionCount int,
	addressGenerator flow.AddressGenerator,
	visitor func(body *flow.TransactionBody),
) *entity.ExecutableBlock {
	collections := make([]*entity.CompleteCollection, collectionCount)
	guarantees := make([]*flow.CollectionGuarantee, collectionCount)
	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection)

	for i := 0; i < collectionCount; i++ {
		collection := generateCollection(transactionCount, addressGenerator, visitor)
		collections[i] = collection
		guarantees[i] = collection.Guarantee
		completeCollections[collection.Guarantee.CollectionID] = collection
	}

	block := unittest.BlockFixture(
		unittest.Block.WithHeight(42),
		unittest.Block.WithView(42),
		unittest.Block.WithParentView(41),
		unittest.Block.WithPayload(
			unittest.PayloadFixture(unittest.WithGuarantees(guarantees...)),
		),
	)

	return &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: completeCollections,
		StartState:          unittest.StateCommitmentPointerFixture(),
	}
}

func generateCollection(
	transactionCount int,
	addressGenerator flow.AddressGenerator,
	visitor func(body *flow.TransactionBody),
) *entity.CompleteCollection {
	transactions := make([]*flow.TransactionBody, transactionCount)

	for i := 0; i < transactionCount; i++ {
		nextAddress, err := addressGenerator.NextAddress()
		if err != nil {
			panic(fmt.Errorf("cannot generate next address in test: %w", err))
		}
		txBody := &flow.TransactionBody{
			Payer:  nextAddress, // a unique payer for each tx to generate a unique id
			Script: []byte("transaction { execute {} }"),
		}
		if visitor != nil {
			visitor(txBody)
		}
		transactions[i] = txBody
	}

	collection := flow.Collection{Transactions: transactions}

	guarantee := &flow.CollectionGuarantee{CollectionID: collection.ID()}

	return &entity.CompleteCollection{
		Guarantee:  guarantee,
		Collection: &flow.Collection{Transactions: transactions},
	}
}

type noOpExecutor struct{}

func (noOpExecutor) Cleanup() {}

func (noOpExecutor) Preprocess() error {
	return nil
}

func (noOpExecutor) Execute() error {
	return nil
}

func (noOpExecutor) Output() fvm.ProcedureOutput {
	return fvm.ProcedureOutput{}
}

type testVM struct {
	t                    *testing.T
	eventsPerTransaction int

	callCount int32 // atomic variable
	err       fvmErrors.CodedError
}

type testExecutor struct {
	*testVM

	ctx      fvm.Context
	proc     fvm.Procedure
	txnState storage.TransactionPreparer
}

func (testExecutor) Cleanup() {
}

func (testExecutor) Preprocess() error {
	return nil
}

func (executor *testExecutor) Execute() error {
	atomic.AddInt32(&executor.callCount, 1)

	// Add timing logs for parallel execution verification
	txn := executor.proc.(*fvm.TransactionProcedure)
	// Get collection index from the transaction request context
	collectionIndex := int(txn.TxIndex) / 2 // This is approximate - we need the real collection index
	executor.t.Logf("🚀 TX%d (Collection ~%d) STARTED at %v",
		txn.TxIndex, collectionIndex, time.Now())

	getSetAProgram(executor.t, executor.txnState)

	executor.t.Logf("✅ TX%d (Collection ~%d) COMPLETED at %v",
		txn.TxIndex, collectionIndex, time.Now())

	return nil
}

func (executor *testExecutor) Output() fvm.ProcedureOutput {
	txn := executor.proc.(*fvm.TransactionProcedure)

	return fvm.ProcedureOutput{
		Events: generateEvents(executor.eventsPerTransaction, txn.TxIndex),
		Err:    executor.err,
	}
}

func (vm *testVM) NewExecutor(
	ctx fvm.Context,
	proc fvm.Procedure,
	txnState storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	return &testExecutor{
		testVM:   vm,
		proc:     proc,
		ctx:      ctx,
		txnState: txnState,
	}
}

func (vm *testVM) CallCount() int {
	return int(atomic.LoadInt32(&vm.callCount))
}

func (vm *testVM) Run(
	ctx fvm.Context,
	proc fvm.Procedure,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	database := storage.NewBlockDatabase(
		storageSnapshot,
		proc.ExecutionTime(),
		ctx.DerivedBlockData)

	txn, err := database.NewTransaction(
		proc.ExecutionTime(),
		state.DefaultParameters())
	require.NoError(vm.t, err)

	executor := vm.NewExecutor(ctx, proc, txn)
	err = fvm.Run(executor)
	require.NoError(vm.t, err)

	err = txn.Finalize()
	require.NoError(vm.t, err)

	executionSnapshot, err := txn.Commit()
	require.NoError(vm.t, err)

	return executionSnapshot, executor.Output(), nil
}

func (testVM) GetAccount(
	_ fvm.Context,
	_ flow.Address,
	_ snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	panic("not implemented")
}

func generateEvents(eventCount int, txIndex uint32) []flow.Event {
	events := make([]flow.Event, eventCount)
	for i := 0; i < eventCount; i++ {
		// creating some dummy event
		event := flow.Event{
			Type:             "whatever",
			EventIndex:       uint32(i),
			TransactionIndex: txIndex,
		}
		events[i] = event
	}
	return events
}

type errorVM struct {
	errorAt logical.Time
}

type errorExecutor struct {
	err error
}

func (errorExecutor) Cleanup() {}

func (errorExecutor) Preprocess() error {
	return nil
}

func (e errorExecutor) Execute() error {
	return e.err
}

func (errorExecutor) Output() fvm.ProcedureOutput {
	return fvm.ProcedureOutput{}
}

func (vm errorVM) NewExecutor(
	ctx fvm.Context,
	proc fvm.Procedure,
	txn storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	var err error
	if proc.ExecutionTime() == vm.errorAt {
		err = fmt.Errorf("boom - internal error")
	}

	return errorExecutor{err: err}
}

func (vm errorVM) Run(
	ctx fvm.Context,
	proc fvm.Procedure,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	var err error
	if proc.ExecutionTime() == vm.errorAt {
		err = fmt.Errorf("boom - internal error")
	}
	return &snapshot.ExecutionSnapshot{}, fvm.ProcedureOutput{}, err
}

func (errorVM) GetAccount(
	ctx fvm.Context,
	addr flow.Address,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	panic("not implemented")
}

func getSetAProgram(
	t *testing.T,
	txnState storage.TransactionPreparer,
) {
	loc := common.AddressLocation{
		Name:    "SomeContract",
		Address: common.MustBytesToAddress([]byte{0x1}),
	}
	_, err := txnState.GetOrComputeProgram(
		txnState,
		loc,
		&programLoader{
			load: func() (*derived.Program, error) {
				return &derived.Program{}, nil
			},
		},
	)
	require.NoError(t, err)
}

type programLoader struct {
	load func() (*derived.Program, error)
}

func (p *programLoader) Compute(
	_ state.NestedTransactionPreparer,
	_ common.AddressLocation,
) (
	*derived.Program,
	error,
) {
	return p.load()
}

// TestLogger captures log output for testing and provides methods to verify logged messages.
type TestLogger struct {
	buffer bytes.Buffer
	Logger zerolog.Logger
}

func NewTestLogger() *TestLogger {
	tl := &TestLogger{}
	tl.Logger = zerolog.New(&tl.buffer).Level(zerolog.DebugLevel)
	return tl
}

type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (tl *TestLogger) Logs() []LogEntry {
	var entries []LogEntry
	lines := strings.Split(tl.buffer.String(), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var rawEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rawEntry); err != nil {
			continue
		}
		entry := LogEntry{
			Fields: make(map[string]interface{}),
		}
		for k, v := range rawEntry {
			switch k {
			case "level":
				entry.Level = fmt.Sprintf("%v", v)
			case "message":
				entry.Message = fmt.Sprintf("%v", v)
			default:
				entry.Fields[k] = v
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

func (tl *TestLogger) HasLog(message string) bool {
	return strings.Contains(tl.buffer.String(), message)
}

func (tl *TestLogger) HasLogWithField(message string, fieldName string, fieldValue interface{}) bool {
	for _, entry := range tl.Logs() {
		if strings.Contains(entry.Message, message) {
			if val, ok := entry.Fields[fieldName]; ok {
				if val == fieldValue {
					return true
				}
			}
		}
	}
	return false
}
