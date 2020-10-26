package test

import (
	"math"
	"os"
	"testing"

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
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/utils/unittest"
)

type TopologyTestSuite struct {
	suite.Suite
}

func TestRandPermTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(TopologyTestSuite))
}

func (suite *TopologyTestSuite) TestTopologySmallScaleCollectionMinority() {
	suite.testTopology(12, flow.RoleCollection)
}

func (suite *TopologyTestSuite) TestTopologyModerateScaleCollectionMinority() {
	suite.testTopology(100, flow.RoleCollection)
}

func (suite *TopologyTestSuite) TestTopologyMatureScaleCollectionMinority() {
	suite.testTopology(1000, flow.RoleCollection)
}

// testTopology tests overall node connectedness and connectedness by channel ID by keeping nodes of one role type in
// minority (~2%).
// It is also evaluating the validity of identities in the generated topology as well as overall size of topology.
func (suite *TopologyTestSuite) testTopology(total int, minorityRole flow.Role) {
	distribution := createDistribution(total, minorityRole)
	keys := make([]crypto.PrivateKey, 0)
	ids := make(flow.IdentityList, 0)
	for role, count := range distribution {
		roleIDs, roleKeys := GenerateIDs(suite.T(), count, RunNetwork, unittest.WithRole(role))
		ids = append(ids, roleIDs...)
		keys = append(keys, roleKeys...)
	}

	adjencyMap := make(map[flow.Identifier]flow.IdentityList, total)

	// mocks state for collector nodes topology

	state := topology.CreateMockStateForCollectionNodes(suite.T(), ids.Filter(filter.HasRole(flow.RoleCollection)), 1)

	// creates topology instances for the nodes based on their roles
	tops := GenerateTopologies(suite.T(), state, ids)

	// creates topology instances for the nodes based on their roles
	golog.SetAllLoggers(golog.LevelError)
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws := GenerateMiddlewares(suite.T(), logger, ids, keys)

	// mocks subscription manager and creates network in dryrun
	sms := MockSubscriptionManager(suite.T(), ids)
	nets := GenerateNetworks(suite.T(), logger, ids, mws, 100, tops, sms, DryRunNetwork)

	// extracts adjacency matrix of the entire system
	for i, net := range nets {
		top, err := net.Topology()
		require.NoError(suite.T(), err)

		// evaluates size of topology
		checkTopologySize(suite.T(), total, top)

		// evaluates all ids in topology are valid
		checkMembership(suite.T(), top, ids)
		adjencyMap[ids[i].NodeID] = top
	}

	// evaluates subgraph of nodes subscribed to a channelID is connected.
	for _, channelID := range engine.ChannelIDs() {
		checkConnectednessByChannelID(suite.T(), adjencyMap, ids, channelID)

		if engine.IsClusterChannelID(channelID) {
			checkConnectedness()
		}
	}

	// evaluates graph as whole is connected
	// this implies that follower engines on distinct nodes can exchange messages
	// and follow the consensus.
	checkConnectedness(suite.T(), adjencyMap, ids)
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

// checkConnectedness verifies graph as a whole is connected.
func checkConnectedness(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, ids flow.IdentityList) {
	checkGraphConnected(t, adjMap, ids, filter.Any)
}

// checkConnectednessByChannelID verifies that the subgraph of nodes subscribed to a channelID is connected.
func checkConnectednessByChannelID(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, ids flow.IdentityList,
	channelID string) {
	roles, ok := engine.RolesByChannelID(channelID)
	require.True(t, ok)
	checkGraphConnected(t, adjMap, ids, filter.HasRole(roles...))
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

// checkMembership checks each identity in a top list belongs to all identity list
func checkMembership(t *testing.T, top flow.IdentityList, all flow.IdentityList) {
	for _, id := range top {
		require.Contains(t, all, id.NodeID)
	}
}

// TODO: fix this test after we have fanout optimized.
// checkTopologySize evaluates that overall topology size of a node is bound by the fanout of system.
func checkTopologySize(t *testing.T, total int, top flow.IdentityList) {
	t.Skip("this test requires optimizing the fanout per topic")
	fanout := (total + 1) / 2
	require.True(t, len(top) <= fanout)
}

func clusterNum(t *testing.T, ids flow.IdentityList, size int) int {
	collectors := ids.Filter(filter.HasRole(flow.RoleCollection))

	// we need at least two collector nodes to generate a cluster
	// and check the connectedness
	require.True(t, len(collectors) >= 2)
	require.True(t, size > 0)

	clusterNum := len(collectors) / size
	return int(math.Max(float64(clusterNum), 1))
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
