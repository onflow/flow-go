package topology_test

import (
	"math"
	"os"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/network/gossip/libp2p/test"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/network/mocks"
	"github.com/onflow/flow-go/utils/unittest"
)

type ConnectednessTestSuite struct {
	suite.Suite
	nets []*libp2p.Network
}

func TestRandPermTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectednessTestSuite))
}

func (suite *ConnectednessTestSuite) TestTopologySmallScaleCollectionMinority() {
	suite.testTopology(12, flow.RoleCollection)
}

func (suite *ConnectednessTestSuite) TestTopologyModerateScaleCollectionMinority() {
	suite.testTopology(100, flow.RoleCollection)
}

func (suite *ConnectednessTestSuite) TestTopologyMatureScaleCollectionMinority() {
	suite.T().Skip("skipping this test as it requires injectable subscription manager")
	suite.testTopology(1000, flow.RoleCollection)
}

// testTopology tests overall node connectedness and connectedness by role by keeping nodes of one role type in
// minority (~2%)
func (suite *ConnectednessTestSuite) testTopology(total int, minorityRole flow.Role) {
	distribution := createDistribution(total, minorityRole)
	keys := make([]crypto.PrivateKey, 0)
	ids := make(flow.IdentityList, 0)
	for role, count := range distribution {
		roleIDs, roleKeys := test.GenerateIDs(suite.T(), count, unittest.WithRole(role))
		ids = append(ids, roleIDs...)
		keys = append(keys, roleKeys...)
	}

	adjencyMap := make(map[flow.Identifier]flow.IdentityList, total)

	// mocks state for collector nodes topology
	// considers only a single cluster as higher cluster numbers are tested
	// in collectionTopology_test
	state := topology.CreateMockStateForCollectionNodes(suite.T(), ids.Filter(filter.HasRole(flow.RoleCollection)), 1)

	// creates topology instances for the nodes based on their roles
	tops := test.GenerateTopologies(suite.T(), state, ids)

	// creates topology instances for the nodes based on their roles
	golog.SetAllLoggers(golog.LevelError)
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws := test.GenerateMiddlewares(suite.T(), logger, ids, keys)
	suite.nets = test.GenerateNetworks(suite.T(), logger, ids, mws, 100, tops, false)

	// extracts adjacency matrix of the entire system
	for i, net := range suite.nets {
		// registers all topics of this node on subscription manager
		// so that it later can be extracted from it by the network
		topics := engine.GetTopicsByRole(ids[i].Role)
		for _, topic := range topics {
			_, err := net.Register(topic, &mocks.MockEngine{})
			require.NoError(suite.T(), err)
		}

		subset, err := net.Topology()

		require.NoError(suite.T(), err)
		adjencyMap[ids[i].NodeID] = subset
	}

	// check that nodes of the same role form a connected graph
	checkConnectednessByRole(suite.T(), adjencyMap, ids, minorityRole)

	// check that nodes form a connected graph
	checkConnectedness(suite.T(), adjencyMap, ids)
}

func (suite *ConnectednessTestSuite) TearDownTest() {
	for _, net := range suite.nets {
		unittest.RequireCloseBefore(suite.T(), net.Done(), 3*time.Second, "could not stop the network")
	}
}

// createDistribution creates a count distribution of ~total number of nodes with 2% minority node count
func createDistribution(total int, minority flow.Role) map[flow.Role]int {

	minorityPercentage := 0.02
	count := func(per float64) int {
		nodes := int(math.Ceil(per * float64(total))) // assume atleast one node of the minority role
		return nodes
	}
	minorityCount, majorityCount := count(minorityPercentage), count(1-minorityPercentage)
	roles := flow.Roles()
	totalRoles := len(roles) - 1
	majorityCountPerRole := int(math.Ceil(float64(majorityCount) / float64(totalRoles)))

	countMap := make(map[flow.Role]int, totalRoles) // map of role to the number of nodes for that role
	for _, r := range roles {
		if r == minority {
			countMap[r] = minorityCount
		} else {
			countMap[r] = majorityCountPerRole
		}
	}
	return countMap
}

func checkConnectednessByRole(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, ids flow.IdentityList, role flow.Role) {
	checkGraphConnected(t, adjMap, ids, filter.HasRole(role))
}

func checkConnectedness(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, ids flow.IdentityList) {
	checkGraphConnected(t, adjMap, ids, filter.Any)
}

// checkGraphConnected checks if the graph represented by the adjacency matrix is connected.
// It traverses the adjacency map starting from an arbitrary node and checks if all nodes that satisfy the filter
// were visited.
func checkGraphConnected(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, ids flow.IdentityList, f flow.IdentityFilter) {

	// filter the ids and find the expected node count
	expectedIDs := ids.Filter(f)
	expectedCount := len(expectedIDs)

	// start with an arbitrary node which satisfies the filter
	startID := expectedIDs.Sample(1)[0].NodeID

	visited := make(map[flow.Identifier]bool)
	dfs(startID, adjMap, visited, f)

	// assert that expected number of nodes were visited by DFS
	assert.Equal(t, expectedCount, len(visited))
}

// dfs to check graph connectedness
func dfs(currentID flow.Identifier,
	adjMap map[flow.Identifier]flow.IdentityList,
	visited map[flow.Identifier]bool,
	filter flow.IdentityFilter) {

	if visited[currentID] {
		return
	}

	visited[currentID] = true

	for _, id := range adjMap[currentID].Filter(filter) {
		dfs(id.NodeID, adjMap, visited, filter)
	}
}
