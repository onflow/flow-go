package test

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/testutil"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ClusterSwitchoverSuite is the test suite for testing cluster switchover at
// epoch boundaries. Collection nodes are assigned to one cluster each epoch.
// On epoch boundaries they must gracefully terminate cluster consensus for
// the ending epoch and begin cluster consensus the beginning epoch. These two
// consensus committees co-exist for a short period at the beginning of each
// epoch.
type ClusterSwitchoverSuite struct {
	suite.Suite

	identities flow.IdentityList
	hub        *stub.Hub
	root       protocol.Snapshot
	nodes      []testmock.CollectionNode
}

func TestClusterSwitchoverSuite(t *testing.T) {
	suite.Run(t, new(ClusterSwitchoverSuite))
}

// SetupTest sets up the test with the given identity table, creating mock
// nodes for all collection nodes.
func (suite *ClusterSwitchoverSuite) SetupTest(identities flow.IdentityList) {
	suite.identities = identities
	suite.root = unittest.RootSnapshotFixture(identities)
	suite.hub = stub.NewNetworkHub()

	collectors := identities.Filter(filter.HasRole(flow.RoleCollection))
	for _, collector := range collectors {
		node := testutil.CollectionNode(suite.T(), suite.hub, collector, suite.root)
		suite.nodes = append(suite.nodes, node)
	}
}

func (suite *ClusterSwitchoverSuite) StartNodes() {
	nodes := make([]module.ReadyDoneAware, 0, len(suite.nodes))
	for _, node := range suite.nodes {
		nodes = append(nodes, node)
	}
	unittest.RequireCloseBefore(suite.T(), lifecycle.AllReady(nodes...), time.Second, "could not start nodes")
}

func (suite *ClusterSwitchoverSuite) StopNodes() {
	nodes := make([]module.ReadyDoneAware, 0, len(suite.nodes))
	for _, node := range suite.nodes {
		nodes = append(nodes, node)
	}
	unittest.RequireCloseBefore(suite.T(), lifecycle.AllDone(nodes...), time.Second, "could not stop nodes")
}

func (suite *ClusterSwitchoverSuite) RootBlock() *flow.Header {
	head, err := suite.root.Head()
	require.NoError(suite.T(), err)
	return head
}

func (suite *ClusterSwitchoverSuite) ServiceAddress() flow.Address {
	return suite.RootBlock().ChainID.Chain().ServiceAddress()
}

// Transaction returns a transaction which is valid for ingestion by a
// collection node in this test suite.
func (suite *ClusterSwitchoverSuite) Transaction(opts ...func(*flow.TransactionBody)) *flow.TransactionBody {
	tx := flow.NewTransactionBody().
		AddAuthorizer(suite.ServiceAddress()).
		SetPayer(suite.ServiceAddress()).
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(suite.RootBlock().ID())

	for _, apply := range opts {
		apply(tx)
	}

	return tx
}

// TestSingleNode tests cluster switchover with a single node.
func (suite *ClusterSwitchoverSuite) TestSingleNode() {
	unittest.LogVerbose()

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	participants := unittest.CompleteIdentitySet(identity)
	suite.SetupTest(participants)

	// start the nodes
	suite.StartNodes()
	defer suite.StopNodes()

	collector := suite.nodes[0]

	// build out the first epoch and begin the second epoch - at this point
	// the collection node should be running consensus for both epochs 1 & 2
	builder := unittest.NewEpochBuilder(suite.T(), collector.State)
	builder.BuildEpoch().CompleteEpoch()

	final, err := collector.State.Final().Head()
	require.NoError(suite.T(), err)

	// create one transaction for each epoch
	epoch1Tx := suite.Transaction(func(tx *flow.TransactionBody) {
		tx.SetReferenceBlockID(suite.RootBlock().ID())
	})
	epoch2Tx := suite.Transaction(func(tx *flow.TransactionBody) {
		tx.SetReferenceBlockID(final.ID())
	})

	// submit both transactions to the node's ingestion engine
	err = collector.IngestionEngine.ProcessLocal(epoch1Tx)
	require.NoError(suite.T(), err)
	err = collector.IngestionEngine.ProcessLocal(epoch2Tx)
	require.NoError(suite.T(), err)
}
