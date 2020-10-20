package topology_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/network/gossip/libp2p/test"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type CollectionTopologyTestSuite struct {
	suite.Suite
	snapshot    *protocol.Snapshot
	epochQuery  *protocol.EpochQuery
	clusterList flow.ClusterList
	ids         flow.IdentityList
	collectors  flow.IdentityList
	nets        []*libp2p.Network
}

func TestCollectionTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(CollectionTopologyTestSuite))
}

func (suite *CollectionTopologyTestSuite) SetupTest() {
	suite.nets = make([]*libp2p.Network, 0)
	nClusters := 3
	nCollectors := 7

	suite.collectors = unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
	suite.ids = append(unittest.IdentityListFixture(1000, unittest.WithAllRolesExcept(flow.RoleCollection)), suite.collectors...)

	// mocks state for collector nodes topology
	// considers only a 3 clusters
	state := topology.CreateMockStateForCollectionNodes(suite.T(),
		suite.ids.Filter(filter.HasRole(flow.RoleCollection)), uint(nClusters))

	// creates a topology instance for the nodes based on their roles
	tops := test.CreateTopologies(suite.T(), state, suite.ids)

	// creates middleware and network instances
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws := test.CreateMiddleware(suite.T(), logger, suite.ids)
	suite.nets = test.CreateNetworks(suite.T(), logger, suite.ids, mws, tops, 1, true)
}

// TestSubset tests that the collection nodes using CollectionTopology form a connected graph and nodes within the same
// collection clusters also form a connected graph
func (suite *CollectionTopologyTestSuite) TestSubset() {
	var adjencyMap = make(map[flow.Identifier]flow.IdentityList, len(suite.collectors))
	// for each of the collector node, find a subset of nodes it should connect to using the CollectionTopology
	for i, id := range suite.ids {
		if id.Role != flow.RoleCollection {
			continue
		}

		idMap, err := suite.nets[i].Topology()
		require.NoError(suite.T(), err)

		subset := idMapToList(idMap)
		adjencyMap[id.NodeID] = subset
	}

	// check that all collection nodes are either directly connected or indirectly connected via other collection nodes
	checkConnectednessByRole(suite.T(), adjencyMap, suite.collectors, flow.RoleCollection)

	// check that each of the collection clusters forms a connected graph
	for _, cluster := range suite.clusterList {
		checkConnectednessByCluster(suite.T(), adjencyMap, cluster, suite.collectors)
	}
}

func checkConnectednessByCluster(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, cluster flow.IdentityList, ids flow.IdentityList) {
	checkGraphConnected(t, adjMap, ids, filter.In(cluster))
}
