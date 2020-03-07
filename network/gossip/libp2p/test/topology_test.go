package test

import (
	"os"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// TopologyTestSuite tests the bare minimum requirements of a randomized
// topology that is needed for our network. It should not replace the information
// theory assumptions behind the schemes, e.g., random oracle model of hashes
type TopologyTestSuite struct {
	suite.Suite
	ids   flow.IdentityList // represents the identity list of all nodes in the system
	nets  *libp2p.Network   // represents the single network instance that creates topology
	count int               // indicates size of system
}

// TestNetworkTestSuit starts all the tests in this test suite
func TestNetworkTestSuit(t *testing.T) {
	suite.Run(t, new(TopologyTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (n *TopologyTestSuite) SetupTest() {
	n.count = 100
	n.ids = CreateIDs(n.count)

	// takes firs id as the current nodes id
	me := n.ids[0]

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	key, err := GenerateNetworkingKey(me.NodeID)
	require.NoError(n.Suite.T(), err)

	// creates a middleware instance
	mw, err := libp2p.NewMiddleware(logger, json.NewCodec(), "0.0.0.0:0", me.NodeID, key)
	require.NoError(n.Suite.T(), err)

	// creates and mocks a network instance
	nets, err := CreateNetworks(logger, []*libp2p.Middleware{mw}, n.ids, 1, true)
	require.NoError(n.Suite.T(), err)
	require.Len(n.Suite.T(), nets, 1)
	n.nets = nets[0]
}

func (n *TopologyTestSuite) TestTopologySize() {
	// topology of size the entire network
	topSize := n.count / 2
	top, err := n.nets.Topology()
	require.NoError(n.Suite.T(), err)
	require.Len(n.Suite.T(), top, topSize)
}

// TestMembership evaluates every id in topology to be a protocol id
func (n *TopologyTestSuite) TestMembership() {
	// topology of size 10
	topSize := n.count / 2
	top, err := n.nets.Topology()
	require.NoError(n.Suite.T(), err)
	require.Len(n.Suite.T(), top, topSize)

	// every id in topology should be an id of the protocol
	for id := range top {
		require.Contains(n.Suite.T(), n.ids.NodeIDs(), id)
	}
}

// TestDeteministicity verifies that the same seed generates the same topology
func (n *TopologyTestSuite) TestDeteministicity() {
	var top middleware.Topology = &libp2p.RandPermTopology{}
	// topology of size count/2
	topSize := n.count / 2
	var previous, current []string

	for i := 0; i < n.count; i++ {
		previous = current
		current = nil
		// generate a new topology with a the same ids, size and seed
		idMap, err := top.Subset(n.ids, topSize, "sameseed")
		require.NoError(n.Suite.T(), err)

		for _, v := range idMap {
			current = append(current, v.NodeID.String())
		}
		// no guarantees about order is made by Topology.Subset(), hence sort the return values before comparision
		sort.Strings(current)

		if previous == nil {
			continue
		}

		// assert that a different seed generates a different topology
		require.Equal(n.Suite.T(), previous, current)
	}
}

// TestUniqueness verifies that different seeds generates different topologies
func (n *TopologyTestSuite) TestUniqueness() {
	var top middleware.Topology = &libp2p.RandPermTopology{}
	// topology of size count/2
	topSize := n.count / 2
	var previous, current []string

	for i := 0; i < n.count; i++ {
		previous = current
		current = nil
		// generate a new topology with a the same ids, size but a different seed for each iteration
		identity, _ := n.ids.ByIndex(uint(i))
		idMap, err := top.Subset(n.ids, topSize, identity.NodeID.String())
		require.NoError(n.Suite.T(), err)

		for _, v := range idMap {
			current = append(current, v.NodeID.String())
		}
		sort.Strings(current)

		if previous == nil {
			continue
		}

		// assert that a different seed generates a different topology
		require.NotEqual(n.Suite.T(), previous, current)
	}
}
