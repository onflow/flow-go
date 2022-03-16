package ingestion

import (
	"context"
	"crypto/rand"
	mathRand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	engineCommon "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	computation "github.com/onflow/flow-go/engine/execution/computation/mock"
	provider "github.com/onflow/flow-go/engine/execution/provider/mock"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	state "github.com/onflow/flow-go/engine/execution/state/mock"
	executionUnittest "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/engine/testutil/mocklocal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mocks"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	stateProtocol "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storageerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mocks"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
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
	blocks              *storage.MockBlocks
	collections         *storage.MockCollections
	state               *protocol.State
	conduit             *mocknetwork.Conduit
	collectionConduit   *mocknetwork.Conduit
	computationManager  *computation.ComputationManager
	providerEngine      *provider.ProviderEngine
	executionState      *state.ExecutionState
	snapshot            *protocol.Snapshot
	identity            *flow.Identity
	broadcastedReceipts map[flow.Identifier]*flow.ExecutionReceipt
	collectionRequester *module.MockRequester
}

func runWithEngine(t *testing.T, f func(testingContext)) {

	ctrl := gomock.NewController(t)

	net := mocknetwork.NewMockNetwork(ctrl)
	request := module.NewMockRequester(ctrl)

	// initialize the mocks and engine
	conduit := &mocknetwork.Conduit{}
	collectionConduit := &mocknetwork.Conduit{}
	syncConduit := &mocknetwork.Conduit{}

	// generates signing identity including staking key for signing
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	myIdentity.StakingPubKey = sk.PublicKey()
	me := mocklocal.NewMockLocal(sk, myIdentity.ID(), t)

	blocks := storage.NewMockBlocks(ctrl)
	payloads := storage.NewMockPayloads(ctrl)
	collections := storage.NewMockCollections(ctrl)
	events := storage.NewMockEvents(ctrl)
	serviceEvents := storage.NewMockServiceEvents(ctrl)
	txResults := storage.NewMockTransactionResults(ctrl)

	computationManager := new(computation.ComputationManager)
	providerEngine := new(provider.ProviderEngine)
	protocolState := new(protocol.State)
	executionState := new(state.ExecutionState)
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

	identityList := flow.IdentityList{myIdentity, collection1Identity, collection2Identity, collection3Identity}

	executionState.On("DiskSize").Return(int64(1024*1024), nil).Maybe()

	snapshot.On("Identities", mock.Anything).Return(func(selector flow.IdentityFilter) flow.IdentityList {
		return identityList.Filter(selector)
	}, nil)

	snapshot.On("Identity", mock.Anything).Return(func(nodeID flow.Identifier) *flow.Identity {
		identity, ok := identityList.ByNodeID(nodeID)
		require.Truef(t, ok, "Could not find nodeID %v in identityList", nodeID)
		return identity
	}, nil)

	txResults.EXPECT().BatchStore(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	payloads.EXPECT().Store(gomock.Any(), gomock.Any()).AnyTimes()

	log := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	tracer, err := trace.NewTracer(log, "test", "test", trace.SensitivityCaptureAll)
	require.NoError(t, err)

	request.EXPECT().Force().Return().AnyTimes()

	net.EXPECT().Register(gomock.Eq(engineCommon.SyncExecution), gomock.AssignableToTypeOf(engine)).Return(syncConduit, nil)

	deltas, err := NewDeltas(1000)
	require.NoError(t, err)

	checkAuthorizedAtBlock := func(blockID flow.Identifier) (bool, error) {
		return stateProtocol.IsNodeAuthorizedAt(protocolState.AtBlockID(blockID), myIdentity.NodeID)
	}

	engine, err = New(
		log,
		net,
		me,
		request,
		protocolState,
		blocks,
		collections,
		events,
		serviceEvents,
		txResults,
		computationManager,
		providerEngine,
		executionState,
		metrics,
		tracer,
		false,
		filter.Any,
		deltas,
		10,
		false,
		checkAuthorizedAtBlock,
		false,
	)
	require.NoError(t, err)

	f(testingContext{
		t:                   t,
		engine:              engine,
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
	computationResult *execution.ComputationResult) *protocol.Snapshot {

	if computationResult == nil {
		computationResult = executionUnittest.ComputationResultForBlockFixture(executableBlock)
	}

	if len(computationResult.StateCommitments) > 0 {
		computationResult.StateCommitments[len(computationResult.StateCommitments)-1] = newStateCommitment
	}

	// copy executable block to set `Executing` state for arguments matching
	// without changing original object
	eb := *executableBlock
	eb.Executing = true
	eb.StartState = &newStateCommitment

	ctx.computationManager.
		On("ComputeBlock", mock.Anything, &eb, mock.Anything).Run(func(args mock.Arguments) {
	}).
		Return(computationResult, nil).Once()

	ctx.executionState.On("NewView", newStateCommitment).Return(new(delta.View))

	ctx.executionState.
		On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

	mocked := ctx.executionState.
		On("SaveExecutionResults",
			mock.Anything,
			executableBlock.Block.Header,
			newStateCommitment,
			mock.MatchedBy(func(fs []*flow.ChunkDataPack) bool {
				for _, f := range fs {
					if f.StartState != *executableBlock.StartState {
						return false
					}
				}
				return true
			}),
			mock.MatchedBy(func(executionReceipt *flow.ExecutionReceipt) bool {
				return executionReceipt.ExecutionResult.BlockID == executableBlock.Block.ID() &&
					executionReceipt.ExecutionResult.PreviousResultID == previousExecutionResultID
			}),
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).
		Return(nil)

	mocked.RunFn =
		func(args mock.Arguments) {
			//lock.Lock()
			//defer lock.Unlock()

			blockID := args[1].(*flow.Header).ID()
			commit := args[2].(flow.StateCommitment)
			commits[blockID] = commit
			onPersisted(blockID, commit)
		}

	mocked.ReturnArguments = mock.Arguments{nil}

	broadcastMock := ctx.providerEngine.
		On(
			"BroadcastExecutionReceipt",
			mock.Anything,
			mock.MatchedBy(func(er *flow.ExecutionReceipt) bool {
				return er.ExecutionResult.BlockID == executableBlock.Block.ID() &&
					er.ExecutionResult.PreviousResultID == previousExecutionResultID
			}),
		).
		Run(func(args mock.Arguments) {
			receipt := args[1].(*flow.ExecutionReceipt)

			executor, err := ctx.snapshot.Identity(receipt.ExecutorID)
			assert.NoError(ctx.t, err, "could not find executor in protocol state")

			// verify the signature
			id := receipt.ID()
			validSig, err := executor.StakingPubKey.Verify(receipt.ExecutorSignature, id[:], ctx.engine.receiptHasher)
			assert.NoError(ctx.t, err)

			assert.True(ctx.t, validSig, "execution receipt signature invalid")

			spocks := receipt.Spocks

			assert.Len(ctx.t, spocks, len(computationResult.StateSnapshots))

			for i, stateSnapshot := range computationResult.StateSnapshots {

				valid, err := crypto.SPOCKVerifyAgainstData(
					ctx.identity.StakingPubKey,
					spocks[i],
					stateSnapshot.SpockSecret,
					ctx.engine.spockHasher,
				)

				assert.NoError(ctx.t, err)
				assert.True(ctx.t, valid)
			}

			ctx.broadcastedReceipts[receipt.ExecutionResult.BlockID] = receipt
		}).
		Return(nil)

	protocolSnapshot := ctx.mockHasWeightAtBlockID(executableBlock.ID(), expectBroadcast)

	if !expectBroadcast {
		broadcastMock.Maybe()
	}

	return protocolSnapshot
}

func (ctx testingContext) mockHasWeightAtBlockID(blockID flow.Identifier, hasWeight bool) *protocol.Snapshot {
	identity := *ctx.identity
	identity.Weight = 0
	if hasWeight {
		identity.Weight = 100
	}
	snap := new(protocol.Snapshot)
	snap.On("Identity", identity.NodeID).Return(&identity, nil)
	ctx.state.On("AtBlockID", blockID).Return(snap)

	return snap
}

func (ctx *testingContext) stateCommitmentExist(blockID flow.Identifier, commit flow.StateCommitment) {
	ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blockID).Return(commit, nil)
}

func (ctx *testingContext) mockStateCommitsWithMap(commits map[flow.Identifier]flow.StateCommitment) {
	lock := sync.Mutex{}

	{
		mocked := ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, mock.Anything)
		// https://github.com/stretchr/testify/issues/350#issuecomment-570478958
		mocked.RunFn = func(args mock.Arguments) {
			// prevent concurrency issue
			lock.Lock()
			defer lock.Unlock()

			blockID := args[1].(flow.Identifier)
			commit, ok := commits[blockID]
			if ok {
				mocked.ReturnArguments = mock.Arguments{commit, nil}
				return
			}

			mocked.ReturnArguments = mock.Arguments{flow.StateCommitment{}, storageerr.ErrNotFound}
		}
	}
}

func TestChunkIndexIsSet(t *testing.T) {

	i := mathRand.Int()
	chunk := execution.GenerateChunk(i,
		unittest.StateCommitmentFixture(),
		unittest.StateCommitmentFixture(),
		unittest.IdentifierFixture(),
		unittest.IdentifierFixture(),
		21)

	assert.Equal(t, i, int(chunk.Index))
	assert.Equal(t, i, int(chunk.CollectionIndex))
}

func TestChunkNumberOfTxsIsSet(t *testing.T) {

	i := mathRand.Uint64()
	chunk := execution.GenerateChunk(3,
		unittest.StateCommitmentFixture(),
		unittest.StateCommitmentFixture(),
		unittest.IdentifierFixture(),
		unittest.IdentifierFixture(),
		i)

	assert.Equal(t, i, chunk.NumberOfTransactions)
}

func TestExecuteOneBlock(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {

		// A <- B
		blockA := unittest.BlockHeaderFixture()
		blockB := unittest.ExecutableBlockFixtureWithParent(nil, &blockA)
		blockB.StartState = unittest.StateCommitmentPointerFixture()

		ctx.mockHasWeightAtBlockID(blockB.ID(), true)

		// blockA's start state is its parent's state commitment,
		// and blockA's parent has been executed.
		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockB.Block.Header.ParentID] = *blockB.StartState
		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits)

		ctx.state.On("Sealed").Return(ctx.snapshot)
		ctx.snapshot.On("Head").Return(&blockA, nil)

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		}, blockB, unittest.IdentifierFixture(), true, *blockB.StartState, nil)

		wg.Add(1) // wait for block B to be executed
		err := ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		require.False(t, more)

		_, ok := commits[blockB.ID()]
		require.True(t, ok)

	})
}

func Test_OnlyHeadOfTheQueueIsExecuted(t *testing.T) {
	// only head of the queue should be executing.
	// Restarting node or errors in consensus module could trigger
	// block (or its parent) which already has been executed to be enqueued again
	// as we already have state commitment for it, it will be executed right away.
	// Now if it finishes execution before it parent - situation can occur that we try to
	// dequeue it, but it will fail since only queue heads are checked.
	//
	// Similarly, rebuild of queues can happen block connecting two heads is added - for example
	// block 1 and 3 are handled and both start executing, in the meantime block 2 is added, and it
	// shouldn't make block 3 requeued as child of 2 (which is child of 1) because it's already being executed
	//
	// Should any of this happen the execution will halt.

	runWithEngine(t, func(ctx testingContext) {

		// A <- B <- C <- D

		// root block
		blockA := unittest.BlockHeaderFixture(func(header *flow.Header) {
			header.Height = 920
		})

		// last executed block - it will be re-queued regardless of state commit
		blockB := unittest.ExecutableBlockFixtureWithParent(nil, &blockA)
		blockB.StartState = unittest.StateCommitmentPointerFixture()

		// finalized block - it can be executed in parallel, as blockB has been executed
		// and this should be fixed
		blockC := unittest.ExecutableBlockFixtureWithParent(nil, blockB.Block.Header)
		blockC.StartState = blockB.StartState

		// expected to be executed afterwards
		blockD := unittest.ExecutableBlockFixtureWithParent(nil, blockC.Block.Header)
		blockD.StartState = blockC.StartState

		logBlocks(map[string]*entity.ExecutableBlock{
			"B": blockB,
			"C": blockC,
			"D": blockD,
		})

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockB.Block.Header.ParentID] = *blockB.StartState
		commits[blockC.Block.Header.ParentID] = *blockC.StartState
		//ctx.mockStateCommitsWithMap(commits)

		wg := sync.WaitGroup{}

		// this intentionally faulty behaviour (block cannot have no state commitment and later have one without being executed)
		// is to hack the first check for block execution and intentionally cause situation where
		// next check (executing only queue head) can be tested
		bFirstTime := true
		bStateCommitment := ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blockB.ID())
		bStateCommitment.RunFn = func(args mock.Arguments) {
			if bFirstTime {
				bStateCommitment.ReturnArguments = mock.Arguments{flow.StateCommitment{}, storageerr.ErrNotFound}
				bFirstTime = false
				return
			}
			bStateCommitment.ReturnArguments = mock.Arguments{*blockB.StartState, nil}
		}

		ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blockA.ID()).Return(*blockB.StartState, nil)
		ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, mock.Anything).Return(nil, storageerr.ErrNotFound)

		ctx.state.On("Sealed").Return(ctx.snapshot)
		ctx.snapshot.On("Head").Return(&blockA, nil)

		wgB := sync.WaitGroup{}
		wgB.Add(1)

		bDone := false
		cDone := false

		// expect B and C to be loaded by loading unexecuted blocks in engine Ready
		wg.Add(2)

		blockBSnapshot := ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			require.False(t, bDone)
			require.False(t, cDone)
			wg.Done()

			// make sure block B execution takes enough time so C can start executing to showcase an error
			time.Sleep(10 * time.Millisecond)

			bDone = true
		}, blockB, unittest.IdentifierFixture(), true, *blockB.StartState, nil)

		blockCSnapshot := ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			require.True(t, bDone)
			require.False(t, cDone)

			wg.Done()
			cDone = true

		}, blockC, unittest.IdentifierFixture(), true, *blockC.StartState, nil)

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			require.True(t, bDone)
			require.True(t, cDone)

			wg.Done()
		}, blockD, unittest.IdentifierFixture(), true, *blockC.StartState, nil)

		// mock loading unexecuted blocks at startup
		ctx.executionState.On("GetHighestExecutedBlockID", mock.Anything).Return(blockB.Height(), blockB.ID(), nil)
		blockASnapshot := new(protocol.Snapshot)

		ctx.state.On("AtHeight", blockB.Height()).Return(blockBSnapshot)
		blockBSnapshot.On("Head").Return(blockB.Block.Header, nil)

		params := new(protocol.Params)
		ctx.state.On("Final").Return(blockCSnapshot)

		// for reloading
		ctx.blocks.EXPECT().ByID(blockB.ID()).Return(blockB.Block, nil)
		ctx.blocks.EXPECT().ByID(blockC.ID()).Return(blockC.Block, nil)

		blockASnapshot.On("Head").Return(&blockA, nil)
		blockCSnapshot.On("Head").Return(blockC.Block.Header, nil)
		blockCSnapshot.On("ValidDescendants").Return(nil, nil)

		ctx.state.On("AtHeight", blockC.Height()).Return(blockCSnapshot)

		ctx.state.On("Params").Return(params)
		params.On("Root").Return(&blockA, nil)

		<-ctx.engine.Ready()

		wg.Add(1) // for block E to be executed - it should wait for D to finish
		err := ctx.engine.handleBlock(context.Background(), blockD.Block)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		require.False(t, more)

		_, ok := commits[blockB.ID()]
		require.True(t, ok)

		_, ok = commits[blockC.ID()]
		require.True(t, ok)

		_, ok = commits[blockD.ID()]
		require.True(t, ok)
	})
}

func TestBlocksArentExecutedMultipleTimes_multipleBlockEnqueue(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "flaky test")

	runWithEngine(t, func(ctx testingContext) {

		colSigner := unittest.IdentifierFixture()

		// A <- B <- C
		blockA := unittest.BlockHeaderFixture()
		blockB := unittest.ExecutableBlockFixtureWithParent(nil, &blockA)
		blockB.StartState = unittest.StateCommitmentPointerFixture()

		//blockCstartState := unittest.StateCommitmentFixture()

		blockC := unittest.ExecutableBlockFixtureWithParent([][]flow.Identifier{{colSigner}}, blockB.Block.Header)
		//blockC.StartState = blockB.StartState //blocks are empty, so no state change is expected

		logBlocks(map[string]*entity.ExecutableBlock{
			"B": blockB,
			"C": blockC,
		})

		collection := blockC.Collections()[0].Collection()

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockB.Block.Header.ParentID] = *blockB.StartState

		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits)

		ctx.state.On("Sealed").Return(ctx.snapshot)
		ctx.snapshot.On("Head").Return(&blockA, nil)

		// wait finishing execution until all the blocks are sent to execution
		wgPut := sync.WaitGroup{}
		wgPut.Add(1)

		// add extra flag to make sure B was indeed executed before C
		wasBExecuted := false

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wgPut.Wait()
			wg.Done()

			wasBExecuted = true
		}, blockB, unittest.IdentifierFixture(), true, *blockB.StartState, nil)

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
			require.True(t, wasBExecuted)
		}, blockC, unittest.IdentifierFixture(), true, *blockB.StartState, nil)

		// make sure collection requests are sent
		// first, the collection should not be found, so the request will be sent. Next, it will be queried again, and this time
		// it should return fine
		gomock.InOrder(
			ctx.collections.EXPECT().ByID(blockC.Collections()[0].Guarantee.CollectionID).DoAndReturn(func(_ flow.Identifier) (*flow.Collection, error) {
				// make sure request for collection from block C are sent before block B finishes execution
				require.False(t, wasBExecuted)
				return nil, storageerr.ErrNotFound
			}),
			ctx.collections.EXPECT().ByID(blockC.Collections()[0].Guarantee.CollectionID).DoAndReturn(func(_ flow.Identifier) (*flow.Collection, error) {
				return &collection, nil
			}),
		)

		ctx.collectionRequester.EXPECT().EntityByID(gomock.Any(), gomock.Any()).DoAndReturn(func(_ flow.Identifier, _ flow.IdentityFilter) {
			// parallel run to avoid deadlock, ingestion engine is thread-safe
			go func() {
				err := ctx.engine.handleCollection(unittest.IdentifierFixture(), &collection)
				require.NoError(t, err)
			}()
		})
		ctx.collections.EXPECT().Store(&collection)

		times := 4

		wg.Add(1) // wait for block B to be executed
		for i := 0; i < times; i++ {
			err := ctx.engine.handleBlock(context.Background(), blockB.Block)
			require.NoError(t, err)
		}
		wg.Add(1) // wait for block C to be executed
		// add extra block to ensure the execution can continue after duplicated blocks
		err := ctx.engine.handleBlock(context.Background(), blockC.Block)
		require.NoError(t, err)
		wgPut.Done()

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		require.False(t, more)

		_, ok := commits[blockB.ID()]
		require.True(t, ok)

		_, ok = commits[blockC.ID()]
		require.True(t, ok)
	})
}

func TestBlocksArentExecutedMultipleTimes_collectionArrival(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {

		// block in the queue are removed only after the execution has finished
		// this gives a brief window for multiple execution
		// when parent block is executing and collection arrives, completing the block
		// it gets executed. When parent finishes it checks it children, finds complete
		// block and executes it again.
		// It should rather not occur during normal execution because StartState won't be set
		// before parent has finished, but we should handle this edge case that it is set as well.

		colSigner := unittest.IdentifierFixture()

		// A <- B <- C <- D
		blockA := unittest.BlockHeaderFixture()
		blockB := unittest.ExecutableBlockFixtureWithParent(nil, &blockA)
		blockB.StartState = unittest.StateCommitmentPointerFixture()

		blockC := unittest.ExecutableBlockFixtureWithParent([][]flow.Identifier{{colSigner}}, blockB.Block.Header)
		blockC.StartState = blockB.StartState //blocks are empty, so no state change is expected

		// block D to make sure execution resumes after block C multiple execution has been prevented
		blockD := unittest.ExecutableBlockFixtureWithParent(nil, blockC.Block.Header)
		blockD.StartState = blockC.StartState

		logBlocks(map[string]*entity.ExecutableBlock{
			"B": blockB,
			"C": blockC,
			"D": blockD,
		})

		collection := blockC.Collections()[0].Collection()

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockB.Block.Header.ParentID] = *blockB.StartState

		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits)

		ctx.state.On("Sealed").Return(ctx.snapshot)
		ctx.snapshot.On("Head").Return(&blockA, nil)

		// wait to control parent (block B) execution until we are ready
		wgB := sync.WaitGroup{}
		wgB.Add(1)

		wgC := sync.WaitGroup{}
		wgC.Add(1)

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wgB.Wait()
			wg.Done()
		}, blockB, unittest.IdentifierFixture(), true, *blockB.StartState, nil)

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wgC.Wait()
			wg.Done()
		}, blockC, unittest.IdentifierFixture(), true, *blockC.StartState, nil)

		ctx.assertSuccessfulBlockComputation(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		}, blockD, unittest.IdentifierFixture(), true, *blockD.StartState, nil)

		// make sure collection requests are sent
		// first, the collection should not be found, so the request will be sent. Next, it will be queried again, and this time
		// it should return fine
		gomock.InOrder(
			ctx.collections.EXPECT().ByID(blockC.Collections()[0].Guarantee.CollectionID).DoAndReturn(func(_ flow.Identifier) (*flow.Collection, error) {
				return nil, storageerr.ErrNotFound

			}),
			ctx.collections.EXPECT().Store(&collection),
			ctx.collections.EXPECT().ByID(blockC.Collections()[0].Guarantee.CollectionID).DoAndReturn(func(_ flow.Identifier) (*flow.Collection, error) {
				return &collection, nil
			}),
		)

		ctx.collectionRequester.EXPECT().EntityByID(gomock.Any(), gomock.Any()).DoAndReturn(func(_ flow.Identifier, _ flow.IdentityFilter) {
			// parallel run to avoid deadlock, ingestion engine is thread-safe
			go func() {
				// OnCollection is official callback for collection requester engine
				ctx.engine.OnCollection(unittest.IdentifierFixture(), &collection)

				// if block C execution started, it will be unblocked, and next execution will cause WaitGroup/mock failure
				// if not, it will be run only once and all will be good
				wgC.Done()
				wgB.Done()
			}()
		}).Times(1)

		wg.Add(1) // wait for block B to be executed
		err := ctx.engine.handleBlock(context.Background(), blockB.Block)
		require.NoError(t, err)

		wg.Add(1) // wait for block C to be executed
		err = ctx.engine.handleBlock(context.Background(), blockC.Block)
		require.NoError(t, err)

		wg.Add(1) // wait for block D to be executed
		err = ctx.engine.handleBlock(context.Background(), blockD.Block)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		require.False(t, more)

		_, ok := commits[blockB.ID()]
		require.True(t, ok)

		_, ok = commits[blockC.ID()]
		require.True(t, ok)

		_, ok = commits[blockD.ID()]
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
		blocks["A"] = unittest.ExecutableBlockFixtureWithParent(nil, &blockSealed)
		blocks["A"].StartState = unittest.StateCommitmentPointerFixture()

		blocks["B"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["A"].Block.Header)
		blocks["C"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["A"].Block.Header)
		blocks["D"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["C"].Block.Header)

		// log the blocks, so that we can link the block ID in the log with the blocks in tests
		logBlocks(blocks)

		// none of the blocks has any collection, so state is essentially the same
		blocks["C"].StartState = blocks["A"].StartState
		blocks["B"].StartState = blocks["A"].StartState
		blocks["D"].StartState = blocks["C"].StartState

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blocks["A"].Block.Header.ParentID] = *blocks["A"].StartState

		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits)

		// make sure the seal height won't trigger state syncing, so that all blocks
		// will be executed.
		ctx.state.On("Sealed").Return(ctx.snapshot)
		// a receipt for sealed block won't be broadcasted
		ctx.snapshot.On("Head").Return(&blockSealed, nil)

		// once block A is computed, it should trigger B and C being sent to compute,
		// which in turn should trigger D
		blockAExecutionResultID := unittest.IdentifierFixture()
		onPersisted := func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		}
		ctx.assertSuccessfulBlockComputation(commits, onPersisted, blocks["A"], unittest.IdentifierFixture(), true, *blocks["A"].StartState, nil)
		ctx.assertSuccessfulBlockComputation(commits, onPersisted, blocks["B"], blockAExecutionResultID, true, *blocks["B"].StartState, nil)
		ctx.assertSuccessfulBlockComputation(commits, onPersisted, blocks["C"], blockAExecutionResultID, true, *blocks["C"].StartState, nil)
		ctx.assertSuccessfulBlockComputation(commits, onPersisted, blocks["D"], unittest.IdentifierFixture(), true, *blocks["D"].StartState, nil)

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

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
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
	})
}

func TestExecutionGenerationResultsAreChained(t *testing.T) {

	execState := new(state.ExecutionState)

	ctrl := gomock.NewController(t)
	me := module.NewMockLocal(ctrl)

	executableBlock := unittest.ExecutableBlockFixture([][]flow.Identifier{{collection1Identity.NodeID}, {collection1Identity.NodeID}})
	startState := unittest.StateCommitmentFixture()
	previousExecutionResultID := unittest.IdentifierFixture()

	// mock execution state conversion and signing of

	me.EXPECT().SignFunc(gomock.Any(), gomock.Any(), gomock.Any())
	me.EXPECT().NodeID()
	me.EXPECT().Sign(gomock.Any(), gomock.Any())

	execState.
		On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

	execState.
		On("SaveExecutionResults", mock.Anything, executableBlock.Block.Header, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	e := Engine{
		execState: execState,
		tracer:    trace.NewNoopTracer(),
		me:        me,
	}

	cr := executionUnittest.ComputationResultFixture(nil)
	cr.ExecutableBlock = executableBlock

	er, err := e.saveExecutionResults(context.Background(), cr, startState)
	assert.NoError(t, err)

	assert.Equal(t, previousExecutionResultID, er.ExecutionResult.PreviousResultID)

	execState.AssertExpectations(t)
}

func TestExecuteScriptAtBlockID(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		// Meaningless script
		script := []byte{1, 1, 2, 3, 5, 8, 11}
		scriptResult := []byte{1}

		// Ensure block we're about to query against is executable
		blockA := unittest.ExecutableBlockFixture(nil)
		blockA.StartState = unittest.StateCommitmentPointerFixture()

		snapshot := new(protocol.Snapshot)
		snapshot.On("Head").Return(blockA.Block.Header, nil)

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockA.ID()] = *blockA.StartState

		ctx.stateCommitmentExist(blockA.ID(), *blockA.StartState)

		ctx.state.On("AtBlockID", blockA.Block.ID()).Return(snapshot)
		view := new(delta.View)
		ctx.executionState.On("NewView", *blockA.StartState).Return(view)

		// Successful call to computation manager
		ctx.computationManager.
			On("ExecuteScript", script, [][]byte(nil), blockA.Block.Header, view).
			Return(scriptResult, nil)

		// Execute our script and expect no error
		res, err := ctx.engine.ExecuteScriptAtBlockID(context.Background(), script, nil, blockA.Block.ID())
		assert.NoError(t, err)
		assert.Equal(t, scriptResult, res)

		// Assert other components were called as expected
		ctx.computationManager.AssertExpectations(t)
		ctx.executionState.AssertExpectations(t)
		ctx.state.AssertExpectations(t)
	})
}

func Test_SPOCKGeneration(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {

		snapshots := []*delta.SpockSnapshot{
			{
				SpockSecret: []byte{1, 2, 3},
			},
			{
				SpockSecret: []byte{3, 2, 1},
			},
			{
				SpockSecret: []byte{},
			},
			{
				SpockSecret: unittest.RandomBytes(100),
			},
		}

		executionReceipt, err := ctx.engine.generateExecutionReceipt(
			context.Background(),
			&flow.ExecutionResult{},
			snapshots,
		)
		require.NoError(t, err)

		for i, snapshot := range snapshots {
			valid, err := crypto.SPOCKVerifyAgainstData(
				ctx.identity.StakingPubKey,
				executionReceipt.Spocks[i],
				snapshot.SpockSecret,
				ctx.engine.spockHasher,
			)

			require.NoError(t, err)
			require.True(t, valid)
		}

	})
}

func TestUnauthorizedNodeDoesNotBroadcastReceipts(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "flaky test")

	runWithEngine(t, func(ctx testingContext) {

		// create blocks with the following relations
		// A <- B <- C <- D
		blockSealed := unittest.BlockHeaderFixture()

		blocks := make(map[string]*entity.ExecutableBlock)
		blocks["A"] = unittest.ExecutableBlockFixtureWithParent(nil, &blockSealed)
		blocks["A"].StartState = unittest.StateCommitmentPointerFixture()

		blocks["B"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["A"].Block.Header)
		blocks["C"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["B"].Block.Header)
		blocks["D"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["C"].Block.Header)

		// log the blocks, so that we can link the block ID in the log with the blocks in tests
		logBlocks(blocks)

		// none of the blocks has any collection, so state is essentially the same
		blocks["B"].StartState = blocks["A"].StartState
		blocks["C"].StartState = blocks["B"].StartState
		blocks["D"].StartState = blocks["C"].StartState

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blocks["A"].Block.Header.ParentID] = *blocks["A"].StartState

		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits)

		onPersisted := func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		}

		// make sure the seal height won't trigger state syncing, so that all blocks
		// will be executed.
		ctx.state.On("Sealed").Return(ctx.snapshot)
		// a receipt for sealed block won't be broadcasted
		ctx.snapshot.On("Head").Return(&blockSealed, nil)

		ctx.mockHasWeightAtBlockID(blocks["A"].ID(), true)

		ctx.assertSuccessfulBlockComputation(commits, onPersisted, blocks["A"], unittest.IdentifierFixture(), true, *blocks["A"].StartState, nil)
		ctx.assertSuccessfulBlockComputation(commits, onPersisted, blocks["B"], unittest.IdentifierFixture(), false, *blocks["B"].StartState, nil)
		ctx.assertSuccessfulBlockComputation(commits, onPersisted, blocks["C"], unittest.IdentifierFixture(), true, *blocks["C"].StartState, nil)
		ctx.assertSuccessfulBlockComputation(commits, onPersisted, blocks["D"], unittest.IdentifierFixture(), false, *blocks["D"].StartState, nil)

		wg.Add(1)
		ctx.mockHasWeightAtBlockID(blocks["A"].ID(), true)

		err := ctx.engine.handleBlock(context.Background(), blocks["A"].Block)
		require.NoError(t, err)

		wg.Add(1)
		ctx.mockHasWeightAtBlockID(blocks["B"].ID(), false)

		err = ctx.engine.handleBlock(context.Background(), blocks["B"].Block)
		require.NoError(t, err)

		wg.Add(1)
		ctx.mockHasWeightAtBlockID(blocks["C"].ID(), true)

		err = ctx.engine.handleBlock(context.Background(), blocks["C"].Block)
		require.NoError(t, err)

		wg.Add(1)
		ctx.mockHasWeightAtBlockID(blocks["D"].ID(), false)

		err = ctx.engine.handleBlock(context.Background(), blocks["D"].Block)
		require.NoError(t, err)

		//// wait until all 4 blocks have been executed
		unittest.AssertReturnsBefore(t, wg.Wait, 15*time.Second)
		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		assert.False(t, more)

		require.Len(t, ctx.broadcastedReceipts, 2)

		var ok bool

		// make sure only selected receipts were broadcasted
		_, ok = ctx.broadcastedReceipts[blocks["A"].ID()]
		require.True(t, ok)
		_, ok = ctx.broadcastedReceipts[blocks["B"].ID()]
		require.False(t, ok)
		_, ok = ctx.broadcastedReceipts[blocks["C"].ID()]
		require.True(t, ok)
		_, ok = ctx.broadcastedReceipts[blocks["D"].ID()]
		require.False(t, ok)

		_, ok = commits[blocks["A"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["B"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["C"].ID()]
		require.True(t, ok)
		_, ok = commits[blocks["D"].ID()]
		require.True(t, ok)
	})
}

// func TestShouldTriggerStateSync(t *testing.T) {
// 	require.True(t, shouldTriggerStateSync(1, 2, 2))
// 	require.False(t, shouldTriggerStateSync(1, 1, 2))
// 	require.True(t, shouldTriggerStateSync(1, 3, 2))
// 	require.True(t, shouldTriggerStateSync(1, 4, 2))
//
// 	// there are only 9 sealed and unexecuted blocks between height 20 and 28,
// 	// haven't reach the threshold 10 yet, so should not trigger
// 	require.False(t, shouldTriggerStateSync(20, 28, 10))
//
// 	// there are 10 sealed and unexecuted blocks between height 20 and 29,
// 	// reached the threshold 10, so should trigger
// 	require.True(t, shouldTriggerStateSync(20, 29, 10))
// }

func newIngestionEngine(t *testing.T, ps *mocks.ProtocolState, es *mocks.ExecutionState) *Engine {
	log := unittest.Logger()
	metrics := metrics.NewNoopCollector()
	tracer, err := trace.NewTracer(log, "test", "test", trace.SensitivityCaptureAll)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	net := mocknetwork.NewMockNetwork(ctrl)
	request := module.NewMockRequester(ctrl)
	syncConduit := &mocknetwork.Conduit{}
	var engine *Engine
	net.EXPECT().Register(gomock.Eq(engineCommon.SyncExecution), gomock.AssignableToTypeOf(engine)).Return(syncConduit, nil)

	// generates signing identity including staking key for signing
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	myIdentity.StakingPubKey = sk.PublicKey()
	me := mocklocal.NewMockLocal(sk, myIdentity.ID(), t)

	blocks := storage.NewMockBlocks(ctrl)
	collections := storage.NewMockCollections(ctrl)
	events := storage.NewMockEvents(ctrl)
	serviceEvents := storage.NewMockServiceEvents(ctrl)
	txResults := storage.NewMockTransactionResults(ctrl)

	computationManager := new(computation.ComputationManager)
	providerEngine := new(provider.ProviderEngine)

	deltas, err := NewDeltas(10)
	require.NoError(t, err)

	checkAuthorizedAtBlock := func(blockID flow.Identifier) (bool, error) {
		return stateProtocol.IsNodeAuthorizedAt(ps.AtBlockID(blockID), myIdentity.NodeID)
	}

	engine, err = New(
		log,
		net,
		me,
		request,
		ps,
		blocks,
		collections,
		events,
		serviceEvents,
		txResults,
		computationManager,
		providerEngine,
		es,
		metrics,
		tracer,
		false,
		filter.Any,
		deltas,
		10,
		false,
		checkAuthorizedAtBlock,
		false,
	)

	require.NoError(t, err)
	return engine
}

func logChain(chain []*flow.Block) {
	log := unittest.Logger()
	for i, block := range chain {
		log.Info().Msgf("block %v, height: %v, ID: %v", i, block.Header.Height, block.ID())
	}
}

func TestLoadingUnexecutedBlocks(t *testing.T) {
	t.Run("only genesis", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(0)
		genesis := chain[0]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))

		es := mocks.NewExecutionState(seal)
		engine := newIngestionEngine(t, ps, es)

		finalized, pending, err := engine.unexecutedBlocks()
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, finalized)
		unittest.IDsEqual(t, []flow.Identifier{}, pending)
	})

	t.Run("no finalized, nor pending unexected", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))

		es := mocks.NewExecutionState(seal)
		engine := newIngestionEngine(t, ps, es)

		finalized, pending, err := engine.unexecutedBlocks()
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, finalized)
		unittest.IDsEqual(t, []flow.Identifier{blockA.ID(), blockB.ID(), blockC.ID(), blockD.ID()}, pending)
	})

	t.Run("no finalized, some pending executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))

		es := mocks.NewExecutionState(seal)
		engine := newIngestionEngine(t, ps, es)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)

		finalized, pending, err := engine.unexecutedBlocks()
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, finalized)
		unittest.IDsEqual(t, []flow.Identifier{blockC.ID(), blockD.ID()}, pending)
	})

	t.Run("all finalized have been executed, and no pending executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))

		require.NoError(t, ps.Finalize(blockC.ID()))

		es := mocks.NewExecutionState(seal)
		engine := newIngestionEngine(t, ps, es)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)
		es.ExecuteBlock(t, blockC)

		finalized, pending, err := engine.unexecutedBlocks()
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, finalized)
		unittest.IDsEqual(t, []flow.Identifier{blockD.ID()}, pending)
	})

	t.Run("some finalized are executed and conflicting are executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))

		require.NoError(t, ps.Finalize(blockC.ID()))

		es := mocks.NewExecutionState(seal)
		engine := newIngestionEngine(t, ps, es)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)
		es.ExecuteBlock(t, blockC)

		finalized, pending, err := engine.unexecutedBlocks()
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, finalized)
		unittest.IDsEqual(t, []flow.Identifier{blockD.ID()}, pending)
	})

	t.Run("all pending executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		chain, result, seal := unittest.ChainFixture(4)
		genesis, blockA, blockB, blockC, blockD :=
			chain[0], chain[1], chain[2], chain[3], chain[4]

		logChain(chain)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockD))
		require.NoError(t, ps.Finalize(blockA.ID()))

		es := mocks.NewExecutionState(seal)
		engine := newIngestionEngine(t, ps, es)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)
		es.ExecuteBlock(t, blockC)
		es.ExecuteBlock(t, blockD)

		finalized, pending, err := engine.unexecutedBlocks()
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, finalized)
		unittest.IDsEqual(t, []flow.Identifier{}, pending)
	})

	t.Run("some fork is executed", func(t *testing.T) {
		ps := mocks.NewProtocolState()

		// Genesis <- A <- B <- C (finalized) <- D <- E <- F
		//                                       ^--- G <- H
		//                      ^-- I
		//						     ^--- J <- K
		chain, result, seal := unittest.ChainFixture(6)
		genesis, blockA, blockB, blockC, blockD, blockE, blockF :=
			chain[0], chain[1], chain[2], chain[3], chain[4], chain[5], chain[6]

		fork1 := unittest.ChainFixtureFrom(2, blockD.Header)
		blockG, blockH := fork1[0], fork1[1]

		fork2 := unittest.ChainFixtureFrom(1, blockC.Header)
		blockI := fork2[0]

		fork3 := unittest.ChainFixtureFrom(2, blockB.Header)
		blockJ, blockK := fork3[0], fork3[1]

		logChain(chain)
		logChain(fork1)
		logChain(fork2)
		logChain(fork3)

		require.NoError(t, ps.Bootstrap(genesis, result, seal))
		require.NoError(t, ps.Extend(blockA))
		require.NoError(t, ps.Extend(blockB))
		require.NoError(t, ps.Extend(blockC))
		require.NoError(t, ps.Extend(blockI))
		require.NoError(t, ps.Extend(blockJ))
		require.NoError(t, ps.Extend(blockK))
		require.NoError(t, ps.Extend(blockD))
		require.NoError(t, ps.Extend(blockE))
		require.NoError(t, ps.Extend(blockF))
		require.NoError(t, ps.Extend(blockG))
		require.NoError(t, ps.Extend(blockH))

		require.NoError(t, ps.Finalize(blockC.ID()))

		es := mocks.NewExecutionState(seal)

		engine := newIngestionEngine(t, ps, es)

		es.ExecuteBlock(t, blockA)
		es.ExecuteBlock(t, blockB)
		es.ExecuteBlock(t, blockC)
		es.ExecuteBlock(t, blockD)
		es.ExecuteBlock(t, blockG)
		es.ExecuteBlock(t, blockJ)

		finalized, pending, err := engine.unexecutedBlocks()
		require.NoError(t, err)

		unittest.IDsEqual(t, []flow.Identifier{}, finalized)
		unittest.IDsEqual(t, []flow.Identifier{
			blockI.ID(), // I is still pending, and unexecuted
			blockE.ID(),
			blockF.ID(),
			// note K is not a pending block, but a conflicting block, even if it's not executed,
			// it won't included
			blockH.ID()},
			pending)
	})
}
