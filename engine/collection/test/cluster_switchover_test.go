package test

import (
	"testing"

	"github.com/onflow/flow-go/engine/testutil"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
)

type ClusterSwitchoverSuite struct {
	suite.Suite

	collectors   []testmock.CollectionNode
	hub          *stub.Hub
	rootSnapshot protocol.Snapshot
}

func TestClusterSwitchover(t *testing.T) {
	suite.Run(t, new(ClusterSwitchoverSuite))
}

func (suite *ClusterSwitchoverSuite) Root() *flow.Header {
	root, err := suite.rootSnapshot.Head()
	suite.Require().NoError(err)
	return root
}

func (suite *ClusterSwitchoverSuite) ChainID() flow.ChainID {
	return suite.Root().ChainID
}

func (suite *ClusterSwitchoverSuite) ServiceAddress() flow.Address {
	return suite.ChainID().Chain().ServiceAddress()
}

var noopTxScript = `
transaction {
	prepare(acct: AuthAccount) {}
	execute {}
}
`

func (suite *ClusterSwitchoverSuite) TestSingleNode() {
	unittest.LogVerbose()

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	participants := unittest.CompleteIdentitySet(identity)
	suite.rootSnapshot = unittest.RootSnapshotFixture(participants)
	hub := stub.NewNetworkHub()

	node := testutil.CollectionNode(suite.T(), hub, identity, suite.rootSnapshot)

	// for now just bring up and down the node
	<-node.Ready()

	tx := flow.NewTransactionBody().
		SetScript([]byte(noopTxScript)).
		SetPayer(suite.ServiceAddress()).
		SetReferenceBlockID(suite.Root().ID()).
		AddAuthorizer(node.ChainID.Chain().ServiceAddress())
	err := node.IngestionEngine.ProcessLocal(tx)
	suite.Require().NoError(err)

	<-node.Done()
}
