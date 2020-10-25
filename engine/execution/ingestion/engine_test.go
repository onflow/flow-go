package ingestion

import (
	"bytes"
	"context"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	engineCommon "github.com/onflow/flow-go/engine"
	computation "github.com/onflow/flow-go/engine/execution/computation/mock"
	provider "github.com/onflow/flow-go/engine/execution/provider/mock"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	state "github.com/onflow/flow-go/engine/execution/state/mock"
	executionUnittest "github.com/onflow/flow-go/engine/execution/state/unittest"
	mocklocal "github.com/onflow/flow-go/engine/testutil/mocklocal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mocks"
	"github.com/onflow/flow-go/module/trace"
	network "github.com/onflow/flow-go/network/mocks"
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
	t                  *testing.T
	engine             *Engine
	blocks             *storage.MockBlocks
	collections        *storage.MockCollections
	state              *protocol.State
	conduit            *network.MockConduit
	collectionConduit  *network.MockConduit
	computationManager *computation.ComputationManager
	providerEngine     *provider.ProviderEngine
	executionState     *state.ExecutionState
	snapshot           *protocol.Snapshot
	identity           *flow.Identity
}

func runWithEngine(t *testing.T, f func(testingContext)) {

	ctrl := gomock.NewController(t)

	net := module.NewMockNetwork(ctrl)
	request := module.NewMockRequester(ctrl)

	// initialize the mocks and engine
	conduit := network.NewMockConduit(ctrl)
	collectionConduit := network.NewMockConduit(ctrl)
	syncConduit := network.NewMockConduit(ctrl)

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

	txResults.EXPECT().BatchStore(gomock.Any(), gomock.Any()).AnyTimes()
	payloads.EXPECT().Store(gomock.Any(), gomock.Any()).AnyTimes()

	log := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	tracer, err := trace.NewTracer(log, "test")
	require.NoError(t, err)

	request.EXPECT().Force().Return().AnyTimes()

	net.EXPECT().Register(gomock.Eq(engineCommon.SyncExecution), gomock.AssignableToTypeOf(engine)).Return(syncConduit, nil)

	deltas, err := NewDeltas(1000)
	require.NoError(t, err)

	engine, err = New(
		log,
		net,
		me,
		request,
		protocolState,
		blocks,
		collections,
		events,
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
	)
	require.NoError(t, err)

	f(testingContext{
		t:                  t,
		engine:             engine,
		blocks:             blocks,
		collections:        collections,
		state:              protocolState,
		conduit:            conduit,
		collectionConduit:  collectionConduit,
		computationManager: computationManager,
		providerEngine:     providerEngine,
		executionState:     executionState,
		snapshot:           snapshot,
		identity:           myIdentity,
	})

	<-engine.Done()
}

func (ctx *testingContext) assertSuccessfulBlockComputation(executableBlock *entity.ExecutableBlock, previousExecutionResultID flow.Identifier) {
	computationResult := executionUnittest.ComputationResultForBlockFixture(executableBlock)
	newStateCommitment := unittest.StateCommitmentFixture()
	if len(computationResult.StateSnapshots) == 0 { // if block was empty, no new state commitment is produced
		newStateCommitment = executableBlock.StartState
	}

	ctx.computationManager.
		On("ComputeBlock", mock.Anything, executableBlock, mock.Anything).
		Return(computationResult, nil).Once()

	for _, view := range computationResult.StateSnapshots {
		ctx.executionState.
			On("CommitDelta", mock.Anything, view.Delta, executableBlock.StartState).
			Return(newStateCommitment, nil)

		ctx.executionState.
			On("GetRegistersWithProofs", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil, nil)

		ctx.executionState.
			On("PersistChunkDataPack", mock.Anything, mock.MatchedBy(func(f *flow.ChunkDataPack) bool {
				return bytes.Equal(f.StartState, executableBlock.StartState)
			})).
			Return(nil)
	}

	ctx.executionState.On("NewView", executableBlock.StartState).Return(new(delta.View))

	ctx.executionState.
		On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

	ctx.executionState.
		On("UpdateHighestExecutedBlockIfHigher", mock.Anything, executableBlock.Block.Header).
		Return(nil)

	ctx.executionState.
		On(
			"PersistExecutionReceipt",
			mock.Anything,
			mock.MatchedBy(func(receipt *flow.ExecutionReceipt) bool {
				return receipt.ExecutionResult.BlockID == executableBlock.Block.ID() && receipt.ExecutionResult.PreviousResultID == previousExecutionResultID
			}),
		).
		Return(nil)

	ctx.executionState.On("PersistStateInteractions", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ctx.providerEngine.
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

		}).
		Return(nil)
}

func (ctx *testingContext) stateCommitmentExist(blockID flow.Identifier, commit flow.StateCommitment) {
	ctx.executionState.On("StateCommitmentByBlockID", mock.Anything, blockID).Return(commit, nil)
}

func (ctx *testingContext) mockStateCommitsWithMap(commits map[flow.Identifier]flow.StateCommitment, onPersisted func(flow.Identifier, flow.StateCommitment)) {
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

	{
		mocked := ctx.executionState.
			On("PersistStateCommitment", mock.Anything, mock.Anything, mock.Anything)
		mocked.RunFn =
			func(args mock.Arguments) {
				lock.Lock()
				defer lock.Unlock()

				blockID := args[1].(flow.Identifier)
				commit := args[2].(flow.StateCommitment)
				commits[blockID] = commit
				onPersisted(blockID, commit)
			}

		mocked.ReturnArguments = mock.Arguments{nil}
	}

}

func TestExecuteOneBlock(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		// A <- B
		blockA := unittest.ExecutableBlockFixture(nil)
		blockA.StartState = unittest.StateCommitmentFixture()

		// blockA's start state is its parent's state commitment,
		// and blockA's parent has been executed.
		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockA.Block.Header.ParentID] = blockA.StartState
		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		})

		ctx.state.On("Sealed").Return(ctx.snapshot)
		ctx.snapshot.On("Head").Return(blockA.Block.Header, nil)

		ctx.assertSuccessfulBlockComputation(blockA, unittest.IdentifierFixture())

		wg.Add(1) // wait for block A to be executed
		err := ctx.engine.handleBlock(context.Background(), blockA.Block)
		require.NoError(t, err)

		unittest.AssertReturnsBefore(t, wg.Wait, 5*time.Second)

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		require.False(t, more)

		_, ok := commits[blockA.ID()]
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
		blocks := make(map[string]*entity.ExecutableBlock)
		blocks["A"] = unittest.ExecutableBlockFixture(nil)
		blocks["A"].StartState = unittest.StateCommitmentFixture()

		blocks["B"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["A"].Block.Header)
		blocks["C"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["B"].Block.Header)
		blocks["D"] = unittest.ExecutableBlockFixtureWithParent(nil, blocks["C"].Block.Header)

		// log the blocks, so that we can link the block ID in the log with the blocks in tests
		logBlocks(blocks)

		// none of the blocks has any collection, so state is essentially the same
		blocks["C"].StartState = blocks["A"].StartState
		blocks["B"].StartState = blocks["A"].StartState
		blocks["D"].StartState = blocks["C"].StartState

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blocks["A"].Block.Header.ParentID] = blocks["A"].StartState

		wg := sync.WaitGroup{}
		ctx.mockStateCommitsWithMap(commits, func(blockID flow.Identifier, commit flow.StateCommitment) {
			wg.Done()
		})

		// make sure the seal height won't trigger state syncing, so that all blocks
		// will be executed.
		ctx.state.On("Sealed").Return(ctx.snapshot)
		ctx.snapshot.On("Head").Return(blocks["A"].Block.Header, nil)

		// once block A is computed, it should trigger B and C being sent to compute,
		// which in turn should trigger D
		blockAExecutionResultID := unittest.IdentifierFixture()
		ctx.assertSuccessfulBlockComputation(blocks["A"], unittest.IdentifierFixture())
		ctx.assertSuccessfulBlockComputation(blocks["B"], blockAExecutionResultID)
		ctx.assertSuccessfulBlockComputation(blocks["C"], blockAExecutionResultID)
		ctx.assertSuccessfulBlockComputation(blocks["D"], unittest.IdentifierFixture())

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
		unittest.AssertReturnsBefore(t, wg.Wait, 5*time.Second)

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

	e := Engine{
		execState: execState,
	}

	executableBlock := unittest.ExecutableBlockFixture([][]flow.Identifier{{collection1Identity.NodeID}, {collection1Identity.NodeID}})
	endState := unittest.StateCommitmentFixture()
	previousExecutionResultID := unittest.IdentifierFixture()

	execState.
		On("GetExecutionResultID", mock.Anything, executableBlock.Block.Header.ParentID).
		Return(previousExecutionResultID, nil)

	er, err := e.generateExecutionResultForBlock(context.Background(), executableBlock.Block, nil, endState)
	assert.NoError(t, err)

	assert.Equal(t, previousExecutionResultID, er.PreviousResultID)

	execState.AssertExpectations(t)
}

func TestExecuteScriptAtBlockID(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		// Meaningless script
		script := []byte{1, 1, 2, 3, 5, 8, 11}
		scriptResult := []byte{1}

		// Ensure block we're about to query against is executable
		blockA := unittest.ExecutableBlockFixture(nil)
		blockA.StartState = unittest.StateCommitmentFixture()

		snapshot := new(protocol.Snapshot)
		snapshot.On("Head").Return(blockA.Block.Header, nil)

		commits := make(map[flow.Identifier]flow.StateCommitment)
		commits[blockA.ID()] = blockA.StartState

		ctx.stateCommitmentExist(blockA.ID(), blockA.StartState)

		ctx.state.On("AtBlockID", blockA.Block.ID()).Return(snapshot)
		view := new(delta.View)
		ctx.executionState.On("NewView", blockA.StartState).Return(view)

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

		snapshots := []*delta.Snapshot{
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
			&flow.ExecutionResult{
				ExecutionResultBody: flow.ExecutionResultBody{},
				Signatures:          nil,
			},
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

func TestShouldTriggerStateSync(t *testing.T) {
	require.True(t, shouldTriggerStateSync(1, 2, 2))
	require.False(t, shouldTriggerStateSync(1, 1, 2))
	require.True(t, shouldTriggerStateSync(1, 3, 2))
	require.True(t, shouldTriggerStateSync(1, 4, 2))

	// there are only 9 sealed and unexecuted blocks between height 20 and 28,
	// haven't reach the threshold 10 yet, so should not trigger
	require.False(t, shouldTriggerStateSync(20, 28, 10))

	// there are 10 sealed and unexecuted blocks between height 20 and 29,
	// reached the threshold 10, so should trigger
	require.True(t, shouldTriggerStateSync(20, 29, 10))
}
