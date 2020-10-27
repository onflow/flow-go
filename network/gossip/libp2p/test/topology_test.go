package test

import (
	"math"
	"os"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/utils/unittest"
)

// TopologyTestSuite tests the bare minimum requirements of a randomized
// topology that is needed for our network. It should not replace the information
// theory assumptions behind the schemes, e.g., random oracle model of hashes
type TopologyTestSuite struct {
	suite.Suite
}

// TestNetworkTestSuit starts all the tests in this test suite
func TestNetworkTestSuit(t *testing.T) {
	suite.Run(t, new(TopologyTestSuite))
}

func (n *TopologyTestSuite) TestTopologySize() {
	totalNodes := 100
	logger := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	// create totalNodes number of networks
	_, _, nets := generateIDsMiddlewaresNetworks(n.T(), totalNodes, logger, 100, nil, true)

	// determine the expected size of the id list that should be returned by RandPermTopology
	rndSubsetSize := int(math.Ceil(float64(totalNodes+1) / 2))
	oneOfEachNodetype := 0 // there is only one node type in this test
	remaining := totalNodes - rndSubsetSize - oneOfEachNodetype
	halfOfRemainingNodes := int(math.Ceil(float64(remaining+1) / 2))
	expectedSize := rndSubsetSize + oneOfEachNodetype + halfOfRemainingNodes

	top, err := nets[0].Topology()
	require.NoError(n.T(), err)
	// assert id list returned is of expected size
	require.Len(n.T(), top, expectedSize)
}

// TestMembership evaluates every id in topology to be a protocol id
func (n *TopologyTestSuite) TestMembership() {
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	ids, _, nets := generateIDsMiddlewaresNetworks(n.T(), 100, logger, 100, nil, true)

	// get the subset from the first network
	subset, err := nets[0].Topology()
	require.NoError(n.T(), err)

	// every id in topology should be an id of the protocol
	require.Empty(n.T(), subset.Filter(filter.Not(filter.In(ids))))
}

// TestDeteministicity verifies that the same seed generates the same topology
func (n *TopologyTestSuite) TestDeteministicity() {
	top, err := topology.NewRandPermTopology(flow.RoleCollection, unittest.IdentifierFixture())
	require.NoError(n.T(), err)

	totalIDs := 100
	ids := unittest.IdentityListFixture(totalIDs)

	// topology of size count/2
	topSize := uint(totalIDs / 2)
	var previous, current []string

	// call the topology.Subset function 100 times and assert that output is always the same
	for i := 0; i < 100; i++ {
		previous = current
		current = nil
		// generate a new topology with the same ids, size and seed
		idMap, err := top.Subset(ids, topSize)
		require.NoError(n.T(), err)

		for _, v := range idMap {
			current = append(current, v.NodeID.String())
		}
		// no guarantees about order is made by Topology.Subset(), hence sort the return values before comparision
		sort.Strings(current)

		if previous == nil {
			continue
		}

		// assert that a different seed generates a different topology
		require.Equal(n.T(), previous, current)
	}
}

// TestUniqueness verifies that different seeds generates different topologies
func (n *TopologyTestSuite) TestUniqueness() {

	totalIDs := 100
	ids := unittest.IdentityListFixture(totalIDs)

	// topology of size count/2
	topSize := uint(totalIDs / 2)
	var previous, current []string

	// call the topology.Subset function 100 times and assert that output is always different
	for i := 0; i < 100; i++ {
		previous = current
		current = nil
		// generate a new topology with a the same ids, size but a different seed for each iteration
		identity, _ := ids.ByIndex(uint(i))
		top, err := topology.NewRandPermTopology(flow.RoleCollection, identity.NodeID)
		require.NoError(n.T(), err)
		idMap, err := top.Subset(ids, topSize)
		require.NoError(n.T(), err)

		for _, v := range idMap {
			current = append(current, v.NodeID.String())
		}
		sort.Strings(current)

		if previous == nil {
			continue
		}

		// assert that a different seed generates a different topology
		require.NotEqual(n.T(), previous, current)
	}
}
