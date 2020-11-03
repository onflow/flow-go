package topology

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/test"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

type StatefulTopologyTestSuite struct {
	suite.Suite
	state    protocol.State    // represents a mocked protocol state
	ids      flow.IdentityList // represents the identity list of all nodes in the system
	clusters flow.ClusterList  // represents list of cluster ids of collection nodes
	me       flow.Identity     // represents identity of single instance of node that creates topology

}

// TestStatefulTopologyTestSuite runs all tests in this test suite
func TestStatefulTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(StatefulTopologyTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (suite *StatefulTopologyTestSuite) SetupTest() {
	nClusters := 3
	nCollectors := 100
	nTotal := 1000

	collectors, _ := test.GenerateIDs(suite.T(), nCollectors, test.RunNetwork, unittest.WithRole(flow.RoleCollection))
	others, _ := test.GenerateIDs(suite.T(), nTotal, test.RunNetwork,
		unittest.WithAllRolesExcept(flow.RoleCollection))
	suite.ids = append(others, collectors...)

	// mocks state for collector nodes topology
	suite.state, suite.clusters = CreateMockStateForCollectionNodes(suite.T(),
		suite.ids.Filter(filter.HasRole(flow.RoleCollection)), uint(nClusters))

	// takes first id as the current nodes id
	suite.me = *suite.ids[0]
}

func (suite *StatefulTopologyTestSuite) TestRoleSize() {

}
