package blocks

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engine2 "github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	executionMock "github.com/dapperlabs/flow-go/engine/execution/execution/mocks"
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mocks"
	network "github.com/dapperlabs/flow-go/network/mocks"
	protocol "github.com/dapperlabs/flow-go/protocol/mocks"
	storage "github.com/dapperlabs/flow-go/storage/mocks"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

var (
	collectionIdentity = unittest.IdentityFixture()
	myIdentity         = unittest.IdentityFixture()
)

func runWithEngine(t *testing.T, f func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, state *protocol.MockState, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine)) {

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
	collections := storage.NewMockCollections(ctrl)
	execution := executionMock.NewMockExecutionEngine(ctrl)
	state := protocol.NewMockState(ctrl)

	snapshot := protocol.NewMockSnapshot(ctrl)

	identityList := flow.IdentityList{myIdentity, collectionIdentity}

	state.EXPECT().Final().Return(snapshot).AnyTimes()
	snapshot.EXPECT().Identities(gomock.Any()).DoAndReturn(func(f flow.IdentityFilter) (flow.IdentityList, error) {
		return identityList.Filter(f), nil
	})

	log := zerolog.Logger{}

	var engine *Engine

	net.EXPECT().Register(gomock.Eq(uint8(engine2.ExecutionBlockIngestion)), gomock.AssignableToTypeOf(engine)).Return(conduit, nil)
	net.EXPECT().Register(gomock.Eq(uint8(engine2.CollectionProvider)), gomock.AssignableToTypeOf(engine)).Return(collectionConduit, nil)

	engine, err := New(log, net, me, blocks, collections, state, execution)
	require.NoError(t, err)

	f(t, engine, blocks, collections, state, conduit, collectionConduit, execution)
}

// TODO Currently those tests check if objects are stored directly
// actually validating data is a part of further tasks and likely those
// tests will have to change to reflect this
func TestBlockStorage(t *testing.T) {

	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, state *protocol.MockState, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		block := unittest.BlockFixture()

		blocks.EXPECT().Save(gomock.Eq(&block))
		collectionConduit.EXPECT().Submit(gomock.Any(), gomock.Any()).Times(len(block.CollectionGuarantees))

		err := engine.Process(myIdentity.NodeID, block)
		assert.NoError(t, err)
	})
}

func TestCollectionRequests(t *testing.T) {

	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, state *protocol.MockState, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		//collectionIdentity := unittest.IdentityFixture()
		//collectionIdentity.Role = flow.RoleCollection

		block := unittest.BlockFixture()
		//To make sure we always have collection if the block fixture changes
		block.CollectionGuarantees = unittest.CollectionGuaranteesFixture(5)

		blocks.EXPECT().Save(gomock.Eq(&block))
		for _, col := range block.CollectionGuarantees {
			collectionConduit.EXPECT().Submit(gomock.Eq(provider.CollectionRequest{Fingerprint: col.Fingerprint()}), gomock.Eq(collectionIdentity.NodeID))
		}

		err := engine.ProcessLocal(block)
		require.NoError(t, err)
	})
}

func TestValidatingCollectionResponse(t *testing.T) {

	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, state *protocol.MockState, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		block := unittest.BlockFixture()
		block.CollectionGuarantees = unittest.CollectionGuaranteesFixture(1)

		blocks.EXPECT().Save(gomock.Eq(&block))

		fingerprint := block.CollectionGuarantees[0].Fingerprint()

		collectionConduit.EXPECT().Submit(gomock.Eq(provider.CollectionRequest{Fingerprint: fingerprint}), gomock.Eq(collectionIdentity.NodeID)).Return(nil)

		err := engine.ProcessLocal(block)
		require.NoError(t, err)

		tx := unittest.TransactionBodyFixture()

		rightResponse := provider.CollectionResponse{
			Fingerprint:  fingerprint,
			Transactions: []flow.TransactionBody{tx},
		}

		// TODO Enable wrong response sending once we have a way to hash collection

		// wrongResponse := provider.CollectionResponse{
		//	Fingerprint:  fingerprint,
		//	Transactions: []flow.TransactionBody{tx},
		// }

		// engine.Submit(collectionIdentity.NodeID, wrongResponse)

		// no interaction with conduit for finished block
		// </TODO enable>

		execution.EXPECT().ExecuteBlock(gomock.Any())

		err = engine.ProcessLocal(rightResponse)
		require.NoError(t, err)
	})
}

func TestForwardingToExecution(t *testing.T) {

	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, state *protocol.MockState, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		block := unittest.BlockFixture()
		block.CollectionGuarantees = unittest.CollectionGuaranteesFixture(3)

		blocks.EXPECT().Save(gomock.Eq(&block))

		for _, col := range block.CollectionGuarantees {
			collectionConduit.EXPECT().Submit(gomock.Eq(provider.CollectionRequest{Fingerprint: col.Fingerprint()}), gomock.Eq(collectionIdentity.NodeID))
		}

		err := engine.ProcessLocal(block)
		require.NoError(t, err)

		tx := unittest.TransactionBodyFixture()

		execution.EXPECT().ExecuteBlock(gomock.Any())

		for _, col := range block.CollectionGuarantees {
			rightResponse := provider.CollectionResponse{
				Fingerprint:  col.Fingerprint(),
				Transactions: []flow.TransactionBody{tx},
			}

			err := engine.ProcessLocal(rightResponse)
			require.NoError(t, err)
		}
	})
}
