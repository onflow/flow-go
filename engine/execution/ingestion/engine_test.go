package ingestion

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	engineCommon "github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution"
	computation "github.com/dapperlabs/flow-go/engine/execution/computation/mock"
	state "github.com/dapperlabs/flow-go/engine/execution/state/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mocks"
	network "github.com/dapperlabs/flow-go/network/mocks"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	realStorage "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

var (
	collectionIdentity = unittest.IdentityFixture()
	myIdentity         = unittest.IdentityFixture()
)

type testingContext struct {
	t                 *testing.T
	engine            *Engine
	blocks            *storage.MockBlocks
	collections       *storage.MockCollections
	state             *protocol.State
	conduit           *network.MockConduit
	collectionConduit *network.MockConduit
	executionEngine   *computation.ComputationEngine
	executionState    *state.ExecutionState
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
	computationEngine := new(computation.ComputationEngine)
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

	engine, err := New(log, net, me, protocolState, blocks, payloads, collections, computationEngine, executionState)
	require.NoError(t, err)

	f(testingContext{
		t:                 t,
		engine:            engine,
		blocks:            blocks,
		collections:       collections,
		state:             protocolState,
		conduit:           conduit,
		collectionConduit: collectionConduit,
		executionEngine:   computationEngine,
		executionState:    executionState,
	})

	computationEngine.AssertExpectations(t)
	protocolState.AssertExpectations(t)
	executionState.AssertExpectations(t)
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

		block, colls := makeRealBlock(1)

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))

		id := block.Guarantees[0].ID()

		ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: id}), gomock.Eq(collectionIdentity.NodeID)).Return(nil)

		err := ctx.engine.ProcessLocal(&block)
		require.NoError(t, err)

		rightResponse := messages.CollectionResponse{
			Collection: colls[0],
		}

		// TODO Enable wrong response sending once we have a way to hash collection

		// wrongResponse := provider.CollectionResponse{
		//	Fingerprint:  fingerprint,
		//	Transactions: []flow.TransactionBody{tx},
		// }

		// engine.Submit(collectionIdentity.NodeID, wrongResponse)

		// no interaction with conduit for finished block
		// </TODO enable>

		ctx.executionState.On("StateCommitmentByBlockID", block.ParentID).Return(unittest.StateCommitmentFixture(), nil)
		ctx.executionState.On("NewView", mock.Anything).Return(nil)
		ctx.executionEngine.On("SubmitLocal", mock.AnythingOfType("*execution.ComputationOrder")).Once()

		err = ctx.engine.ProcessLocal(&rightResponse)
		require.NoError(t, err)
	})
}

func TestForwardingToExecution(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block, colls := makeRealBlock(3)

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))

		for _, col := range block.Guarantees {
			ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: col.ID()}), gomock.Eq(collectionIdentity.NodeID))
		}

		err := ctx.engine.ProcessLocal(&block)
		require.NoError(t, err)

		ctx.executionState.On("StateCommitmentByBlockID", block.ParentID).Return(unittest.StateCommitmentFixture(), nil)
		ctx.executionState.On("NewView", mock.Anything).Return(nil)
		ctx.executionEngine.On("SubmitLocal", mock.AnythingOfType("*execution.ComputationOrder")).Once()

		for _, col := range colls {
			rightResponse := messages.CollectionResponse{
				Collection: col,
			}

			err := ctx.engine.ProcessLocal(&rightResponse)
			require.NoError(t, err)
		}
	})
}

func TestNoBlockExecutedUntilAllCollectionsArePosted(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		block, colls := makeRealBlock(3)

		for _, col := range block.Guarantees {
			ctx.collectionConduit.EXPECT().Submit(gomock.Eq(&messages.CollectionRequest{ID: col.ID()}), gomock.Eq(collectionIdentity.NodeID))
		}

		ctx.blocks.EXPECT().Store(gomock.Eq(&block))

		err := ctx.engine.ProcessLocal(&block)
		require.NoError(t, err)

		// No expected calls to "SubmitLocal", so test should fail if any occurs

		rightResponse := messages.CollectionResponse{
			Collection: colls[1],
		}

		err = ctx.engine.ProcessLocal(&rightResponse)
		require.NoError(t, err)
	})
}

func TestBlockOutOfOrder(t *testing.T) {

	runWithEngine(t, func(ctx testingContext) {

		blockA, _ := makeRealBlock(0)
		blockB, _ := makeRealBlockWithParent(0, blockA.ID())
		blockC, _ := makeRealBlockWithParent(0, blockA.ID())
		blockD, _ := makeRealBlockWithParent(0, blockC.ID())

		/* Artists recreation of the block structure:

		  b
		   \
		    a
		   /
		d-c

		*/

		ctx.blocks.EXPECT().Store(gomock.Eq(&blockA))
		ctx.blocks.EXPECT().Store(gomock.Eq(&blockB))
		ctx.blocks.EXPECT().Store(gomock.Eq(&blockC))
		ctx.blocks.EXPECT().Store(gomock.Eq(&blockD))

		stateCommitmentA := unittest.StateCommitmentFixture()

		// no execution state, so puts to waiting queue
		ctx.executionState.On("StateCommitmentByBlockID", blockB.ParentID).Return(nil, realStorage.ErrNotFound)
		err := ctx.engine.handleBlock(&blockB)
		require.NoError(t, err)

		// no execution state, no connection to other nodes
		ctx.executionState.On("StateCommitmentByBlockID", blockC.ParentID).Return(nil, realStorage.ErrNotFound)
		err = ctx.engine.handleBlock(&blockC)
		require.NoError(t, err)

		// child of c so no need to query execution state

		// we account for every call, so if this call would have happen, test will fail
		// ctx.executionState.On("StateCommitmentByBlockID", blockD.ParentID).Return(nil, realStorage.ErrNotFound)
		err = ctx.engine.handleBlock(&blockD)
		require.NoError(t, err)

		// make sure there were no extra calls at this point in test
		ctx.executionState.AssertExpectations(t)
		ctx.executionEngine.AssertExpectations(t)

		// existing execution state, should trigger processing all other blocks
		ctx.executionState.On("StateCommitmentByBlockID", blockA.ParentID).Return(stateCommitmentA, nil)
		ctx.executionState.On("NewView", stateCommitmentA). Return(nil)
		ctx.executionEngine.On("SubmitLocal", mock.MatchedBy(func(computationOrder *execution.ComputationOrder) bool {
			return computationOrder.Block.Block.ID() == blockA.ID()
		}))

		err = ctx.engine.handleBlock(&blockA)
		require.NoError(t, err)

		// once block A is computed, it should trigger B and C being sent to compute

		ctx.executionState.AssertExpectations(t)


	})
}

func makeRealBlock(collectionsCount int) (flow.Block, []flow.Collection) {
	return makeRealBlockWithParent(collectionsCount, unittest.IdentifierFixture())
}

func makeRealBlockWithParent(collectionsCount int, parentID flow.Identifier) (flow.Block, []flow.Collection) {
	colls := make([]flow.Collection, collectionsCount)
	collsGuarantees := make([]*flow.CollectionGuarantee, collectionsCount)

	for i := range colls {
		tx := unittest.TransactionBodyFixture()
		colls[i].Transactions = []*flow.TransactionBody{&tx}

		collsGuarantees[i] = &flow.CollectionGuarantee{
			CollectionID: colls[i].ID(),
		}
	}

	block := unittest.BlockWithParentFixture(parentID)
	block.Guarantees = collsGuarantees
	return block, colls
}
