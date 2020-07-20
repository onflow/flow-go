package pusher_test

import (
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
	"github.com/dapperlabs/flow-go/network"
	mocknetwork "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	hub        *stub.Hub
	identities flow.IdentityList

	colNode     mock.CollectionNode // the node we are testing
	conNode     mock.ConsensusNode  // used for checking collection guarantee transmission
	reqNode     mock.GenericNode    // used for request/response flows
	reqEngine   *mocknetwork.Engine
	pushConduit network.Conduit
	chainID     flow.ChainID
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
	suite.pushConduit, err = suite.reqNode.Net.Register(engine.PushGuarantees, suite.reqEngine)
	suite.Require().Nil(err)
}

func (suite *Suite) TearDownTest() {
	suite.reqNode.Done()
	suite.colNode.Done()
	suite.conNode.Done()
}

func TestPusherEngine(t *testing.T) {
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

	err = suite.colNode.PusherEngine.SubmitCollectionGuarantee(guarantee)
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
