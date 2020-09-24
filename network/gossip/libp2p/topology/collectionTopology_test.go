package topology_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type CollectionTopologyTestSuite struct {
	suite.Suite
	state       *protocol.State
	snapshot    *protocol.Snapshot
	epochQuery  *protocol.EpochQuery
	clusterList flow.ClusterList
	ids         flow.IdentityList
	collectors  flow.IdentityList
}

func TestCollectionTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(CollectionTopologyTestSuite))
}

func (suite *CollectionTopologyTestSuite) SetupTest() {

	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.epochQuery = new(protocol.EpochQuery)

	nClusters := 3
	nCollectors := 7
	suite.collectors = unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
	suite.ids = append(unittest.IdentityListFixture(1000, unittest.WithAllRolesExcept(flow.RoleCollection)), suite.collectors...)
	assignments := unittest.ClusterAssignment(uint(nClusters), suite.collectors)
	clusters, err := flow.NewClusterList(assignments, suite.collectors)
	require.NoError(suite.T(), err)
	suite.clusterList = clusters
	epoch := new(protocol.Epoch)
	epoch.On("Clustering").Return(clusters, nil).Times(nCollectors)
	suite.epochQuery.On("Current").Return(epoch).Times(nCollectors)
	suite.snapshot.On("Epochs").Return(suite.epochQuery).Times(nCollectors)
	suite.state.On("Final").Return(suite.snapshot, nil).Times(nCollectors)
}

func (suite *CollectionTopologyTestSuite) TestSubset() {
	var adjencyMap = make(map[flow.Identifier]flow.IdentityList, len(suite.collectors))
	// for each of the collector node, find a subset of nodes it should connect to using the CollectionTopology
	for _, c := range suite.collectors {
		collectionTopology, err := topology.NewCollectionTopology(c.NodeID, suite.state)
		assert.NoError(suite.T(), err)
		subset, err := collectionTopology.Subset(suite.ids, uint(len(suite.ids)))
		assert.NoError(suite.T(), err)
		adjencyMap[c.NodeID] = subset
	}

	// check that all collection nodes are either directly connected or indirectly connected via other collection nodes
	checkConnectednessByRole(suite.T(), adjencyMap, flow.RoleCollection)

	// check that each of the collection clusters forms a connected graph
	for _, cluster := range suite.clusterList {
		checkConnectednessByCluster(suite.T(), adjencyMap, cluster)
	}
}

func checkConnectednessByCluster(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, cluster flow.IdentityList) {
	checkGraphConnected(t, adjMap, filter.In(cluster))
}
