package ingestion

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	engineCommon "github.com/dapperlabs/flow-go/engine"
	computation "github.com/dapperlabs/flow-go/engine/execution/computation/mock"
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
	mutator := protocol.NewMockMutator(ctrl)
	snapshot := new(protocol.Snapshot)

	identityList := flow.IdentityList{myIdentity, collectionIdentity}

	protocolState.On("Final").Return(snapshot)
	snapshot.On("Identities", mock.Anything).Return(func(f ...flow.IdentityFilter) flow.IdentityList {
		return identityList.Filter(f[0])
	}, nil)

	mutator := new(protocol.Mutator)
	mutator.On("Mutate").Return(mutator)
	state.EXPECT().Mutate().Return(mutator).AnyTimes()
	mutator.EXPECT().Finalize(gomock.Any()).AnyTimes()
	payloads.EXPECT().Store(gomock.Any()).AnyTimes()

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

func makeRealBlock(n int) (flow.Block, []flow.Collection) {
	colls := make([]flow.Collection, n)
	collsGuarantees := make([]*flow.CollectionGuarantee, n)

	for i := range colls {
		tx := unittest.TransactionBodyFixture()
		colls[i].Transactions = []*flow.TransactionBody{&tx}

		collsGuarantees[i] = &flow.CollectionGuarantee{
			CollectionID: colls[i].ID(),
		}
	}

	block := unittest.BlockFixture()
	block.Guarantees = collsGuarantees
	return block, colls
}
