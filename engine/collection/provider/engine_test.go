package provider_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/network"
	mocknetwork "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
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
	chainID   flow.ChainID
}

func (suite *Suite) SetupTest() {
	var err error

	suite.hub = stub.NewNetworkHub()
	suite.chainID = flow.Mainnet

	// add some dummy identities so we have one of each role
	suite.identities = unittest.IdentityListFixture(5, unittest.WithAllRoles())
	colIdentity := suite.identities.Filter(filter.HasRole(flow.RoleCollection))[0]
	conIdentity := suite.identities.Filter(filter.HasRole(flow.RoleConsensus))[0]
	reqIdentity := suite.identities.Filter(filter.HasRole(flow.RoleExecution))[0]

	suite.colNode = testutil.CollectionNode(suite.T(), suite.hub, colIdentity, suite.identities, suite.chainID)
	suite.conNode = testutil.ConsensusNode(suite.T(), suite.hub, conIdentity, suite.identities, suite.chainID)
	suite.reqNode = testutil.GenericNode(suite.T(), suite.hub, reqIdentity, suite.identities, suite.chainID)

	suite.reqEngine = new(mocknetwork.Engine)
	suite.conduit, err = suite.reqNode.Net.Register(engine.PushCollections, suite.reqEngine)
	suite.Require().Nil(err)
}

func (suite *Suite) TearDownTest() {
	suite.reqNode.Done()
	suite.colNode.Done()
	suite.conNode.Done()
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
	genesis, err := suite.conNode.State.Final().Head()
	assert.NoError(t, err)
	guarantee.ReferenceBlockID = genesis.ID()

	err = suite.colNode.ProviderEngine.SubmitCollectionGuarantee(guarantee)
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

		nonce := rand.Uint64()

		// expect that the requester will receive the collection
		expectedRes := &messages.CollectionResponse{
			Collection: coll,
			Nonce:      nonce,
		}
		suite.reqEngine.On("Process", suite.colNode.Me.NodeID(), expectedRes).Return(nil).Once()

		// send a request for the collection
		req := messages.CollectionRequest{ID: coll.ID(), Nonce: nonce}
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
		req := &messages.CollectionRequest{ID: unittest.IdentifierFixture(), Nonce: rand.Uint64()}

		// provider should return error
		err := suite.colNode.ProviderEngine.ProcessLocal(req)
		// we can't distinguish from invalid fingerprint or not found (because we are behind),
		// so there won't be an error
		assert.Nil(t, err)
	})
}
