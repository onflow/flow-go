package ingestion

import (
	"bytes"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	engineCommon "github.com/dapperlabs/flow-go/engine"
	computation "github.com/dapperlabs/flow-go/engine/execution/computation/mock"
	provider "github.com/dapperlabs/flow-go/engine/execution/provider/mock"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	state "github.com/dapperlabs/flow-go/engine/execution/state/mock"
	executionUnittest "github.com/dapperlabs/flow-go/engine/execution/state/unittest"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	module "github.com/dapperlabs/flow-go/module/mocks"
	network "github.com/dapperlabs/flow-go/network/mocks"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	realStorage "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

var (
	collectionIdentity = unittest.IdentityFixture()
	myIdentity         = unittest.IdentityFixture()
)

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
}

func runWithEngine(t *testing.T, f func(ctx testingContext)) {

	collectionIdentity.Role = flow.RoleCollection
	myIdentity.Role = flow.RoleExecution

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	net := module.NewMockNetwork(ctrl)

	myself := unittest.IdentifierFixture()

	// initialize the mocks and engine
	conduit := network.NewMockConduit(ctrl)
	collectionConduit := network.NewMockConduit(ctrl)
	syncConduit := network.NewMockConduit(ctrl)

	me := module.NewMockLocal(ctrl)
	me.EXPECT().NodeID().Return(myself).AnyTimes()

	blocks := storage.NewMockBlocks(ctrl)
	payloads := storage.NewMockPayloads(ctrl)
	collections := storage.NewMockCollections(ctrl)
	events := storage.NewMockEvents(ctrl)
	computationEngine := new(computation.ComputationManager)
	providerEngine := new(provider.ProviderEngine)
	protocolState := new(protocol.State)
	executionState := new(state.ExecutionState)
	snapshot := new(protocol.Snapshot)

	identityList := flow.IdentityList{myIdentity, collectionIdentity}

	protocolState.On("Final").Return(snapshot).Maybe()
	snapshot.On("Identities", mock.Anything).Return(func(f ...flow.IdentityFilter) flow.IdentityList {
		return identityList.Filter(f[0])
	}, nil)

	payloads.EXPECT().Store(gomock.Any(), gomock.Any()).AnyTimes()

	log := zerolog.Logger{}

	var engine *Engine

	net.EXPECT().Register(gomock.Eq(uint8(engineCommon.BlockProvider)), gomock.AssignableToTypeOf(engine)).Return(conduit, nil)
	net.EXPECT().Register(gomock.Eq(uint8(engineCommon.CollectionProvider)), gomock.AssignableToTypeOf(engine)).Return(collectionConduit, nil)
	net.EXPECT().Register(gomock.Eq(uint8(engineCommon.ExecutionSync)), gomock.AssignableToTypeOf(engine)).Return(syncConduit, nil)

	engine, err := New(log, net, me, protocolState, blocks, payloads, collections, events, computationEngine, providerEngine, executionState, 21)
	require.NoError(t, err)

	f(testingContext{
		t:                  t,
		engine:             engine,
		blocks:             blocks,
		collections:        collections,
		state:              protocolState,
		conduit:            conduit,
		collectionConduit:  collectionConduit,
		computationManager: computationEngine,
		providerEngine:     providerEngine,
		executionState:     executionState,
	})

	computationEngine.AssertExpectations(t)
	protocolState.AssertExpectations(t)
	executionState.AssertExpectations(t)
	providerEngine.AssertExpectations(t)
}

// TODO Currently those tests check if objects are stored directly
// actually validating data is a part of further tasks and likely those
// tests will have to change to reflect this
func TestCollectionRequests(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block := unittest.BlockFixture()
		//To make sure we always have collection if the block fixture changes
		block.Guarantees = unittest.CollectionGuaranteesFixture(5)

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))
		for _, col := range block.Guarantees {
			ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: col.ID()}), gomock.Eq(collectionIdentity.NodeID))
		}
		ctx.executionState.On("StateCommitmentByBlockID", block.ParentID).Return(nil, realStorage.ErrNotFound)

		err := ctx.engine.ProcessLocal(&block)

		require.NoError(t, err)
	})
}

func TestValidatingCollectionResponse(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		executableBlock := unittest.ExecutableBlockFixture(1)

		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlock.Block))

		id := executableBlock.Collections()[0].Guarantee.ID()

		ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: id}), gomock.Eq(collectionIdentity.NodeID)).Return(nil)
		ctx.executionState.On("StateCommitmentByBlockID", executableBlock.Block.ParentID).Return(unittest.StateCommitmentFixture(), realStorage.ErrNotFound)

		err := ctx.engine.ProcessLocal(executableBlock.Block)
		require.NoError(t, err)

		rightResponse := messages.CollectionResponse{
			Collection: flow.Collection{Transactions: executableBlock.Collections()[0].Transactions},
		}

		// TODO Enable wrong response sending once we have a way to hash collection

		// wrongResponse := provider.CollectionResponse{
		//	Fingerprint:  fingerprint,
		//	Transactions: []flow.TransactionBody{tx},
		// }

		// engine.Submit(collectionIdentity.NodeID, wrongResponse)

		// no interaction with conduit for finished executableBlock
		// </TODO enable>

		//ctx.executionState.On("StateCommitmentByBlockID", executableBlock.Block.ParentID).Return(unittest.StateCommitmentFixture(), realStorage.ErrNotFound)

		//ctx.assertSuccessfulBlockComputation(executableBlock.Block)

		err = ctx.engine.ProcessLocal(&rightResponse)
		require.NoError(t, err)
	})
}

func (ctx *testingContext) assertSuccessfulBlockComputation(executableBlock *entity.ExecutableBlock, previousExecutionResultID flow.Identifier) {
	computationResult := executionUnittest.ComputationResultForBlockFixture(executableBlock)
	newStateCommitment := unittest.StateCommitmentFixture()
	if len(computationResult.StateSnapshots) == 0 { //if block was empty, no new state commitment is produced
		newStateCommitment = executableBlock.StartState
	}
	ctx.executionState.On("NewView", executableBlock.StartState).Return(new(delta.View))

	ctx.computationManager.On("ComputeBlock", executableBlock, mock.Anything).Return(computationResult, nil).Once()

	ctx.executionState.On("PersistStateInteractions", executableBlock.Block.ID(), mock.Anything).Return(nil)

	for _, view := range computationResult.StateSnapshots {
		ctx.executionState.On("CommitDelta", view.Delta).Return(newStateCommitment, nil)
		ctx.executionState.On("PersistChunkDataPack", mock.MatchedBy(func(f *flow.ChunkDataPack) bool {
			return bytes.Equal(f.StartState, executableBlock.StartState)
		})).Return(nil)
	}

	ctx.executionState.On("GetExecutionResultID", executableBlock.Block.ParentID).Return(func(blockID flow.Identifier) flow.Identifier {
		return previousExecutionResultID
	}, nil)

	ctx.executionState.On("UpdateHighestExecutedBlockIfHigher", &executableBlock.Block.Header).Return(nil)

	ctx.executionState.On("PersistExecutionResult", executableBlock.Block.ID(), mock.MatchedBy(func(er flow.ExecutionResult) bool {
		return er.BlockID == executableBlock.Block.ID() && er.PreviousResultID == previousExecutionResultID
	})).Return(nil)
	ctx.executionState.On("PersistStateCommitment", executableBlock.Block.ID(), newStateCommitment).Return(nil)
	ctx.providerEngine.On("BroadcastExecutionReceipt", mock.MatchedBy(func(er *flow.ExecutionReceipt) bool {
		return er.ExecutionResult.BlockID == executableBlock.Block.ID() && er.ExecutionResult.PreviousResultID == previousExecutionResultID
	})).Return(nil)
}

func TestNoBlockExecutedUntilAllCollectionsArePosted(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		executableBlock := unittest.ExecutableBlockFixture(3)

		for _, col := range executableBlock.Block.Guarantees {
			ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: col.ID()}), gomock.Eq(collectionIdentity.NodeID))
		}

		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlock.Block))
		ctx.executionState.On("StateCommitmentByBlockID", executableBlock.Block.ParentID).Return(unittest.StateCommitmentFixture(), realStorage.ErrNotFound)

		err := ctx.engine.ProcessLocal(executableBlock.Block)
		require.NoError(t, err)

		// Expected no calls so test should fail if any occurs

		rightResponse := messages.CollectionResponse{
			Collection: flow.Collection{Transactions: executableBlock.Collections()[1].Transactions},
		}

		err = ctx.engine.ProcessLocal(&rightResponse)
		require.NoError(t, err)
	})
}

func TestExecutionGenerationResultsAreChained(t *testing.T) {

	execState := new(state.ExecutionState)

	e := Engine{
		execState: execState,
	}

	executableBlock := unittest.ExecutableBlockFixture(2)
	endState := unittest.StateCommitmentFixture()
	previousExecutionResultID := unittest.IdentifierFixture()

	execState.On("GetExecutionResultID", executableBlock.Block.ParentID).Return(previousExecutionResultID, nil)
	execState.On("PersistExecutionResult", executableBlock.Block.ID(), mock.Anything).Return(nil)

	er, err := e.generateExecutionResultForBlock(executableBlock.Block, nil, endState)
	assert.NoError(t, err)

	assert.Equal(t, previousExecutionResultID, er.PreviousResultID)

	execState.AssertExpectations(t)
}

func TestBlockOutOfOrder(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		executableBlockA := unittest.ExecutableBlockFixture(0)
		executableBlockB := unittest.ExecutableBlockFixtureWithParent(0, &executableBlockA.Block.Header)
		executableBlockC := unittest.ExecutableBlockFixtureWithParent(0, &executableBlockA.Block.Header)
		executableBlockD := unittest.ExecutableBlockFixtureWithParent(0, &executableBlockC.Block.Header)
		executableBlockA.StartState = unittest.StateCommitmentFixture()

		// blocks has no collections, so state is essentially the same
		executableBlockC.StartState = executableBlockA.StartState
		executableBlockB.StartState = executableBlockA.StartState
		executableBlockD.StartState = executableBlockC.StartState

		/* Artists recreation of the blocks structure:

		  b
		   \
		    a
		   /
		d-c

		*/

		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockA.Block))
		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockB.Block))
		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockC.Block))
		ctx.blocks.EXPECT().Store(gomock.Eq(executableBlockD.Block))

		// no execution state, so puts to waiting queue
		ctx.executionState.On("StateCommitmentByBlockID", executableBlockB.Block.ParentID).Return(nil, realStorage.ErrNotFound)
		err := ctx.engine.handleBlock(executableBlockB.Block)
		require.NoError(t, err)

		// no execution state, no connection to other nodes
		ctx.executionState.On("StateCommitmentByBlockID", executableBlockC.Block.ParentID).Return(nil, realStorage.ErrNotFound)
		err = ctx.engine.handleBlock(executableBlockC.Block)
		require.NoError(t, err)

		// child of c so no need to query execution state

		// we account for every call, so if this call would have happen, test will fail
		// ctx.executionState.On("StateCommitmentByBlockID", executableBlockD.Block.ParentID).Return(nil, realStorage.ErrNotFound)
		err = ctx.engine.handleBlock(executableBlockD.Block)
		require.NoError(t, err)

		// make sure there were no extra calls at this point in test
		ctx.executionState.AssertExpectations(t)
		ctx.computationManager.AssertExpectations(t)

		// once block A is computed, it should trigger B and C being sent to compute, which in turn should trigger D
		blockAExecutionResultID := unittest.IdentifierFixture()
		ctx.assertSuccessfulBlockComputation(executableBlockA, unittest.IdentifierFixture())
		ctx.assertSuccessfulBlockComputation(executableBlockB, blockAExecutionResultID)
		ctx.assertSuccessfulBlockComputation(executableBlockC, blockAExecutionResultID)
		ctx.assertSuccessfulBlockComputation(executableBlockD, unittest.IdentifierFixture())

		ctx.executionState.On("StateCommitmentByBlockID", executableBlockA.Block.ParentID).Return(executableBlockA.StartState, nil)
		err = ctx.engine.handleBlock(executableBlockA.Block)
		require.NoError(t, err)

		_, more := <-ctx.engine.Done() //wait for all the blocks to be processed
		assert.False(t, more)
	})

}

func TestExecuteScriptAtBlockID(t *testing.T) {
	runWithEngine(t, func(ctx testingContext) {
		// Meaningless script
		script := []byte{1, 1, 2, 3, 5, 8, 11}
		scriptResult := []byte{1}

		// Ensure block we're about to query against is executable
		executableBlock := unittest.ExecutableBlockFixture(0)
		executableBlock.StartState = unittest.StateCommitmentFixture()

		snapshot := new(protocol.Snapshot)
		snapshot.On("Head").Return(&executableBlock.Block.Header, nil)

		// Add all data needed for execution of script
		ctx.executionState.On("StateCommitmentByBlockID", executableBlock.Block.ID()).Return(executableBlock.StartState, nil)
		ctx.state.On("AtBlockID", executableBlock.Block.ID()).Return(snapshot)
		view := new(delta.View)
		ctx.executionState.On("NewView", executableBlock.StartState).Return(view)

		// Successful call to computation manager
		ctx.computationManager.On("ExecuteScript", script, &executableBlock.Block.Header, view).Return(scriptResult, nil)

		// Execute our script and expect no error
		res, err := ctx.engine.ExecuteScriptAtBlockID(script, executableBlock.Block.ID())
		assert.NoError(t, err)
		assert.Equal(t, scriptResult, res)

		// Assert other components were called as expected
		ctx.computationManager.AssertExpectations(t)
		ctx.executionState.AssertExpectations(t)
		ctx.state.AssertExpectations(t)
	})
}
