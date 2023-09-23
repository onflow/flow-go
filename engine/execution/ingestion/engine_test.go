package ingestion

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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
	executionUnittest "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/engine/testutil/mocklocal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mocks"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storageerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mocks"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	collection1Identity = unittest.IdentityFixture()
	collection2Identity = unittest.IdentityFixture()
	collection3Identity = unittest.IdentityFixture()
	myIdentity          = unittest.IdentityFixture()
)

func init() {
	collection1Identity.Role = flow.RoleCollection
	collection2Identity.Role = flow.RoleCollection
	collection3Identity.Role = flow.RoleCollection
	myIdentity.Role = flow.RoleExecution
}

type testingContext struct {
	t                   *testing.T
	engine              *Engine
	headers             *storage.MockHeaders
	blocks              *storage.MockBlocks
	collections         *mocks.MockCollectionStore
	state               *protocol.State
	conduit             *mocknetwork.Conduit
	collectionConduit   *mocknetwork.Conduit
	computationManager  *computation.ComputationManager
	providerEngine      *provider.ProviderEngine
	executionState      *stateMock.ExecutionState
	snapshot            *protocol.Snapshot
	identity            *flow.Identity
	broadcastedReceipts map[flow.Identifier]*flow.ExecutionReceipt
	collectionRequester *module.MockRequester
	identities          flow.IdentityList
	stopControl         *stop.StopControl
	uploadMgr           *uploader.Manager
	fetcher             *mocks.MockFetcher

	mu *sync.Mutex
}

func runWithEngine(t *testing.T, f func(testingContext)) {

	ctrl := gomock.NewController(t)

	net := mocknetwork.NewMockEngineRegistry(ctrl)
	request := module.NewMockRequester(ctrl)

	// initialize the mocks and engine
	conduit := &mocknetwork.Conduit{}
	collectionConduit := &mocknetwork.Conduit{}

	// generates signing identity including staking key for signing
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLen)
	require.NoError(t, err)
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	myIdentity.StakingPubKey = sk.PublicKey()
	me := mocklocal.NewMockLocal(sk, myIdentity.ID(), t)

	headers := storage.NewMockHeaders(ctrl)
	blocks := storage.NewMockBlocks(ctrl)
	payloads := storage.NewMockPayloads(ctrl)
	collections := mocks.NewMockCollectionStore()

	computationManager := new(computation.ComputationManager)
	providerEngine := new(provider.ProviderEngine)
	protocolState := new(protocol.State)
	executionState := new(stateMock.ExecutionState)
	snapshot := new(protocol.Snapshot)

	var engine *Engine

	defer func() {
		<-engine.Done()
		ctrl.Finish()
		computationManager.AssertExpectations(t)
		protocolState.AssertExpectations(t)
		executionState.AssertExpectations(t)
		providerEngine.AssertExpectations(t)
	}()

	identityListUnsorted := flow.IdentityList{myIdentity, collection1Identity, collection2Identity, collection3Identity}
	identityList := identityListUnsorted.Sort(order.Canonical)

	snapshot.On("Identities", mock.Anything).Return(func(selector flow.IdentityFilter) flow.IdentityList {
		return identityList.Filter(selector)
	}, nil)

	snapshot.On("Identity", mock.Anything).Return(func(nodeID flow.Identifier) *flow.Identity {
		identity, ok := identityList.ByNodeID(nodeID)
		require.Truef(t, ok, "Could not find nodeID %v in identityList", nodeID)
		return identity
	}, nil)

	payloads.EXPECT().Store(gomock.Any(), gomock.Any()).AnyTimes()

	log := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	tracer, err := trace.NewTracer(log, "test", "test", trace.SensitivityCaptureAll)
	require.NoError(t, err)

	request.EXPECT().Force().Return().AnyTimes()

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
	loader := loader.NewLoader(log, protocolState, headers, executionState)

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
		t:                   t,
		engine:              engine,
		headers:             headers,
		blocks:              blocks,
		collections:         collections,
		state:               protocolState,
		collectionRequester: request,
		conduit:             conduit,
		collectionConduit:   collectionConduit,
		computationManager:  computationManager,
		providerEngine:      providerEngine,
		executionState:      executionState,
		snapshot:            snapshot,
		identity:            myIdentity,
		broadcastedReceipts: make(map[flow.Identifier]*flow.ExecutionReceipt),
		identities:          identityList,
		uploadMgr:           uploadMgr,
		stopControl:         stopControl,
		fetcher:             fetcher,

		mu: &sync.Mutex{},
	})

	<-engine.Done()
}

func (ctx *testingContext) assertExecution(
	commits map[flow.Identifier]flow.StateCommitment,
	block *entity.ExecutableBlock,
	previousExecutionResultID flow.Identifier,
	wg *sync.WaitGroup,
) *execution.ComputationResult {
	return ctx.assertSuccessfulBlockComputation(
		commits,
		func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		},
		block,
		previousExecutionResultID,
		false,
		unittest.StateCommitmentFixture(),
		nil)
}

func (ctx *testingContext) assertSuccessfulBlockComputation(
	commits map[flow.Identifier]flow.StateCommitment,
	onPersisted func(blockID flow.Identifier, commit flow.StateCommitment),
	executableBlock *entity.ExecutableBlock,
	previousExecutionResultID flow.Identifier,
	expectBroadcast bool,
	newStateCommitment flow.StateCommitment,
	computationResult *execution.ComputationResult,
) *execution.ComputationResult {
	if computationResult == nil {
		computationResult = executionUnittest.ComputationResultForBlockFixture(
			ctx.t,
			previousExecutionResultID,
			executableBlock)
	}

	if len(computationResult.Chunks) > 0 {
		computationResult.Chunks[len(computationResult.Chunks)-1].EndState = newStateCommitment
	}

	ctx.executionState.
		On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

	ctx.executionState.On("NewStorageSnapshot", mock.Anything).Return(nil)

	matcher := mock.MatchedBy(func(block *entity.ExecutableBlock) bool {
		return block.ID() == executableBlock.ID()
	})
	ctx.computationManager.
		On("ComputeBlock", mock.Anything, mock.Anything, matcher, mock.Anything).
		Return(computationResult, nil)

	mocked := ctx.executionState.
		On("SaveExecutionResults", mock.Anything, computationResult).
		Return(nil)

	mocked.RunFn =
		func(args mock.Arguments) {
			result := args[1].(*execution.ComputationResult)
			blockID := result.ExecutableBlock.Block.Header.ID()
			commit := result.CurrentEndState()

			ctx.mu.Lock()
			commits[blockID] = commit
			ctx.mu.Unlock()
			onPersisted(blockID, commit)
		}

	mocked.ReturnArguments = mock.Arguments{nil}

	ctx.providerEngine.
		On(
			"BroadcastExecutionReceipt",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).
		Run(func(args mock.Arguments) {
			receipt := args[2].(*flow.ExecutionReceipt)

			assert.Equal(ctx.t,
				len(computationResult.ServiceEvents),
				len(receipt.ExecutionResult.ServiceEvents),
			)

			ctx.mu.Lock()
			ctx.broadcastedReceipts[receipt.ExecutionResult.BlockID] = receipt
			ctx.mu.Unlock()
		}).
		Return(expectBroadcast, nil)

	return computationResult
}

func (ctx *testingContext) mockCompuation(
	executableBlock *entity.ExecutableBlock,
	commits map[flow.Identifier]flow.StateCommitment,
	previousExecutionResultID flow.Identifier,
	newStateCommitment flow.StateCommitment,
	wg *sync.WaitGroup,
) *execution.ComputationResult {

	computationResult := executionUnittest.ComputationResultForBlockFixture(
		ctx.t,
		previousExecutionResultID,
		executableBlock)

	if len(computationResult.Chunks) > 0 {
		computationResult.Chunks[len(computationResult.Chunks)-1].EndState = newStateCommitment
	}

	ctx.executionState.On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

	ctx.executionState.On("NewStorageSnapshot", mock.Anything).Return(nil)

	matcher := mock.MatchedBy(func(block *entity.ExecutableBlock) bool {
		return block.ID() == executableBlock.ID()
	})
	ctx.computationManager.
		On("ComputeBlock", mock.Anything, mock.Anything, matcher, mock.Anything).
		Return(computationResult, nil)

	mocked := ctx.executionState.
		On("SaveExecutionResults", mock.Anything, computationResult).
		Return(nil)

	mocked.RunFn =
		func(args mock.Arguments) {
			result := args[1].(*execution.ComputationResult)
			blockID := result.ExecutableBlock.Block.Header.ID()
			commit := result.CurrentEndState()

			ctx.mu.Lock()
			commits[blockID] = commit
			ctx.mu.Unlock()
			wg.Done()
		}

	mocked.ReturnArguments = mock.Arguments{nil}

	return computationResult
}

func (ctx *testingContext) stateCommitmentExist(blockID flow.Identifier, commit flow.StateCommitment) {
	ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blockID).Return(commit, nil)
}

func (ctx *testingContext) mockStateCommitsWithMap(commits map[flow.Identifier]flow.StateCommitment) {
	mocked := ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, mock.Anything)
	// https://github.com/stretchr/testify/issues/350#issuecomment-570478958
	mocked.RunFn = func(args mock.Arguments) {

		blockID := args[1].(flow.Identifier)
		ctx.mu.Lock()
		commit, ok := commits[blockID]
		ctx.mu.Unlock()
		if ok {
			mocked.ReturnArguments = mock.Arguments{commit, nil}
			return
		}

		mocked.ReturnArguments = mock.Arguments{flow.StateCommitment{}, storageerr.ErrNotFound}
	}
}

func (ctx *testingContext) mockStateCommitmentByBlockID(store *mocks.MockBlockStore) {
	mocked := ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, mock.Anything)
	// https://github.com/stretchr/testify/issues/350#issuecomment-570478958
	mocked.RunFn = func(args mock.Arguments) {
		blockID := args[1].(flow.Identifier)
		fmt.Println("mockStateCommitmentByBlockID", blockID)
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
		defer func() {
			wg.Done()
		}()

		result := args[1].(*execution.ComputationResult)

		err := store.MarkExecuted(result)
		if err != nil {
			mocked.ReturnArguments = mock.Arguments{err}
			return
		}
		fmt.Println("mockSaveExecutionResults", result.ExecutableBlock.ID())
		mocked.ReturnArguments = mock.Arguments{nil}
	}
}

func newBlockWithCollection(parent *flow.Header, parentState flow.StateCommitment) *entity.ExecutableBlock {
	col := unittest.CollectionFixture(1)
	gua := col.Guarantee()

	colSigner := unittest.IdentifierFixture()
	block := unittest.ExecutableBlockFixtureWithParent([][]flow.Identifier{{colSigner}}, parent, unittest.StateCommitmentPointerFixture())
	block.Block.Payload.Guarantees[0] = &gua
	block.Block.Header.PayloadHash = block.Block.Payload.Hash()
	return block
}

// TestExecuteOneBlock verifies after collection is received,
// block is executed, uploaded, and broadcasted
func TestExecuteOneBlock(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		col := unittest.CollectionFixture(1)
		// Root <- A
		blockA := makeBlockWithCollection(store.RootBlock, &col)
		result := store.CreateBlockAndMockResult(t, blockA)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything).Return(nil)

		// receive block
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		wg := sync.WaitGroup{}
		wg.Add(1) // wait for block B to be executed

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

		unittest.AssertReturnsBefore(t, wg.Wait, 3*time.Second)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col.ID()))

		// verify block is executed
		store.AssertExecuted(t, blockA.ID())
	})
}

// verify block will be executed if collection is received first
func TestExecuteBlocks(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {
		store := mocks.NewMockBlockStore(t)

		col1 := unittest.CollectionFixture(1)
		col2 := unittest.CollectionFixture(1)
		// Root <- A[C1] <- B[C2]
		blockA := makeBlockWithCollection(store.RootBlock, &col1)
		blockB := makeBlockWithCollection(blockA.Block.Header, &col2)
		store.CreateBlockAndMockResult(t, blockA)
		store.CreateBlockAndMockResult(t, blockB)

		ctx.mockStateCommitmentByBlockID(store)
		ctx.mockGetExecutionResultID(store)
		ctx.executionState.On("NewStorageSnapshot", mock.Anything).Return(nil)
		ctx.mockComputeBlock(store)
		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

		// receive block
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		wg := sync.WaitGroup{}
		wg.Add(2) // wait for 2 blocks to be executed

		ctx.mockComputeBlock(store)
		ctx.mockSaveExecutionResults(store, &wg)

		require.NoError(t, ctx.engine.handleCollection(unittest.IdentifierFixture(), &col2))
		require.NoError(t, ctx.engine.handleCollection(unittest.IdentifierFixture(), &col1))

		unittest.AssertReturnsBefore(t, wg.Wait, 3*time.Second)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col1.ID()))
		require.True(t, ctx.fetcher.IsFetched(col2.ID()))

		// verify block is executed
		store.AssertExecuted(t, blockA.ID())
		store.AssertExecuted(t, blockB.ID())
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
		ctx.executionState.On("NewStorageSnapshot", mock.Anything).Return(nil)
		ctx.mockComputeBlock(store)

		// receiving block A and B will not trigger any execution
		// because A is missing collection C1, B is waiting for A to be executed
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		wg := sync.WaitGroup{}
		wg.Add(2)
		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		ctx.mockComputeBlock(store)
		ctx.mockSaveExecutionResults(store, &wg)

		// receiving collection C1 will execute both A and B
		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col1)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col1.ID()))
		require.False(t, ctx.fetcher.IsFetched(col2.ID()))

		// verify block is executed
		store.AssertExecuted(t, blockA.ID())
		store.AssertExecuted(t, blockB.ID())
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
		ctx.executionState.On("NewStorageSnapshot", mock.Anything).Return(nil)

		// receive block
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		// receive block again before collection is received
		err = ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		wg := sync.WaitGroup{}
		wg.Add(1) // wait for block B to be executed
		ctx.mockComputeBlock(store)
		ctx.mockSaveExecutionResults(store, &wg)
		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col)
		require.NoError(t, err)

		// receiving collection again before block is executed
		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 3*time.Second)

		// receiving collection again after block is executed
		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col)
		require.NoError(t, err)

		// verify collection is fetched
		require.True(t, ctx.fetcher.IsFetched(col.ID()))

		// verify block is executed
		store.AssertExecuted(t, blockA.ID())
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
		ctx.executionState.On("NewStorageSnapshot", mock.Anything).Return(nil)
		ctx.mockComputeBlock(store)

		// receive blocks
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		err = ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		wg := sync.WaitGroup{}
		wg.Add(2) // wait for block B to be executed
		ctx.providerEngine.On("BroadcastExecutionReceipt", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		ctx.mockComputeBlock(store)
		ctx.mockSaveExecutionResults(store, &wg)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col1)
		require.NoError(t, err)

		err = ctx.engine.handleCollection(unittest.IdentifierFixture(), &col2)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 3*time.Second)

		// verify block is executed
		store.AssertExecuted(t, blockA.ID())
		store.AssertExecuted(t, blockB.ID())
	})
}

func logBlocks(blocks map[string]*entity.ExecutableBlock) {
	log := unittest.Logger()
	for name, b := range blocks {
		log.Debug().Msgf("creating blocks for testing, block %v's ID:%v", name, b.ID())
	}
}

func TestStopAtHeight(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {

		blockSealed := unittest.BlockHeaderFixture()

		blocks := make(map[string]*entity.ExecutableBlock)
		blocks["A"] = unittest.ExecutableBlockFixtureWithParent(nil, blockSealed, unittest.StateCommitmentPointerFixture())

		// none of the blocks has any collection, so state is essentially the same
		blocks["B"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["A"].Block.Header, blocks["A"].StartState)
		blocks["C"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["B"].Block.Header, blocks["A"].StartState)
		blocks["D"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["C"].Block.Header, blocks["A"].StartState)

		// stop at block C
		err := ctx.stopControl.SetStopParameters(stop.StopParameters{
			StopBeforeHeight: blockSealed.Height + 3,
		})
		require.NoError(t, err)

		// log the blocks, so that we can link the block ID in the log with the blocks in tests
		logBlocks(blocks)

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blocks["A"].Block.Header.ParentID] = *blocks["A"].StartState

		ctx.mockStateCommitsWithMap(commits)

		wg := sync.WaitGroup{}
		onPersisted := func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		}

		ctx.blocks.EXPECT().ByID(blocks["A"].ID()).Return(blocks["A"].Block, nil)
		ctx.blocks.EXPECT().ByID(blocks["B"].ID()).Return(blocks["B"].Block, nil)
		ctx.blocks.EXPECT().ByID(blocks["C"].ID()).Times(0)
		ctx.blocks.EXPECT().ByID(blocks["D"].ID()).Times(0)

		ctx.assertSuccessfulBlockComputation(
			commits,
			onPersisted,
			blocks["A"],
			unittest.IdentifierFixture(),
			true,
			*blocks["A"].StartState,
			nil)
		ctx.assertSuccessfulBlockComputation(
			commits,
			onPersisted,
			blocks["B"],
			unittest.IdentifierFixture(),
			true,
			*blocks["B"].StartState,
			nil)

		assert.False(t, ctx.stopControl.IsExecutionStopped())

		wg.Add(1)
		ctx.engine.BlockProcessable(blocks["A"].Block.Header, nil)
		wg.Add(1)
		ctx.engine.BlockProcessable(blocks["B"].Block.Header, nil)

		ctx.engine.BlockProcessable(blocks["C"].Block.Header, nil)
		ctx.engine.BlockProcessable(blocks["D"].Block.Header, nil)

		// wait until all 4 blocks have been executed
		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		// we don't pause until a block has been finalized
		assert.False(t, ctx.stopControl.IsExecutionStopped())

		ctx.stopControl.BlockFinalizedForTesting(blocks["A"].Block.Header)
		ctx.stopControl.BlockFinalizedForTesting(blocks["B"].Block.Header)

		assert.False(t, ctx.stopControl.IsExecutionStopped())
		ctx.stopControl.BlockFinalizedForTesting(blocks["C"].Block.Header)
		assert.True(t, ctx.stopControl.IsExecutionStopped())

		ctx.stopControl.BlockFinalizedForTesting(blocks["D"].Block.Header)

		_, more := <-ctx.engine.Done() // wait for all the blocks to be processed
		assert.False(t, more)

		var ok bool
		for c := range commits {
			fmt.Printf("%s => ok\n", c.String())
		}
		_, ok = commits[blocks["A"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["B"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["C"].ID()]
		require.False(t, ok)
		_, ok = commits[blocks["D"].ID()]
		require.False(t, ok)

		// make sure C and D were not executed
		ctx.computationManager.AssertNotCalled(
			t,
			"ComputeBlock",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(eb *entity.ExecutableBlock) bool {
				return eb.ID() == blocks["C"].ID()
			}),
			mock.Anything)

		ctx.computationManager.AssertNotCalled(
			t,
			"ComputeBlock",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(eb *entity.ExecutableBlock) bool {
				return eb.ID() == blocks["D"].ID()
			}),
			mock.Anything)
	})
}

// TestStopAtHeightRaceFinalization test a possible race condition which happens
// when block at stop height N is finalized while N-1 is being executed.
// If execution finishes exactly between finalization checking execution state and
// setting block ID to crash, it's possible to miss and never actually stop the EN
func TestStopAtHeightRaceFinalization(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {

		blockSealed := unittest.BlockHeaderFixture()

		blocks := make(map[string]*entity.ExecutableBlock)
		blocks["A"] = unittest.ExecutableBlockFixtureWithParent(nil, blockSealed, unittest.StateCommitmentPointerFixture())
		blocks["B"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["A"].Block.Header, nil)
		blocks["C"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["B"].Block.Header, nil)

		// stop at block B, so B-1 (A) will be last executed
		err := ctx.stopControl.SetStopParameters(stop.StopParameters{
			StopBeforeHeight: blocks["B"].Height(),
		})
		require.NoError(t, err)

		// log the blocks, so that we can link the block ID in the log with the blocks in tests
		logBlocks(blocks)

		commits := make(map[flow.Identifier]flow.StateCommitment)

		ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blocks["A"].Block.Header.ParentID).Return(
			*blocks["A"].StartState, nil,
		)

		executionWg := sync.WaitGroup{}
		onPersisted := func(blockID flow.Identifier, commit flow.StateCommitment) {
			executionWg.Done()
		}

		ctx.blocks.EXPECT().ByID(blocks["A"].ID()).Return(blocks["A"].Block, nil)

		ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blocks["A"].ID()).Return(nil, storageerr.ErrNotFound).Once()

		// second call should come from finalization handler, which should wait for execution to finish before returning.
		// This way we simulate possible race condition when block execution finishes exactly in the middle of finalization handler
		// setting stopping blockID
		finalizationWg := sync.WaitGroup{}
		ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blocks["A"].ID()).Run(func(args mock.Arguments) {
			executionWg.Wait()
			finalizationWg.Done()
		}).Return(nil, storageerr.ErrNotFound).Once()

		// second call from finalization handler, third overall
		ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blocks["A"].ID()).
			Return(flow.StateCommitment{}, nil).Maybe()

		ctx.assertSuccessfulBlockComputation(
			commits,
			onPersisted,
			blocks["A"],
			unittest.IdentifierFixture(),
			true,
			*blocks["A"].StartState,
			nil)

		assert.False(t, ctx.stopControl.IsExecutionStopped())

		executionWg.Add(1)
		ctx.engine.BlockProcessable(blocks["A"].Block.Header, nil)
		ctx.engine.BlockProcessable(blocks["B"].Block.Header, nil)

		assert.False(t, ctx.stopControl.IsExecutionStopped())

		finalizationWg.Add(1)
		ctx.stopControl.BlockFinalizedForTesting(blocks["B"].Block.Header)

		finalizationWg.Wait()
		executionWg.Wait()

		_, more := <-ctx.engine.Done() // wait for all the blocks to be processed
		assert.False(t, more)

		assert.True(t, ctx.stopControl.IsExecutionStopped())

		var ok bool

		// make sure B and C were not executed
		_, ok = commits[blocks["A"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["B"].ID()]
		require.False(t, ok)
		_, ok = commits[blocks["C"].ID()]
		require.False(t, ok)

		ctx.computationManager.AssertNotCalled(
			t,
			"ComputeBlock",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(eb *entity.ExecutableBlock) bool {
				return eb.ID() == blocks["B"].ID()
			}),
			mock.Anything)

		ctx.computationManager.AssertNotCalled(
			t,
			"ComputeBlock",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(eb *entity.ExecutableBlock) bool {
				return eb.ID() == blocks["C"].ID()
			}),
			mock.Anything)
	})
}

func TestExecutionGenerationResultsAreChained(t *testing.T) {

	execState := new(stateMock.ExecutionState)

	ctrl := gomock.NewController(t)
	me := module.NewMockLocal(ctrl)

	startState := unittest.StateCommitmentFixture()
	executableBlock := unittest.ExecutableBlockFixture(
		[][]flow.Identifier{{collection1Identity.NodeID},
			{collection1Identity.NodeID}},
		&startState,
	)
	previousExecutionResultID := unittest.IdentifierFixture()

	cr := executionUnittest.ComputationResultFixture(
		t,
		previousExecutionResultID,
		nil)
	cr.ExecutableBlock = executableBlock

	execState.
		On("SaveExecutionResults", mock.Anything, cr).
		Return(nil)

	e := Engine{
		execState: execState,
		tracer:    trace.NewNoopTracer(),
		metrics:   metrics.NewNoopCollector(),
		me:        me,
	}

	err := e.saveExecutionResults(context.Background(), cr)
	assert.NoError(t, err)

	execState.AssertExpectations(t)
}

// TestExecutedBlockUploadedFailureDoesntBlock tests that block processing continues even the
// uploader fails with an error
func TestExecutedBlockUploadedFailureDoesntBlock(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {

		// A <- B
		blockA := unittest.BlockHeaderFixture()
		blockB := unittest.ExecutableBlockFixtureWithParent(nil, blockA, unittest.StateCommitmentPointerFixture())

		previousExecutionResultID := unittest.IdentifierFixture()

		computationResultB := executionUnittest.ComputationResultForBlockFixture(
			t,
			previousExecutionResultID,
			blockB)

		// configure upload manager with a single uploader that returns an error
		uploader1 := uploadermock.NewUploader(ctx.t)
		uploader1.On("Upload", computationResultB).Return(fmt.Errorf("error uploading")).Once()
		ctx.uploadMgr.AddUploader(uploader1)

		// blockA's start state is its parent's state commitment,
		// and blockA's parent has been executed.
		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockB.Block.Header.ParentID] = *blockB.StartState
		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits)

		ctx.assertSuccessfulBlockComputation(
			commits,
			func(blockID flow.Identifier, commit flow.StateCommitment) {
				wg.Done()
			},
			blockB,
			previousExecutionResultID,
			true,
			*blockB.StartState,
			computationResultB)

		wg.Add(1) // wait for block B to be executed
		err := ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		_, more := <-ctx.engine.Done() // wait for all the blocks to be processed
		require.False(t, more)

		_, ok := commits[blockB.ID()]
		require.True(t, ok)

	})
}

type collectionHandler interface {
	handleCollection(flow.Identifier, *flow.Collection) error
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
