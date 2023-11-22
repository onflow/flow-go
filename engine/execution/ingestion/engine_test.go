package ingestion

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"

	enginePkg "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	computation "github.com/onflow/flow-go/engine/execution/computation/mock"
	"github.com/onflow/flow-go/engine/execution/ingestion/loader"
	"github.com/onflow/flow-go/engine/execution/ingestion/mocks"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader"
	uploadermock "github.com/onflow/flow-go/engine/execution/ingestion/uploader/mock"
	provider "github.com/onflow/flow-go/engine/execution/provider/mock"
	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/engine/testutil/mocklocal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storageerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type testingContext struct {
	t                  *testing.T
	engine             *Engine
	headers            *storage.Headers
	blocks             *storage.Blocks
	collections        *mocks.MockCollectionStore
	state              *protocol.State
	computationManager *computation.ComputationManager
	providerEngine     *provider.ProviderEngine
	executionState     *stateMock.ExecutionState
	stopControl        *stop.StopControl
	uploadMgr          *uploader.Manager
	fetcher            *mocks.MockFetcher

	mu *sync.Mutex
}

func runWithEngine(t *testing.T, f func(testingContext)) {

	net := new(mocknetwork.EngineRegistry)

	// generates signing identity including staking key for signing
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLen)
	require.NoError(t, err)
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	myIdentity := unittest.IdentityFixture()
	myIdentity.Role = flow.RoleExecution
	myIdentity.StakingPubKey = sk.PublicKey()

	me := mocklocal.NewMockLocal(sk, myIdentity.ID(), t)

	headers := storage.NewHeaders(t)
	blocks := storage.NewBlocks(t)
	collections := mocks.NewMockCollectionStore()

	computationManager := computation.NewComputationManager(t)
	providerEngine := provider.NewProviderEngine(t)
	protocolState := protocol.NewState(t)
	executionState := stateMock.NewExecutionState(t)

	var engine *Engine

	defer func() {
		<-engine.Done()
		computationManager.AssertExpectations(t)
		protocolState.AssertExpectations(t)
		executionState.AssertExpectations(t)
		providerEngine.AssertExpectations(t)
	}()

	log := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	tracer, err := trace.NewTracer(log, "test", "test", trace.SensitivityCaptureAll)
	require.NoError(t, err)

	unit := enginePkg.NewUnit()
	stopControl := stop.NewStopControl(
		unit,
		time.Second,
		zerolog.Nop(),
		executionState,
		headers,
		nil,
		nil,
		&flow.Header{Height: 1},
		false,
		false,
	)

	uploadMgr := uploader.NewManager(trace.NewNoopTracer())

	fetcher := mocks.NewMockFetcher()
	loader := loader.NewUnexecutedLoader(log, protocolState, headers, executionState)

	engine, err = New(
		unit,
		log,
		net,
		me,
		fetcher,
		headers,
		blocks,
		collections,
		computationManager,
		providerEngine,
		executionState,
		metrics,
		tracer,
		false,
		nil,
		uploadMgr,
		stopControl,
		loader,
	)
	require.NoError(t, err)

	f(testingContext{
		t:                  t,
		engine:             engine,
		headers:            headers,
		blocks:             blocks,
		collections:        collections,
		state:              protocolState,
		computationManager: computationManager,
		providerEngine:     providerEngine,
		executionState:     executionState,
		uploadMgr:          uploadMgr,
		stopControl:        stopControl,
		fetcher:            fetcher,

		mu: &sync.Mutex{},
	})

	<-engine.Done()
}

// TestExecuteOneBlock verifies after collection is received,
// block is executed, uploaded, and broadcasted
func TestExecuteOneBlock(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		// create a mocked storage that has similar behavior as the real execution state.
		// the mocked storage allows us to prepare results for the prepared blocks, so that
		// the mocked methods know what to return, and it also allows us to verify that the
		// mocked API is called with correct data.
		store := mocks.NewMockBlockStore(t)

		col := unittest.CollectionFixture(1)
		// Root <- A
		blockA := makeBlockWithCollection(store.RootBlock, &col)
		result := store.CreateBlockAndMockResult(t, blockA)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// receive block
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		wg := sync.WaitGroup{}
		wg.Add(1) // wait for block A to be executed

		ctx.mockComputeBlock(store)
		ctx.mockSaveExecutionResults(store, &wg)

		// verify upload will be called
		uploader := uploadermock.NewUploader(ctx.t)
		uploader.On("Upload", result).Return(nil).Once()
		ctx.uploadMgr.AddUploader(uploader)

		// verify broadcast will be called
		ctx.providerEngine.On("BroadcastExecutionReceipt",
			mock.Anything,
			blockA.Block.Header.Height,
			result.ExecutionReceipt).Return(true, nil).Once()

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col.ID()))

		// verify block is executed
		store.AssertExecuted(t, "A", blockA.ID())
	})
}

// verify block will be executed if collection is received first
func TestExecuteBlocks(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		col1 := unittest.CollectionFixture(1)
		col2 := unittest.CollectionFixture(1)
		// Root <- A[C1] <- B[C2]
		// prepare two blocks, so that receiving C2 before C1 won't trigger any block to be executed,
		// which creates the case where C2 collection is received first, and block B will become
		// executable as soon as its parent block A is executed.
		blockA := makeBlockWithCollection(store.RootBlock, &col1)
		blockB := makeBlockWithCollection(blockA.Block.Header, &col2)
		store.CreateBlockAndMockResult(t, blockA)
		store.CreateBlockAndMockResult(t, blockB)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

		// receive block
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		wg.Add(2) // wait for 2 blocks to be executed
		ctx.mockSaveExecutionResults(store, &wg)

		require.NoError(t, ctx.engine.handleCollection(unittest.IdentifierFixture(), &col2))
		require.NoError(t, ctx.engine.handleCollection(unittest.IdentifierFixture(), &col1))

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col1.ID()))
		require.True(t, ctx.fetcher.IsFetched(col2.ID()))

		// verify block is executed
		store.AssertExecuted(t, "A", blockA.ID())
		store.AssertExecuted(t, "B", blockB.ID())
	})
}

// verify block will be executed if collection is already in storage
func TestExecuteNextBlockIfCollectionIsReady(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		col1 := unittest.CollectionFixture(1)
		col2 := unittest.CollectionFixture(1)

		// Root <- A[C1] <- B[C2]
		blockA := makeBlockWithCollection(store.RootBlock, &col1)
		blockB := makeBlockWithCollection(blockA.Block.Header, &col2)
		store.CreateBlockAndMockResult(t, blockA)
		store.CreateBlockAndMockResult(t, blockB)

		// C2 is available in storage
		require.NoError(t, ctx.collections.Store(&col2))

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// receiving block A and B will not trigger any execution
		// because A is missing collection C1, B is waiting for A to be executed
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		wg.Add(2) // waiting for A and B to be executed
		ctx.mockSaveExecutionResults(store, &wg)

		// receiving collection C1 will execute both A and B
		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col1)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col1.ID()))
		require.False(t, ctx.fetcher.IsFetched(col2.ID()))

		// verify block is executed
		store.AssertExecuted(t, "A", blockA.ID())
		store.AssertExecuted(t, "B", blockB.ID())
	})
}

// verify block will only be executed once even if block or collection are received multiple times
func TestExecuteBlockOnlyOnce(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		col := unittest.CollectionFixture(1)
		// Root <- A[C]
		blockA := makeBlockWithCollection(store.RootBlock, &col)
		store.CreateBlockAndMockResult(t, blockA)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// receive block
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		// receive block again before collection is received
		err = ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		wg.Add(1) // wait for block A to be executed
		ctx.mockSaveExecutionResults(store, &wg)
		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col)
		require.NoError(t, err)

		// receiving collection again before block is executed
		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// receiving collection again after block is executed
		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col)
		require.NoError(t, err)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col.ID()))

		// verify block is executed
		store.AssertExecuted(t, "A", blockA.ID())
	})
}

// given two blocks depend on the same root block and contain same collections,
// receiving all collections will trigger the execution of both blocks concurrently
func TestExecuteForkConcurrently(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		// create A and B that have the same collections and same parent
		// Root <- A[C1, C2]
		//      <- B[C1, C2]
		col1 := unittest.CollectionFixture(1)
		col2 := unittest.CollectionFixture(1)

		blockA := makeBlockWithCollection(store.RootBlock, &col1, &col2)
		blockB := makeBlockWithCollection(store.RootBlock, &col1, &col2)
		store.CreateBlockAndMockResult(t, blockA)
		store.CreateBlockAndMockResult(t, blockB)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// receive blocks
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col1)
		require.NoError(t, err)

		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		wg.Add(2) // wait for A and B to be executed
		ctx.mockSaveExecutionResults(store, &wg)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col2)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// verify block is executed
		store.AssertExecuted(t, "A", blockA.ID())
		store.AssertExecuted(t, "B", blockB.ID())
	})
}

// verify block will be executed in order
func TestExecuteBlockInOrder(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)
		// create A and B that have the same collections and same parent
		// Root <- A[C1, C2]
		//      <- B[C2] <- C[C3]
		// verify receiving C3, C1, then C2 will trigger all blocks to be executed
		col1 := unittest.CollectionFixture(1)
		col2 := unittest.CollectionFixture(1)
		col3 := unittest.CollectionFixture(1)

		blockA := makeBlockWithCollection(store.RootBlock, &col1, &col2)
		blockB := makeBlockWithCollection(store.RootBlock, &col2)
		blockC := makeBlockWithCollection(store.RootBlock, &col3)
		store.CreateBlockAndMockResult(t, blockA)
		store.CreateBlockAndMockResult(t, blockB)
		store.CreateBlockAndMockResult(t, blockC)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// receive blocks
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockC.Block)
		require.NoError(t, err)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col3)
		require.NoError(t, err)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col1)
		require.NoError(t, err)

		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		wg.Add(3) // waiting for A, B, C to be executed
		ctx.mockSaveExecutionResults(store, &wg)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col2)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// verify block is executed
		store.AssertExecuted(t, "A", blockA.ID())
		store.AssertExecuted(t, "B", blockB.ID())
		store.AssertExecuted(t, "C", blockC.ID())
	})
}

func logBlocks(blocks map[string]*entity.ExecutableBlock) {
	log := unittest.Logger()
	for name, b := range blocks {
		log.Debug().Msgf("creating blocks for testing, block %v's ID:%v", name, b.ID())
	}
}

// verify that when blocks above the stop height are finalized, they won't
// be executed
func TestStopAtHeightWhenFinalizedBeforeExecuted(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		// this collection is used as trigger of execution
		executionTrigger := unittest.CollectionFixture(1)
		blockA := makeBlockWithCollection(store.RootBlock, &executionTrigger)
		blockB := makeBlockWithCollection(blockA.Block.Header)
		blockC := makeBlockWithCollection(blockB.Block.Header)
		blockD := makeBlockWithCollection(blockC.Block.Header)

		store.CreateBlockAndMockResult(t, blockA)
		store.CreateBlockAndMockResult(t, blockB)
		store.CreateBlockAndMockResult(t, blockC)
		store.CreateBlockAndMockResult(t, blockD)

		stopHeight := store.RootBlock.Height + 3
		require.Equal(t, stopHeight, blockC.Block.Header.Height) // stop at C (C will not be executed)
		err := ctx.stopControl.SetStopParameters(stop.StopParameters{
			StopBeforeHeight: stopHeight,
		})
		require.NoError(t, err)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// receive blocks
		err = ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockC.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockD.Block)
		require.NoError(t, err)

		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		wg.Add(2) // only 2 blocks (A, B) will be executed
		ctx.mockSaveExecutionResults(store, &wg)

		// all blocks finalized
		ctx.stopControl.BlockFinalizedForTesting(blockA.Block.Header)
		ctx.stopControl.BlockFinalizedForTesting(blockB.Block.Header)
		ctx.stopControl.BlockFinalizedForTesting(blockC.Block.Header)
		ctx.stopControl.BlockFinalizedForTesting(blockD.Block.Header)

		// receiving the colleciton to trigger all blocks to be executed
		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &executionTrigger)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// since stop height is C, verify that only A and B are executed, C and D are not executed
		store.AssertExecuted(t, "A", blockA.ID())
		store.AssertExecuted(t, "B", blockB.ID())

		store.AssertNotExecuted(t, "C", blockC.ID())
		store.AssertNotExecuted(t, "D", blockD.ID())
	})
}

// verify that blocks above the stop height won't be executed, even if they are
// later they got finalized
func TestStopAtHeightWhenExecutedBeforeFinalized(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		blockA := makeBlockWithCollection(store.RootBlock)
		blockB := makeBlockWithCollection(blockA.Block.Header)
		blockC := makeBlockWithCollection(blockB.Block.Header)
		blockD := makeBlockWithCollection(blockC.Block.Header)

		store.CreateBlockAndMockResult(t, blockA)
		store.CreateBlockAndMockResult(t, blockB)
		store.CreateBlockAndMockResult(t, blockC)
		store.CreateBlockAndMockResult(t, blockD)

		stopHeight := store.RootBlock.Height + 3
		require.Equal(t, stopHeight, blockC.Block.Header.Height) // stop at C (C will not be executed)
		err := ctx.stopControl.SetStopParameters(stop.StopParameters{
			StopBeforeHeight: stopHeight,
		})
		require.NoError(t, err)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		wg.Add(2) // waiting for only A, B to be executed
		ctx.mockSaveExecutionResults(store, &wg)

		// receive blocks
		err = ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockC.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockD.Block)
		require.NoError(t, err)

		// all blocks finalized
		ctx.stopControl.BlockFinalizedForTesting(blockA.Block.Header)
		ctx.stopControl.BlockFinalizedForTesting(blockB.Block.Header)
		ctx.stopControl.BlockFinalizedForTesting(blockC.Block.Header)
		ctx.stopControl.BlockFinalizedForTesting(blockD.Block.Header)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// since stop height is C, verify that only A and B are executed, C and D are not executed
		store.AssertExecuted(t, "A", blockA.ID())
		store.AssertExecuted(t, "B", blockB.ID())

		store.AssertNotExecuted(t, "C", blockC.ID())
		store.AssertNotExecuted(t, "D", blockD.ID())
	})
}

// verify that when blocks execution and finalization happen concurrently
func TestStopAtHeightWhenExecutionFinalization(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		// Root <- A <- B (stop height, won't execute) <- C
		// verify when executing A and finalizing B happens concurrently,
		// still won't allow B and C to be executed
		blockA := makeBlockWithCollection(store.RootBlock)
		blockB := makeBlockWithCollection(blockA.Block.Header)
		blockC := makeBlockWithCollection(blockB.Block.Header)

		store.CreateBlockAndMockResult(t, blockA)
		store.CreateBlockAndMockResult(t, blockB)
		store.CreateBlockAndMockResult(t, blockC)

		err := ctx.stopControl.SetStopParameters(stop.StopParameters{
			StopBeforeHeight: blockB.Block.Header.Height,
		})
		require.NoError(t, err)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		// waiting for:
		// 1. A, B, C to be handled
		// 2. A, B, C to be finalized
		// 3. only A to be executed
		wg.Add(3)
		ctx.mockSaveExecutionResults(store, &wg)

		// receive blocks
		go func(wg *sync.WaitGroup) {
			err = ctx.engine.handleBlock(context.Background(), blockA.Block)
			require.NoError(t, err)

			err = ctx.engine.handleBlock(context.Background(), blockB.Block)
			require.NoError(t, err)

			err = ctx.engine.handleBlock(context.Background(), blockC.Block)
			require.NoError(t, err)
			wg.Done()
		}(&wg)

		go func(wg *sync.WaitGroup) {
			// all blocks finalized
			ctx.stopControl.BlockFinalizedForTesting(blockA.Block.Header)
			ctx.stopControl.BlockFinalizedForTesting(blockB.Block.Header)
			ctx.stopControl.BlockFinalizedForTesting(blockC.Block.Header)
			wg.Done()
		}(&wg)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// since stop height is C, verify that only A and B are executed, C and D are not executed
		store.AssertExecuted(t, "A", blockA.ID())
		store.AssertNotExecuted(t, "B", blockB.ID())
		store.AssertNotExecuted(t, "C", blockC.ID())
	})
}

// TestExecutedBlockUploadedFailureDoesntBlock tests that block processing continues even the
// uploader fails with an error
func TestExecutedBlockUploadedFailureDoesntBlock(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		col := unittest.CollectionFixture(1)
		// Root <- A
		blockA := makeBlockWithCollection(store.RootBlock, &col)
		result := store.CreateBlockAndMockResult(t, blockA)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// receive block
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		ctx.mockComputeBlock(store)
		wg := sync.WaitGroup{}
		wg.Add(1) // wait for block A to be executed
		ctx.mockSaveExecutionResults(store, &wg)

		// verify upload will fail
		uploader1 := uploadermock.NewUploader(ctx.t)
		uploader1.On("Upload", result).Return(fmt.Errorf("error uploading")).Once()
		ctx.uploadMgr.AddUploader(uploader1)

		// verify broadcast will be called
		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col.ID()))

		// verify block is executed
		store.AssertExecuted(t, "A", blockA.ID())
	})
}

func makeCollection() (*flow.Collection, *flow.CollectionGuarantee) {
	col := unittest.CollectionFixture(1)
	gua := col.Guarantee()
	return &col, &gua
}

func makeBlockWithCollection(parent *flow.Header, cols ...*flow.Collection) *entity.ExecutableBlock {
	block := unittest.BlockWithParentFixture(parent)
	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, len(block.Payload.Guarantees))
	for _, col := range cols {
		g := col.Guarantee()
		block.Payload.Guarantees = append(block.Payload.Guarantees, &g)

		cc := &entity.CompleteCollection{
			Guarantee:    &g,
			Transactions: col.Transactions,
		}
		completeCollections[col.ID()] = cc
	}
	block.Header.PayloadHash = block.Payload.Hash()

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: completeCollections,
		StartState:          unittest.StateCommitmentPointerFixture(),
	}
	return executableBlock
}

func (ctx *testingContext) mockStateCommitmentByBlockID(store *mocks.MockBlockStore) {
	mocked := ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, mock.Anything)
	// https://github.com/stretchr/testify/issues/350#issuecomment-570478958
	mocked.RunFn = func(args mock.Arguments) {
		blockID := args[1].(flow.Identifier)
		result, err := store.GetExecuted(blockID)
		if err != nil {
			mocked.ReturnArguments = mock.Arguments{flow.StateCommitment{}, storageerr.ErrNotFound}
			return
		}
		mocked.ReturnArguments = mock.Arguments{result.Result.CurrentEndState(), nil}
	}
}

func (ctx *testingContext) mockGetExecutionResultID(store *mocks.MockBlockStore) {

	mocked := ctx.executionState.On("GetExecutionResultID", mock.Anything, mock.Anything)
	mocked.RunFn = func(args mock.Arguments) {
		blockID := args[1].(flow.Identifier)
		blockResult, err := store.GetExecuted(blockID)
		if err != nil {
			mocked.ReturnArguments = mock.Arguments{nil, storageerr.ErrNotFound}
			return
		}

		mocked.ReturnArguments = mock.Arguments{
			blockResult.Result.ExecutionReceipt.ExecutionResult.ID(), nil}
	}
}

func (ctx *testingContext) mockComputeBlock(store *mocks.MockBlockStore) {
	mocked := ctx.computationManager.On("ComputeBlock", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mocked.RunFn = func(args mock.Arguments) {
		block := args[2].(*entity.ExecutableBlock)
		blockResult, ok := store.ResultByBlock[block.ID()]
		if !ok {
			mocked.ReturnArguments = mock.Arguments{nil, fmt.Errorf("block %s not found", block.ID())}
			return
		}
		mocked.ReturnArguments = mock.Arguments{blockResult.Result, nil}
	}
}

func (ctx *testingContext) mockSaveExecutionResults(store *mocks.MockBlockStore, wg *sync.WaitGroup) {
	mocked := ctx.executionState.
		On("SaveExecutionResults", mock.Anything, mock.Anything)

	mocked.RunFn = func(args mock.Arguments) {
		result := args[1].(*execution.ComputationResult)

		err := store.MarkExecuted(result)
		if err != nil {
			mocked.ReturnArguments = mock.Arguments{err}
			wg.Done()
			return
		}
		mocked.ReturnArguments = mock.Arguments{nil}
		wg.Done()
	}
}
