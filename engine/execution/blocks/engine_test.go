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
	storage "github.com/dapperlabs/flow-go/storage/mocks"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func runWithEngine(t *testing.T, f func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine)) {
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

	log := zerolog.Logger{}

	var engine *Engine

	net.EXPECT().Register(gomock.Eq(uint8(engine2.ExecutionBlockIngestion)), gomock.AssignableToTypeOf(engine)).Return(conduit, nil)
	net.EXPECT().Register(gomock.Eq(uint8(engine2.CollectionProvider)), gomock.AssignableToTypeOf(engine)).Return(collectionConduit, nil)

	engine, err := New(log, net, me, blocks, collections, execution)
	require.NoError(t, err)

	f(t, engine, blocks, collections, conduit, collectionConduit, execution)
}

// TODO Currently those tests check if objects are stored directly
// actually validating data is a part of further tasks and likely those
// tests will have to change to reflect this
func TestBlockStorage(t *testing.T) {

	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		identifier := unittest.IdentifierFixture()

		block := unittest.BlockFixture()

		blocks.EXPECT().Save(gomock.Eq(&block))

		err := engine.Process(identifier, block)
		assert.NoError(t, err)
	})

}

func TestCollectionStorage(t *testing.T) {
	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		identifier := unittest.IdentifierFixture()

		collection := unittest.FlowCollectionFixture(1)

		collections.EXPECT().Save(gomock.Eq(&collection))

		err := engine.Process(identifier, collection)
		assert.NoError(t, err)
	})
}

func TestCollectionRequests(t *testing.T) {

	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		collectionIdentity := unittest.IdentityFixture()
		collectionIdentity.Role = flow.RoleCollection

		block := unittest.BlockFixture()
		//To make sure we always have collection if the block fixture changes
		block.GuaranteedCollections = unittest.GuaranteedCollectionsFixture(5)

		blocks.EXPECT().Save(gomock.Eq(&block))

		err := engine.ProcessLocal(block)
		require.NoError(t, err)

		for _, col := range block.GuaranteedCollections {
			collectionConduit.EXPECT().Submit(gomock.Eq(provider.CollectionRequest{Fingerprint: col.Fingerprint()}), gomock.Eq(collectionIdentity))
		}
	})
}

func TestValidatingCollectionResponse(t *testing.T) {

	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		collectionIdentity := unittest.IdentityFixture()
		collectionIdentity.Role = flow.RoleCollection

		block := unittest.BlockFixture()
		block.GuaranteedCollections = unittest.GuaranteedCollectionsFixture(1)

		blocks.EXPECT().Save(gomock.Eq(&block))

		fingerprint := block.GuaranteedCollections[0].Fingerprint()

		err := engine.ProcessLocal(block)
		require.NoError(t, err)

		tx := unittest.TransactionBodyFixture()

		collectionConduit.EXPECT().Submit(gomock.Eq(provider.CollectionRequest{Fingerprint: fingerprint}), gomock.Eq(collectionIdentity)).Return(nil)

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

		engine.Submit(collectionIdentity.NodeID, rightResponse)

		execution.EXPECT().ExecuteBlock(gomock.Any())
	})
}

func TestForwardingToExecution(t *testing.T) {

	runWithEngine(t, func(t *testing.T, engine *Engine, blocks *storage.MockBlocks, collections *storage.MockCollections, conduit *network.MockConduit, collectionConduit *network.MockConduit, execution *executionMock.MockExecutionEngine) {

		collectionIdentity := unittest.IdentityFixture()
		collectionIdentity.Role = flow.RoleCollection

		block := unittest.BlockFixture()
		block.GuaranteedCollections = unittest.GuaranteedCollectionsFixture(3)

		blocks.EXPECT().Save(gomock.Eq(&block))

		fingerprint := block.GuaranteedCollections[0].Fingerprint()

		err := engine.ProcessLocal(block)
		require.NoError(t, err)

		tx := unittest.TransactionBodyFixture()

		collectionConduit.EXPECT().Submit(gomock.Eq(provider.CollectionRequest{Fingerprint: fingerprint}), gomock.Eq(collectionIdentity)).Return(nil)

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

		engine.Submit(collectionIdentity.NodeID, rightResponse)

		execution.EXPECT().ExecuteBlock(gomock.Any())
	})
}
