package test

import (
	"math"
	"os"
	"testing"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

func (suite *TopologyTestSuite) TestTopologySize() {
	totalNodes := 100
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	golog.SetAllLoggers(golog.LevelError)

	// create totalNodes number of networks
	_, _, nets := GenerateIDsMiddlewaresNetworks(suite.T(), totalNodes, logger, 100, nil, DryRunNetwork)

	// determine the expected size of the id list that should be returned by RandPermTopology
	rndSubsetSize := int(math.Ceil(float64(totalNodes+1) / 2))
	oneOfEachNodetype := 0 // there is only one node type in this test
	remaining := totalNodes - rndSubsetSize - oneOfEachNodetype
	halfOfRemainingNodes := int(math.Ceil(float64(remaining+1) / 2))
	expectedSize := rndSubsetSize + oneOfEachNodetype + halfOfRemainingNodes

	top, err := nets[0].Topology()
	require.NoError(suite.T(), err)
	// assert id list returned is of expected size
	require.Len(suite.T(), top, expectedSize)
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
	state, _ := topology.CreateMockStateForCollectionNodes(suite.T(),
		ids.Filter(filter.HasRole(flow.RoleCollection)), 1)

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
		topology.CheckTopologySize(suite.T(), total, top)

		// evaluates all ids in topology are valid
		topology.CheckMembership(suite.T(), top, ids)
		adjencyMap[ids[i].NodeID] = top
	}

	// evaluates subgraph of nodes subscribed to a channelID is connected.
	for _, channelID := range engine.ChannelIDs() {
		topology.CheckConnectednessByChannelID(suite.T(), adjencyMap, ids, channelID)
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
