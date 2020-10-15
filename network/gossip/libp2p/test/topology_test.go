package test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/utils/unittest"
)

// TopologyTestSuite tests the bare minimum requirements of a randomized
// topology that is needed for our network. It should not replace the information
// theory assumptions behind the schemes, e.g., random oracle model of hashes
type TopologyTestSuite struct {
	suite.Suite
	ids flow.IdentityList // represents the identity list of all nodes in the system
	net *libp2p.Network   // represents the single network instance that creates topology
	me  flow.Identity     // represents identity of single instance of node that creates topology
}

// TestNetworkTestSuit starts all the tests in this test suite
func TestNetworkTestSuit(t *testing.T) {
	suite.Run(t, new(TopologyTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (suite *TopologyTestSuite) SetupTest() {
	// creates 20 nodes of each type, 100 nodes overall.
	suite.ids = unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleCollection))
	suite.ids = append(suite.ids, unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleConsensus))...)
	suite.ids = append(suite.ids, unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleVerification))...)
	suite.ids = append(suite.ids, unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleExecution))...)
	suite.ids = append(suite.ids, unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleAccess))...)

	// takes firs id as the current nodes id
	suite.me = *suite.ids[0]

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	key, err := GenerateNetworkingKey(suite.me.NodeID)
	require.NoError(suite.T(), err)

	// creates a middleware instance
	mw, err := libp2p.NewMiddleware(logger,
		json.NewCodec(),
		"0.0.0.0:0",
		suite.me.NodeID,
		key,
		metrics.NewNoopCollector(),
		libp2p.DefaultMaxUnicastMsgSize,
		libp2p.DefaultMaxPubSubMsgSize,
		unittest.IdentifierFixture().String())
	require.NoError(suite.T(), err)

	// creates and returns a topic aware topology instance
	top, err := topology.NewTopicAwareTopology(suite.me.NodeID)
	require.NoError(suite.T(), err)

	// creates a mock network with the topology instance
	nets, err := createNetworks(logger, []*libp2p.Middleware{mw}, suite.ids, 1, true, top)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), nets, 1)
	suite.net = nets[0]
}

func (suite *TopologyTestSuite) TestTopologySize() {
	//// topology of size the entire network
	//top, err := suite.net.Topology()
	//require.NoError(suite.T(), err)
	//require.Len(suite.T(), top, suite.expectedSize)
}

// TestMembership evaluates every id in topology to be a protocol id
func (suite *TopologyTestSuite) TestMembership() {
	top, err := suite.net.Topology()
	require.NoError(suite.T(), err)

	// every id in topology should be an id of the protocol
	for id := range top {
		require.Contains(suite.T(), suite.ids.NodeIDs(), id)
	}
}

//// TestDeteministicity verifies that the same seed generates the same topology
//func (suite *TopologyTestSuite) TestDeteministicity() {
//	top, err := topology.NewRandPermTopology(flow.RoleCollection, unittest.IdentifierFixture())
//	require.NoError(suite.T(), err)
//	// topology of size count/2
//	topSize := uint(suite.count / 2)
//	var previous, current []string
//
//	for i := 0; i < suite.count; i++ {
//		previous = current
//		current = nil
//		// generate a new topology with a the same ids, size and seed
//		idMap, err := top.Subset(suite.ids, topSize, topology.DummyTopic)
//		require.NoError(suite.T(), err)
//
//		for _, v := range idMap {
//			current = append(current, v.NodeID.String())
//		}
//		// no guarantees about order is made by Topology.Subset(), hence sort the return values before comparision
//		sort.Strings(current)
//
//		if previous == nil {
//			continue
//		}
//
//		// assert that a different seed generates a different topology
//		require.Equal(suite.T(), previous, current)
//	}
//}
//
//// TestUniqueness verifies that different seeds generates different topologies
//func (suite *TopologyTestSuite) TestUniqueness() {
//
//	// topology of size count/2
//	topSize := uint(suite.count / 2)
//	var previous, current []string
//
//	for i := 0; i < suite.count; i++ {
//		previous = current
//		current = nil
//		// generate a new topology with a the same ids, size but a different seed for each iteration
//		identity, _ := suite.ids.ByIndex(uint(i))
//		top, err := topology.NewRandPermTopology(flow.RoleCollection, identity.NodeID)
//		require.NoError(suite.T(), err)
//		idMap, err := top.Subset(suite.ids, topSize, topology.DummyTopic)
//		require.NoError(suite.T(), err)
//
//		for _, v := range idMap {
//			current = append(current, v.NodeID.String())
//		}
//		sort.Strings(current)
//
//		if previous == nil {
//			continue
//		}
//
//		// assert that a different seed generates a different topology
//		require.NotEqual(suite.T(), previous, current)
//	}
//}
