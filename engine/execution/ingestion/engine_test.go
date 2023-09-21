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
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader"
	uploadermock "github.com/onflow/flow-go/engine/execution/ingestion/uploader/mock"
	provider "github.com/onflow/flow-go/engine/execution/provider/mock"
	"github.com/onflow/flow-go/engine/execution/state"
	stateMock "github.com/onflow/flow-go/engine/execution/state/mock"
	executionUnittest "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/engine/testutil/mocklocal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
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

// ExecutionState is a mocked version of execution state that
// simulates some of its behavior for testing purpose
type mockExecutionState struct {
	sync.Mutex
	stateMock.ExecutionState
	commits map[flow.Identifier]flow.StateCommitment
}

func newMockExecutionState(seal *flow.Seal) *mockExecutionState {
	commits := make(map[flow.Identifier]flow.StateCommitment)
	commits[seal.BlockID] = seal.FinalState
	return &mockExecutionState{
		commits: commits,
	}
}

func (es *mockExecutionState) StateCommitmentByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
) (
	flow.StateCommitment,
	error,
) {
	es.Lock()
	defer es.Unlock()
	commit, ok := es.commits[blockID]
	if !ok {
		return flow.DummyStateCommitment, storageerr.ErrNotFound
	}

	return commit, nil
}

func (es *mockExecutionState) ExecuteBlock(t *testing.T, block *flow.Block) {
	parentExecuted, err := state.IsBlockExecuted(
		context.Background(),
		es,
		block.Header.ParentID)
	require.NoError(t, err)
	require.True(t, parentExecuted, "parent block not executed")

	es.Lock()
	defer es.Unlock()
	es.commits[block.ID()] = unittest.StateCommitmentFixture()
}

type testingContext struct {
	t                   *testing.T
	engine              *Engine
	headers             *storage.MockHeaders
	blocks              *storage.MockBlocks
	collections         *mockCollectionStore
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
	fetcher             *mockFetcher

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
	collections := newMockCollectionStore()

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

	fetcher := newMockFetcher()
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

	// copy executable block to set `Executing` state for arguments matching
	// without changing original object
	eb := *executableBlock
	eb.Executing = true
	eb.StartState = &newStateCommitment

	// TODO: return computed block
	ctx.computationManager.
		On("ComputeBlock", mock.Anything, previousExecutionResultID, mock.Anything, mock.Anything).
		Return(computationResult, nil).Once()

	ctx.executionState.On("NewStorageSnapshot", newStateCommitment).Return(nil)

	ctx.executionState.
		On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

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

func (ctx testingContext) mockSnapshotWithBlockID(blockID flow.Identifier, identities flow.IdentityList) {
	cluster := new(protocol.Cluster)
	// filter only collections as cluster members
	cluster.On("Members").Return(identities.Filter(filter.HasRole(flow.RoleCollection)))

	epoch := new(protocol.Epoch)
	epoch.On("ClusterByChainID", mock.Anything).Return(cluster, nil)

	epochQuery := new(protocol.EpochQuery)
	epochQuery.On("Current").Return(epoch)

	snap := new(protocol.Snapshot)
	snap.On("Epochs").Return(epochQuery)
	snap.On("Identity", mock.Anything).Return(identities[0], nil)
	ctx.state.On("AtBlockID", blockID).Return(snap)
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

// TestExecuteOneBlock verifies after collection is received,
// block is executed, uploaded, and broadcasted
func TestExecuteOneBlock(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		col := unittest.CollectionFixture(1)
		gua := col.Guarantee()

		colSigner := unittest.IdentifierFixture()
		// A <- B
		blockA := unittest.BlockHeaderFixture()
		blockB := unittest.ExecutableBlockFixtureWithParent([][]flow.Identifier{{colSigner}}, blockA, unittest.StateCommitmentPointerFixture())
		blockB.Block.Payload.Guarantees[0] = &gua
		blockB.Block.Header.PayloadHash = blockB.Block.Payload.Hash()

		// blockA's start state is its parent's state commitment,
		// and blockA's parent has been executed.
		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockB.Block.Header.ParentID] = *blockB.StartState
		ctx.mockStateCommitsWithMap(commits)

		err := ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		wg := sync.WaitGroup{}
		wg.Add(1) // wait for block B to be executed

		result := ctx.assertSuccessfulBlockComputation(
			commits,
			func(blockID flow.Identifier, commit flow.StateCommitment) {
				wg.Done()
			},
			blockB,
			unittest.IdentifierFixture(),
			true,
			*blockB.StartState,
			nil)

		// configure upload manager with a single uploader
		uploader1 := uploadermock.NewUploader(ctx.t)
		uploader1.On("Upload", result).Return(nil).Once()
		ctx.uploadMgr.AddUploader(uploader1)

		err = ctx.engine.handleCollection(
			unittest.IdentifierFixture(),
			&col,
		)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		require.True(t, ctx.fetcher.isFetched(gua.ID()))

		_, more := <-ctx.engine.Done() // wait for all the blocks to be processed
		require.False(t, more)

		_, ok := commits[blockB.ID()]
		require.True(t, ok)

	})
}

func logBlocks(blocks map[string]*entity.ExecutableBlock) {
	log := unittest.Logger()
	for name, b := range blocks {
		log.Debug().Msgf("creating blocks for testing, block %v's ID:%v", name, b.ID())
	}
}

func TestExecuteBlockInOrder(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		// create blocks with the following relations
		// A <- B
		// A <- C <- D

		blockSealed := unittest.BlockHeaderFixture()

		blocks := make(map[string]*entity.ExecutableBlock)
		blocks["A"] = unittest.ExecutableBlockFixtureWithParent(nil, blockSealed, unittest.StateCommitmentPointerFixture())

		// none of the blocks has any collection, so state is essentially the same
		blocks["B"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["A"].Block.Header, blocks["A"].StartState)
		blocks["C"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["A"].Block.Header, blocks["A"].StartState)
		blocks["D"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["C"].Block.Header, blocks["C"].StartState)

		// log the blocks, so that we can link the block ID in the log with the blocks in tests
		logBlocks(blocks)

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blocks["A"].Block.Header.ParentID] = *blocks["A"].StartState

		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits)

		// once block A is computed, it should trigger B and C being sent to compute,
		// which in turn should trigger D
		blockAExecutionResultID := unittest.IdentifierFixture()
		onPersisted := func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		}
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
			blockAExecutionResultID,
			true,
			*blocks["B"].StartState,
			nil)
		ctx.assertSuccessfulBlockComputation(
			commits,
			onPersisted,
			blocks["C"],
			blockAExecutionResultID,
			true,
			*blocks["C"].StartState,
			nil)
		ctx.assertSuccessfulBlockComputation(
			commits,
			onPersisted,
			blocks["D"],
			unittest.IdentifierFixture(),
			true,
			*blocks["D"].StartState,
			nil)

		wg.Add(1)
		err := ctx.engine.handleBlock(context.Background(), blocks["A"].Block)
		require.NoError(t, err)

		wg.Add(1)
		err = ctx.engine.handleBlock(context.Background(), blocks["B"].Block)
		require.NoError(t, err)

		wg.Add(1)
		err = ctx.engine.handleBlock(context.Background(), blocks["C"].Block)
		require.NoError(t, err)

		wg.Add(1)
		err = ctx.engine.handleBlock(context.Background(), blocks["D"].Block)
		require.NoError(t, err)

		// wait until all 4 blocks have been executed
		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		_, more := <-ctx.engine.Done() // wait for all the blocks to be processed
		assert.False(t, more)

		var ok bool
		_, ok = commits[blocks["A"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["B"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["C"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["D"].ID()]
		require.True(t, ok)

		// make sure no stopping has been engaged, as it was not set
		require.False(t, ctx.stopControl.IsExecutionStopped())
		require.False(t, ctx.stopControl.GetStopParameters().Set())
	})
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

// func newIngestionEngine(t *testing.T, ps *mocks.ProtocolState, es *mockExecutionState) (*Engine, *storage.MockHeaders) {
// 	log := unittest.Logger()
// 	metrics := metrics.NewNoopCollector()
// 	tracer, err := trace.NewTracer(log, "test", "test", trace.SensitivityCaptureAll)
// 	require.NoError(t, err)
// 	ctrl := gomock.NewController(t)
// 	net := mocknetwork.NewMockEngineRegistry(ctrl)
// 	var engine *Engine
//
// 	// generates signing identity including staking key for signing
// 	seed := make([]byte, crypto.KeyGenSeedMinLen)
// 	n, err := rand.Read(seed)
// 	require.Equal(t, n, crypto.KeyGenSeedMinLen)
// 	require.NoError(t, err)
// 	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
// 	require.NoError(t, err)
// 	myIdentity.StakingPubKey = sk.PublicKey()
// 	me := mocklocal.NewMockLocal(sk, myIdentity.ID(), t)
//
// 	headers := storage.NewMockHeaders(ctrl)
// 	blocks := storage.NewMockBlocks(ctrl)
// 	collections := storage.NewMockCollections(ctrl)
//
// 	computationManager := new(computation.ComputationManager)
// 	providerEngine := new(provider.ProviderEngine)
//
// 	loader := loader.NewLoader(log, ps, headers, es)
//
// 	unit := enginePkg.NewUnit()
// 	engine, err = New(
// 		unit,
// 		log,
// 		net,
// 		me,
// 		fetcher,
// 		headers,
// 		blocks,
// 		collections,
// 		computationManager,
// 		providerEngine,
// 		es,
// 		metrics,
// 		tracer,
// 		false,
// 		nil,
// 		nil,
// 		stop.NewStopControl(
// 			unit,
// 			time.Second,
// 			zerolog.Nop(),
// 			nil,
// 			headers,
// 			nil,
// 			nil,
// 			&flow.Header{Height: 1},
// 			false,
// 			false,
// 		),
// 		loader,
// 	)
//
// 	require.NoError(t, err)
// 	return engine, headers
// }

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

type mockCollectionStore struct {
	byID map[flow.Identifier]*flow.Collection
}

func newMockCollectionStore() *mockCollectionStore {
	return &mockCollectionStore{
		byID: make(map[flow.Identifier]*flow.Collection),
	}
}

func (m *mockCollectionStore) ByID(id flow.Identifier) (*flow.Collection, error) {
	c, ok := m.byID[id]
	if !ok {
		return nil, fmt.Errorf("collection %s not found: %w", id, storageerr.ErrNotFound)
	}
	return c, nil
}

func (m *mockCollectionStore) Store(c *flow.Collection) error {
	m.byID[c.ID()] = c
	return nil
}

func (m *mockCollectionStore) StoreLightAndIndexByTransaction(collection *flow.LightCollection) error {
	panic("StoreLightIndexByTransaction not implemented")
}

func (m *mockCollectionStore) StoreLight(collection *flow.LightCollection) error {
	panic("LightIndexByTransaction not implemented")
}

func (m *mockCollectionStore) Remove(id flow.Identifier) error {
	delete(m.byID, id)
	return nil
}

func (m *mockCollectionStore) LightByID(id flow.Identifier) (*flow.LightCollection, error) {
	panic("LightByID not implemented")
}

func (m *mockCollectionStore) LightByTransactionID(id flow.Identifier) (*flow.LightCollection, error) {
	panic("LightByTransactionID not implemented")
}

type collectionHandler interface {
	handleCollection(flow.Identifier, *flow.Collection) error
}

type mockFetcher struct {
	byID map[flow.Identifier]struct{}
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{
		byID: make(map[flow.Identifier]struct{}),
	}
}

func (r *mockFetcher) FetchCollection(blockID flow.Identifier, height uint64, guarantee *flow.CollectionGuarantee) error {
	r.byID[guarantee.ID()] = struct{}{}
	return nil
}

func (r *mockFetcher) Force() {
}

func (r *mockFetcher) isFetched(id flow.Identifier) bool {
	_, ok := r.byID[id]
	return ok
}
