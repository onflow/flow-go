package topology_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/utils/unittest"
)

type RandPermTopologyTestSuite struct {
	suite.Suite
}

func TestRandPermTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(RandPermTopologyTestSuite))
}

// TestNodesConnected tests overall node connectedness and connectedness by role by keeping nodes of one role type in
// minority (~2%)
func (r *RandPermTopologyTestSuite) TestNodesConnected() {

	// test vectors for different network sizes
	testVector := []struct {
		total        int
		minorityRole flow.Role
		nodeRole     flow.Role
	}{
		// integration tests - order of 10s
		{
			total:        12,
			minorityRole: flow.RoleCollection,
			nodeRole:     flow.RoleCollection,
		},
		{
			total:        12,
			minorityRole: flow.RoleCollection,
			nodeRole:     flow.RoleConsensus,
		},
		// alpha main net order of 100s
		{
			total:        100,
			minorityRole: flow.RoleCollection,
			nodeRole:     flow.RoleCollection,
		},
		{
			total:        100,
			minorityRole: flow.RoleCollection,
			nodeRole:     flow.RoleConsensus,
		},
		// mature flow order of 1000s
		{
			total:        1000,
			minorityRole: flow.RoleCollection,
			nodeRole:     flow.RoleCollection,
		},
		{
			total:        1000,
			minorityRole: flow.RoleCollection,
			nodeRole:     flow.RoleConsensus,
		},
	}

	for _, v := range testVector {
		r.testTopology(v.total, v.minorityRole, v.nodeRole)
	}
}

func (r *RandPermTopologyTestSuite) testTopology(total int, minorityRole flow.Role, nodeRole flow.Role) {

	distribution := createDistribution(total, minorityRole)

	ids := make(flow.IdentityList, 0)
	for role, count := range distribution {
		roleIDs := unittest.IdentityListFixture(count, unittest.WithRole(role))
		ids = append(ids, roleIDs...)
	}

	n := len(ids)
	var adjencyMap = make(map[flow.Identifier]flow.IdentityList, n)

	for _, id := range ids {
		rpt, err := topology.NewRandPermTopology(id.Role, id.NodeID)
		r.NoError(err)
		top, err := rpt.Subset(ids, uint(n))
		r.NoError(err)
		adjencyMap[id.NodeID] = top
	}

	// check that nodes of the same role form a connected graph
	checkConnectednessByRole(r.T(), adjencyMap, minorityRole)

	// check that nodes form a connected graph
	checkConnectedness(r.T(), adjencyMap)
}

// TestSubsetDeterminism tests that if the id list remains the same, the Topology.Subset call always yields the same
// list of nodes
func (r *RandPermTopologyTestSuite) TestSubsetDeterminism() {
	ids := unittest.IdentityListFixture(100, unittest.WithAllRoles())
	for _, id := range ids {
		rpt, err := topology.NewRandPermTopology(flow.RoleConsensus, id.NodeID)
		r.NoError(err)
		var prev flow.IdentityList
		for i := 0; i < 10; i++ {
			current, err := rpt.Subset(ids, uint(100))
			r.NoError(err)
			if prev != nil {
				assert.EqualValues(r.T(), prev, current)
			}
		}
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

func checkConnectednessByRole(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, role flow.Role) {
	checkGraphConnected(t, adjMap, filter.HasRole(role))
}

func checkConnectedness(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList) {
	checkGraphConnected(t, adjMap, filter.Any)
}

// checkGraphConnected checks if the graph represented by the adjacency matrix is connected.
// It traverses the adjacency map starting from an arbitrary node and checks if all nodes that satisfy the filter
// were visited.
func checkGraphConnected(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, f flow.IdentityFilter) {

	// find the expected node count from the adjacency matrix
	expectedIDs := make(flow.IdentityList, 0)
	for _, v := range adjMap {
		// filter each row to get nodes that satisfy the filter
		ids := v.Filter(f)
		if ids.Count() > 0 {
			expectedIDs = append(expectedIDs, ids.Filter(filter.Not(filter.In(expectedIDs)))...)
		}
	}
	expectedCount := len(expectedIDs)

	var startID flow.Identifier
	// start with an arbitrary node which satisfies the filter
	for _, v := range adjMap {
		ids := v.Filter(f)
		if ids.Count() > 0 {
			startID = ids.Sample(1)[0].NodeID
		}
		break
	}

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
