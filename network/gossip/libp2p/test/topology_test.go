package test

import (
	gojson "encoding/json"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
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

	// creates a middleware instance
	mw, err := libp2p.NewMiddleware(zerolog.Logger{}, json.NewCodec(), "0.0.0.0:0", me.NodeID)
	require.NoError(n.Suite.T(), err)

	// creates and mocks a network instance
	nets, _, err := CreateNetworks([]*libp2p.Middleware{mw}, n.ids, 1, true)
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

// TestUniqueness generates n.count many random topologies of size n.count/2 and
// evaluates all those generated topology to be unique
func (n *TopologyTestSuite) TestUniqueness() {
	// creates a hasher instance
	hasher, err := crypto.NewHasher(crypto.SHA3_256)
	require.NoError(n.Suite.T(), err)

	// set of all generated topologies
	topSet := make(map[string]struct{})

	// topology of size count/2
	topSize := n.count / 2
	require.NotEqual(n.Suite.T(), topSize, 0)

	for i := 0; i < n.count; i++ {
		// generates a randomized topology of half of the size of system
		top, err := n.nets.Topology()
		require.NoError(n.Suite.T(), err)
		require.Len(n.Suite.T(), top, topSize)

		// converts topology to a sorted list
		topList := make([]string, 0)
		for id := range top {
			topList = append(topList, id.String())
		}
		sort.Strings(topList)

		// encodes the list
		fp, err := gojson.Marshal(topList)
		require.NoError(n.Suite.T(), err)
		// takes hash of encoded list
		hash := hasher.ComputeHash(fp)

		// generated topologies should be unique
		if _, ok := topSet[hash.Hex()]; ok {
			require.Fail(n.Suite.T(), "repeated topology detected")
		}

		// stores hash in the set
		topSet[hash.Hex()] = struct{}{}
	}
}
