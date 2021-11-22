package p2p

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	addrutil "github.com/libp2p/go-addr-util"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/utils/unittest"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const tickForAssertEventually = 100 * time.Millisecond

// Creating a node fixture with defaultAddress lets libp2p runs it on an
// allocated port by OS. So after fixture created, its address would be
// "0.0.0.0:<selected-port-by-os>
const defaultAddress = "0.0.0.0:0"

var sporkID = unittest.IdentifierFixture()

type LibP2PNodeTestSuite struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc // used to cancel the context
	logger zerolog.Logger
}

// TestLibP2PNodesTestSuite runs all the test methods in this test suit
func TestLibP2PNodesTestSuite(t *testing.T) {
	suite.Run(t, new(LibP2PNodeTestSuite))
}

// SetupTests initiates the test setups prior to each test
func (suite *LibP2PNodeTestSuite) SetupTest() {
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	golog.SetAllLoggers(golog.LevelError)
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
}

func (suite *LibP2PNodeTestSuite) TearDownTest() {
	suite.cancel()
}

// TestMultiAddress evaluates correct translations from
// dns and ip4 to libp2p multi-address
func (suite *LibP2PNodeTestSuite) TestMultiAddress() {
	key := generateNetworkingKey(suite.T())

	tt := []struct {
		identity     *flow.Identity
		multiaddress string
	}{
		{ // ip4 test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("172.16.254.1:72")),
			multiaddress: "/ip4/172.16.254.1/tcp/72",
		},
		{ // dns test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("consensus:2222")),
			multiaddress: "/dns4/consensus/tcp/2222",
		},
		{ // dns test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("flow.com:3333")),
			multiaddress: "/dns4/flow.com/tcp/3333",
		},
	}

	for _, tc := range tt {
		ip, port, _, err := networkingInfo(*tc.identity)
		require.NoError(suite.T(), err)

		actualAddress := MultiAddressStr(ip, port)
		assert.Equal(suite.T(), tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

}

func (suite *LibP2PNodeTestSuite) TestSingleNodeLifeCycle() {
	// creates a single
	key := generateNetworkingKey(suite.T())
	node, _ := NodeFixture(suite.T(), suite.logger, key, sporkID, nil, false, defaultAddress)

	// stops the created node
	done, err := node.Stop()
	assert.NoError(suite.T(), err)
	<-done
}

// TestGetPeerInfo evaluates the deterministic translation between the nodes address and
// their libp2p info. It generates an address, and checks whether repeated translations
// yields the same info or not.
func (suite *LibP2PNodeTestSuite) TestGetPeerInfo() {
	for i := 0; i < 10; i++ {
		key := generateNetworkingKey(suite.T())

		// creates node-i identity
		identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("1.1.1.1:0"))

		// translates node-i address into info
		info, err := PeerAddressInfo(*identity)
		require.NoError(suite.T(), err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := PeerAddressInfo(*identity)
			require.NoError(suite.T(), err)
			assert.True(suite.T(), rinfo.String() == info.String(), "inconsistent id generated")
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func (suite *LibP2PNodeTestSuite) TestAddPeers() {
	count := 3

	// create nodes
	nodes, identities := NodesFixtureWithHandler(suite.T(), count, nil, false)
	defer StopNodes(suite.T(), nodes)

	// add the remaining nodes to the first node as its set of peers
	for _, identity := range identities[1:] {
		peerInfo, err := PeerAddressInfo(*identity)
		require.NoError(suite.T(), err)
		require.NoError(suite.T(), nodes[0].AddPeer(suite.ctx, peerInfo))
	}

	// Checks if both of the other nodes have been added as peers to the first node
	assert.Len(suite.T(), nodes[0].host.Network().Peers(), count-1)
}

// TestRemovePeers checks if nodes can be removed as peers from a given node
func (suite *LibP2PNodeTestSuite) TestRemovePeers() {

	count := 3

	// create nodes
	nodes, identities := NodesFixtureWithHandler(suite.T(), count, nil, false)
	peerInfos, errs := peerInfosFromIDs(identities)
	assert.Len(suite.T(), errs, 0)
	defer StopNodes(suite.T(), nodes)

	// add nodes two and three to the first node as its peers
	for _, pInfo := range peerInfos[1:] {
		require.NoError(suite.T(), nodes[0].AddPeer(suite.ctx, pInfo))
	}

	// check if all other nodes have been added as peers to the first node
	assert.Len(suite.T(), nodes[0].host.Network().Peers(), count-1)

	// disconnect from each peer and assert that the connection no longer exists
	for _, pInfo := range peerInfos[1:] {
		require.NoError(suite.T(), nodes[0].RemovePeer(suite.ctx, pInfo.ID))
		assert.Equal(suite.T(), network.NotConnected, nodes[0].host.Network().Connectedness(pInfo.ID))
	}
}

// TestPing tests that a node can ping another node
func (suite *LibP2PNodeTestSuite) TestPing() {

	// creates two nodes
	nodes, identities := NodesFixtureWithHandler(suite.T(), 2, nil, false)
	defer StopNodes(suite.T(), nodes)

	node1 := nodes[0]
	node2 := nodes[1]
	node1Id := *identities[0]
	node2Id := *identities[1]

	_, expectedVersion, expectedHeight, expectedView := MockPingInfoProvider()

	// test node1 can ping node 2
	testPing(suite.T(), node1, node2Id, expectedVersion, expectedHeight, expectedView)

	// test node 2 can ping node 1
	testPing(suite.T(), node2, node1Id, expectedVersion, expectedHeight, expectedView)
}

func testPing(t *testing.T, source *Node, target flow.Identity, expectedVersion string, expectedHeight uint64, expectedView uint64) {
	pctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pInfo, err := PeerAddressInfo(target)
	assert.NoError(t, err)
	source.host.Peerstore().AddAddrs(pInfo.ID, pInfo.Addrs, peerstore.AddressTTL)
	resp, rtt, err := source.Ping(pctx, pInfo.ID)
	assert.NoError(t, err)
	assert.NotZero(t, rtt)
	assert.Equal(t, expectedVersion, resp.Version)
	assert.Equal(t, expectedHeight, resp.BlockHeight)
	assert.Equal(t, expectedView, resp.HotstuffView)
}

func (suite *LibP2PNodeTestSuite) TestConnectionGatingBootstrap() {
	// Create a Node with AllowList = false
	node, identity := NodesFixtureWithHandler(suite.T(), 1, nil, false)
	node1 := node[0]
	node1Id := identity[0]
	defer StopNode(suite.T(), node1)
	node1Info, err := PeerAddressInfo(*node1Id)
	assert.NoError(suite.T(), err)

	suite.Run("updating allowlist of node w/o ConnGater does not crash", func() {
		// node1 allowlists node1
		node1.UpdateAllowList(peer.IDSlice{node1Info.ID})
	})
}

// NodesFixtureWithHandler creates a number of LibP2PNodes with the given callback function for stream handling.
// It returns the nodes and their identities.
func NodesFixtureWithHandler(t *testing.T, count int, handler network.StreamHandler, allowList bool) ([]*Node, flow.IdentityList) {
	// keeps track of errors on creating a node
	var err error
	var nodes []*Node

	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			StopNodes(t, nodes)
		}
	}()

	// creating nodes
	var identities flow.IdentityList
	for i := 0; i < count; i++ {
		// create a node on localhost with a random port assigned by the OS
		key := generateNetworkingKey(t)
		node, identity := NodeFixture(t, unittest.Logger(), key, sporkID, handler, allowList, defaultAddress)
		nodes = append(nodes, node)
		identities = append(identities, &identity)
	}
	return nodes, identities
}

// NodeFixture creates a single LibP2PNodes with the given key, root block id, and callback function for stream handling.
// It returns the nodes and their identities.
func NodeFixture(t *testing.T, log zerolog.Logger, key fcrypto.PrivateKey, sporkID flow.Identifier, handler network.StreamHandler, allowList bool, address string) (*Node, flow.Identity) {

	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress(address))

	var handlerFunc network.StreamHandler
	if handler != nil {
		// use the callback that has been passed in
		handlerFunc = handler
	} else {
		// use a default call back
		handlerFunc = func(network.Stream) {}
	}

	pingInfoProvider, _, _, _ := MockPingInfoProvider()

	// dns resolver
	resolver := dns.NewResolver(metrics.NewNoopCollector())
	unittest.RequireCloseBefore(t, resolver.Ready(), 10*time.Millisecond, "could not start resolver")

	noopMetrics := metrics.NewNoopCollector()
	connManager := NewConnManager(log, noopMetrics)

	builder := NewDefaultLibP2PNodeBuilder(identity.NodeID, address, key).
		SetSporkID(sporkID).
		SetConnectionManager(connManager).
		SetPingInfoProvider(pingInfoProvider).
		SetResolver(resolver).
		SetTopicValidation(false).
		SetStreamCompressor(WithGzipCompression).
		SetLogger(log)

	if allowList {
		connGater := NewConnGater(log)
		builder.SetConnectionGater(connGater)
	}

	ctx := context.Background()
	n, err := builder.Build(ctx)
	require.NoError(t, err)

	n.SetFlowProtocolStreamHandler(handlerFunc)

	require.Eventuallyf(t, func() bool {
		ip, p, err := n.GetIPPort()
		return err == nil && ip != "" && p != ""
	}, 3*time.Second, tickForAssertEventually, fmt.Sprintf("could not start node %s", identity.NodeID.String()))

	// get the actual IP and port that have been assigned by the subsystem
	ip, port, err := n.GetIPPort()
	require.NoError(t, err)
	identity.Address = ip + ":" + port

	return n, *identity
}

func MockPingInfoProvider() (*mocknetwork.PingInfoProvider, string, uint64, uint64) {
	version := "version_1"
	height := uint64(5000)
	view := uint64(10)
	pingInfoProvider := new(mocknetwork.PingInfoProvider)
	pingInfoProvider.On("SoftwareVersion").Return(version)
	pingInfoProvider.On("SealedBlockHeight").Return(height)
	pingInfoProvider.On("HotstuffView").Return(view)
	return pingInfoProvider, version, height, view
}

// StopNodes stop all nodes in the input slice
func StopNodes(t *testing.T, nodes []*Node) {
	for _, n := range nodes {
		StopNode(t, n)
	}
	fmt.Println("[debug] nodes stopped")
}

func StopNode(t *testing.T, node *Node) {
	addr := node.host.Addrs()
	fmt.Printf("[debug] stopping %v\n", addr)
	done, err := node.Stop()
	assert.NoError(t, err)
	<-done
	fmt.Printf("[debug] stopped %v\n", addr)
}

// generateNetworkingKey is a test helper that generates a ECDSA flow key pair.
func generateNetworkingKey(t *testing.T) fcrypto.PrivateKey {
	seed := unittest.SeedFixture(48)
	key, err := fcrypto.GeneratePrivateKey(fcrypto.ECDSASecp256k1, seed)
	require.NoError(t, err)
	return key
}

// silentNodeFixture returns a TCP listener and a node which never replies
func silentNodeFixture(t *testing.T) (net.Listener, flow.Identity) {
	key := generateNetworkingKey(t)

	lst, err := net.Listen("tcp4", ":0")
	require.NoError(t, err)

	addr, err := manet.FromNetAddr(lst.Addr())
	require.NoError(t, err)

	addrs := []multiaddr.Multiaddr{addr}
	addrs, err = addrutil.ResolveUnspecifiedAddresses(addrs, nil)
	require.NoError(t, err)

	go acceptAndHang(t, lst)

	ip, port, err := IPPortFromMultiAddress(addrs...)
	require.NoError(t, err)

	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress(ip+":"+port))
	return lst, *identity
}

func acceptAndHang(t *testing.T, l net.Listener) {
	conns := make([]net.Conn, 0, 10)
	for {
		c, err := l.Accept()
		if err != nil {
			break
		}
		if c != nil {
			conns = append(conns, c)
		}
	}
	for _, c := range conns {
		require.NoError(t, c.Close())
	}
}
