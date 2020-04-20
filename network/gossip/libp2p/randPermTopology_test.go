package libp2p

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type RandPermTopologyTestSuite struct {
	suite.Suite
}

func TestRandPermTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(RandPermTopologyTestSuite))
}

// TestMinorityNodesConnected tests node configuration of different sizes with one node role in minority
func (r *RandPermTopologyTestSuite) TestMinorityNodesConnected() {

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
		roleIDs := createIDs(role, count)
		ids = append(ids, roleIDs...)
	}

	rpt := NewRandPermTopology(nodeRole)
	fanout := len(ids) / 2
	seed := unittest.IdentityFixture()

	fanoutMap := make(map[flow.Role]int, len(flow.Roles()))

	// test repeated runs of the random topology
	for i := 0; i < 1000; i++ {

		top, err := rpt.Subset(ids, fanout, seed.String())
		r.NoError(err)

		for _, v := range top {
			fanoutMap[v.Role]++
		}

		// assert that nodes of all role are selected
		for _, role := range flow.Roles() {
			r.GreaterOrEqualf(fanoutMap[role], 1, "node with role %s not selected", role)
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

func createIDs(role flow.Role, count int) []*flow.Identity {
	identities := make([]*flow.Identity, 0)
	for i := 0; i < count; i++ {
		identity := &flow.Identity{
			NodeID: unittest.IdentifierFixture(),
			Role:   role,
		}
		identities = append(identities, identity)
	}
	return identities
}
