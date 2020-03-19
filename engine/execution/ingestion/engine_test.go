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
	"github.com/dapperlabs/flow-go/engine/execution"
	computation "github.com/dapperlabs/flow-go/engine/execution/computation/mock"
	provider "github.com/dapperlabs/flow-go/engine/execution/provider/mock"
	state "github.com/dapperlabs/flow-go/engine/execution/state/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mocks"
	network "github.com/dapperlabs/flow-go/network/mocks"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
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
	ComputationManager *computation.ComputationManager
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

	me := module.NewMockLocal(ctrl)
	me.EXPECT().NodeID().Return(myself).AnyTimes()

	blocks := storage.NewMockBlocks(ctrl)
	payloads := storage.NewMockPayloads(ctrl)
	collections := storage.NewMockCollections(ctrl)
	computationEngine := new(computation.ComputationManager)
	providerEngine := new(provider.ProviderEngine)
	protocolState := new(protocol.State)
	executionState := new(state.ExecutionState)
	mutator := new(protocol.Mutator)
	snapshot := new(protocol.Snapshot)

	identityList := flow.IdentityList{myIdentity, collectionIdentity}

	protocolState.On("Final").Return(snapshot)
	snapshot.On("Identities", mock.Anything).Return(func(f ...flow.IdentityFilter) flow.IdentityList {
		return identityList.Filter(f[0])
	}, nil)

	protocolState.On("Mutate").Return(mutator)
	mutator.On("Finalize", mock.Anything).Return(nil)
	payloads.EXPECT().Store(gomock.Any(), gomock.Any()).AnyTimes()

	log := zerolog.Logger{}

	var engine *Engine

	net.EXPECT().Register(gomock.Eq(uint8(engineCommon.BlockProvider)), gomock.AssignableToTypeOf(engine)).Return(conduit, nil)
	net.EXPECT().Register(gomock.Eq(uint8(engineCommon.CollectionProvider)), gomock.AssignableToTypeOf(engine)).Return(collectionConduit, nil)

	engine, err := New(log, net, me, protocolState, blocks, payloads, collections, computationEngine, providerEngine, executionState)
	require.NoError(t, err)

	f(testingContext{
		t:                  t,
		engine:             engine,
		blocks:             blocks,
		collections:        collections,
		state:              protocolState,
		conduit:            conduit,
		collectionConduit:  collectionConduit,
		ComputationManager: computationEngine,
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
func TestBlockStorage(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block := unittest.BlockFixture()

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))
		ctx.collectionConduit.EXPECT().Submit(gomock.Any(), gomock.Any()).Times(len(block.Guarantees))

		err := ctx.engine.ProcessLocal(&block)
		assert.NoError(t, err)
	})
}

func TestCollectionRequests(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block := unittest.BlockFixture()
		//To make sure we always have collection if the block fixture changes
		block.Guarantees = unittest.CollectionGuaranteesFixture(5)

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))
		for _, col := range block.Guarantees {
			ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: col.ID()}), gomock.Eq(collectionIdentity.NodeID))
		}

		err := ctx.engine.ProcessLocal(&block)
		require.NoError(t, err)
	})
}

func TestValidatingCollectionResponse(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		completeBlock := unittest.CompleteBlockFixture(1)

		ctx.blocks.EXPECT().Store(gomock.Eq(completeBlock.Block))

		id := completeBlock.Collections()[0].Guarantee.ID()

		ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: id}), gomock.Eq(collectionIdentity.NodeID)).Return(nil)

		err := ctx.engine.ProcessLocal(completeBlock.Block)
		require.NoError(t, err)

		rightResponse := messages.CollectionResponse{
			Collection: flow.Collection{Transactions: completeBlock.Collections()[0].Transactions},
		}

		// TODO Enable wrong response sending once we have a way to hash collection

		// wrongResponse := provider.CollectionResponse{
		//	Fingerprint:  fingerprint,
		//	Transactions: []flow.TransactionBody{tx},
		// }

		// engine.Submit(collectionIdentity.NodeID, wrongResponse)

		// no interaction with conduit for finished completeBlock
		// </TODO enable>

		ctx.assertSuccessfulBlockComputation(completeBlock)

		err = ctx.engine.ProcessLocal(&rightResponse)
		require.NoError(t, err)
	})
}

func (ctx *testingContext) assertSuccessfulBlockComputation(completeBlock *execution.CompleteBlock) {
	startStateCommitment := unittest.StateCommitmentFixture()
	computationResult := unittest.ComputationResultForBlockFixture(completeBlock)
	newStateCommitment := unittest.StateCommitmentFixture()
	ctx.executionState.On("StateCommitmentByBlockID", completeBlock.Block.ParentID).Return(startStateCommitment, nil)
	ctx.executionState.On("NewView", startStateCommitment).Return(nil)

	ctx.ComputationManager.On("ComputeBlock", completeBlock, mock.Anything).Return(computationResult, nil).Once()

	ctx.executionState.On("CommitDelta", computationResult.StateViews[0].Delta()).Return(newStateCommitment, nil)
	ctx.executionState.On("PersistChunkHeader", mock.MatchedBy(func(f *flow.ChunkHeader) bool {
		return bytes.Equal(f.StartState, startStateCommitment)
	})).Return(nil)
	ctx.executionState.On("PersistChunkDataPack", mock.MatchedBy(func(f *flow.ChunkDataPack) bool {
		return bytes.Equal(f.StartState, startStateCommitment)
	})).Return(nil)
	ctx.executionState.On("GetRegistersWithProofs", mock.Anything, mock.Anything).Return([][]byte{}, [][]byte{}, nil)

	previousExecutionResultID := unittest.IdentifierFixture()
	ctx.executionState.On("GetExecutionResultID", completeBlock.Block.ParentID).Return(previousExecutionResultID, nil)
	ctx.executionState.On("PersistExecutionResult", completeBlock.Block.ID(), mock.MatchedBy(func(er flow.ExecutionResult) bool {
		return er.BlockID == completeBlock.Block.ID() && er.PreviousResultID == previousExecutionResultID
	})).Return(nil)
	ctx.executionState.On("PersistStateCommitment", completeBlock.Block.ID(), newStateCommitment).Return(nil)
	ctx.providerEngine.On("BroadcastExecutionReceipt", mock.MatchedBy(func(er *flow.ExecutionReceipt) bool {
		return er.ExecutionResult.BlockID == completeBlock.Block.ID() && er.ExecutionResult.PreviousResultID == previousExecutionResultID
	})).Return(nil)
}

func TestNoBlockExecutedUntilAllCollectionsArePosted(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block := unittest.CompleteBlockFixture(3)

		for _, col := range block.Block.Guarantees {
			ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: col.ID()}), gomock.Eq(collectionIdentity.NodeID))
		}

		ctx.blocks.EXPECT().Store(gomock.Eq(block.Block))

		err := ctx.engine.ProcessLocal(block.Block)
		require.NoError(t, err)

		// Expected no calls so test should fail if any occurs

		rightResponse := messages.CollectionResponse{
			Collection: flow.Collection{Transactions: block.Collections()[1].Transactions},
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

	completeBlock := unittest.CompleteBlockFixture(2)
	endState := unittest.StateCommitmentFixture()
	previousExecutionResultID := unittest.IdentifierFixture()

	execState.On("GetExecutionResultID", completeBlock.Block.ParentID).Return(previousExecutionResultID, nil)
	execState.On("PersistExecutionResult", completeBlock.Block.ID(), mock.Anything).Return(nil)

	er, err := e.generateExecutionResultForBlock(completeBlock, nil, endState)
	assert.NoError(t, err)

	assert.Equal(t, previousExecutionResultID, er.PreviousResultID)

	execState.AssertExpectations(t)
}
