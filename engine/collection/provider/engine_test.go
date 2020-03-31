package provider_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	mocknetwork "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	hub        *stub.Hub
	identities flow.IdentityList

	colNode   mock.CollectionNode // the node we are testing
	conNode   mock.ConsensusNode  // used for checking collection guarantee transmission
	reqNode   mock.GenericNode    // used for request/response flows
	reqEngine *mocknetwork.Engine
	conduit   network.Conduit
}

func (suite *Suite) SetupTest() {
	var err error

	suite.hub = stub.NewNetworkHub()

	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	reqIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	suite.identities = flow.IdentityList{colIdentity, conIdentity, reqIdentity}

	suite.colNode = testutil.CollectionNode(suite.T(), suite.hub, colIdentity, suite.identities)
	suite.conNode = testutil.ConsensusNode(suite.T(), suite.hub, conIdentity, suite.identities)
	suite.reqNode = testutil.GenericNode(suite.T(), suite.hub, reqIdentity, suite.identities)

	suite.reqEngine = new(mocknetwork.Engine)
	suite.conduit, err = suite.reqNode.Net.Register(engine.CollectionProvider, suite.reqEngine)
	suite.Require().Nil(err)
}

func TestProviderEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

// should be able to submit collection guarantees to consensus nodes
func (suite *Suite) TestSubmitCollectionGuarantee() {
	t := suite.T()

	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{suite.colNode.Me.NodeID()}
	guarantee.Signature = unittest.SignatureFixture()

	err := suite.colNode.ProviderEngine.SubmitCollectionGuarantee(guarantee)
	assert.NoError(t, err)

	// flush messages from the collection node
	net, ok := suite.hub.GetNetwork(suite.colNode.Me.NodeID())
	require.True(t, ok)
	net.DeliverAll(false)

	assert.Eventually(t, func() bool {
		has := suite.conNode.Guarantees.Has(guarantee.ID())
		return has
	}, time.Millisecond*15, time.Millisecond)
}

func (suite *Suite) TestCollectionRequest() {
	suite.Run("should return existing collection", func() {
		t := suite.T()

		// save a collection to the store
		coll := unittest.CollectionFixture(3)
		err := suite.colNode.Collections.Store(&coll)
		suite.Assert().Nil(err)

		// expect that the requester will receive the collection
		expectedRes := &messages.CollectionResponse{
			Collection: coll,
		}
		suite.reqEngine.On("Process", suite.colNode.Me.NodeID(), expectedRes).Return(nil).Once()

		// send a request for the collection
		req := messages.CollectionRequest{ID: coll.ID()}
		err = suite.conduit.Submit(&req, suite.colNode.Me.NodeID())
		assert.NoError(t, err)

		// flush the request
		net, ok := suite.hub.GetNetwork(suite.reqNode.Me.NodeID())
		require.True(t, ok)
		net.DeliverAll(true)

		// flush the response
		net, ok = suite.hub.GetNetwork(suite.colNode.Me.NodeID())
		require.True(t, ok)
		net.DeliverAll(true)

		//assert we received the right collection
		suite.reqEngine.AssertExpectations(t)
	})

	suite.Run("should return error for non-existent collection", func() {
		t := suite.T()

		// create request with invalid/nonexistent fingerprint
		req := &messages.CollectionRequest{ID: unittest.IdentifierFixture()}

		// provider should return error
		err := suite.colNode.ProviderEngine.ProcessLocal(req)
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func (suite *Suite) TestTransactionRequest() {
	suite.Run("should return transaction from pool", func() {
		t := suite.T()

		// create a transaction to request
		tx := unittest.TransactionBodyFixture()
		err := suite.colNode.Pool.Add(&tx)
		assert.NoError(t, err)

		// expect that the requester will receive the collection
		expectedRes := &messages.TransactionResponse{
			Transaction: tx,
		}
		suite.reqEngine.On("Process", suite.colNode.Me.NodeID(), expectedRes).Return(nil).Once()

		// send a request for the collection
		req := messages.TransactionRequest{ID: tx.ID()}
		err = suite.conduit.Submit(&req, suite.colNode.Me.NodeID())
		assert.NoError(t, err)

		// flush the request
		net, ok := suite.hub.GetNetwork(suite.reqNode.Me.NodeID())
		require.True(t, ok)
		net.DeliverAll(true)

		// flush the response
		net, ok = suite.hub.GetNetwork(suite.colNode.Me.NodeID())
		require.True(t, ok)
		net.DeliverAll(true)

		//assert we received the right transaction
		suite.reqEngine.AssertExpectations(t)
	})

	suite.Run("should return transaction from DB", func() {
		t := suite.T()

		// create a transaction to request
		tx := unittest.TransactionBodyFixture()
		err := suite.colNode.Transactions.Store(&tx)
		assert.NoError(t, err)

		// expect that the requester will receive the collection
		expectedRes := &messages.TransactionResponse{
			Transaction: tx,
		}
		suite.reqEngine.On("Process", suite.colNode.Me.NodeID(), expectedRes).Return(nil).Once()

		// send a request for the collection
		req := messages.TransactionRequest{ID: tx.ID()}
		err = suite.conduit.Submit(&req, suite.colNode.Me.NodeID())
		assert.NoError(t, err)

		// flush the request
		net, ok := suite.hub.GetNetwork(suite.reqNode.Me.NodeID())
		require.True(t, ok)
		net.DeliverAll(true)

		// flush the response
		net, ok = suite.hub.GetNetwork(suite.colNode.Me.NodeID())
		require.True(t, ok)
		net.DeliverAll(true)

		//assert we received the right transaction
		suite.reqEngine.AssertExpectations(t)
	})

	suite.Run("should return error for non-existent transaction", func() {
		t := suite.T()

		// create request with invalid/nonexistent fingerprint
		req := &messages.TransactionRequest{ID: unittest.IdentifierFixture()}

		// provider should return error
		err := suite.colNode.ProviderEngine.ProcessLocal(req)
		assert.True(t, errors.Is(err, storage.ErrNotFound) || errors.Is(err, mempool.ErrEntityNotFound))
	})
}
