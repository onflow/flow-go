package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMultiAddress evaluates correct translations from dns and ip4 to libp2p multi-address
func TestMultiAddress(t *testing.T) {
	key := generateNetworkingKey(t)

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
		require.NoError(t, err)

		actualAddress := MultiAddressStr(ip, port)
		assert.Equal(t, tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

}

// TestSingleNodeLifeCycle evaluates correct lifecycle translation from start to stop the node
func TestSingleNodeLifeCycle(t *testing.T) {
	key := generateNetworkingKey(t)
	node, _ := nodeFixture(t, unittest.Logger(), key, rootBlockID, defaultAddress)

	done, err := node.Stop()
	unittest.RequireCloseBefore(t, done, 100*time.Millisecond, "could not stop node on time")
	assert.NoError(t, err)
}

// TestGetPeerInfo evaluates the deterministic translation between the nodes address and
// their libp2p info. It generates an address, and checks whether repeated translations
// yields the same info or not.
func TestGetPeerInfo(t *testing.T) {
	for i := 0; i < 10; i++ {
		key := generateNetworkingKey(t)

		// creates node-i identity
		identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("1.1.1.1:0"))

		// translates node-i address into info
		info, err := PeerAddressInfo(*identity)
		require.NoError(t, err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := PeerAddressInfo(*identity)
			require.NoError(t, err)
			assert.True(t, rinfo.String() == info.String(), "inconsistent id generated")
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func TestAddPeers(t *testing.T) {
	count := 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create nodes
	nodes, identities := nodesFixture(t, count)
	defer stopNodes(t, nodes)

	// add the remaining nodes to the first node as its set of peers
	for _, identity := range identities[1:] {
		peerInfo, err := PeerAddressInfo(*identity)
		require.NoError(t, err)
		require.NoError(t, nodes[0].AddPeer(ctx, peerInfo))
	}

	// Checks if both of the other nodes have been added as peers to the first node
	assert.Len(t, nodes[0].host.Network().Peers(), count-1)
}

// TestRemovePeers checks if nodes can be removed as peers from a given node
func TestRemovePeers(t *testing.T) {
	count := 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create nodes
	nodes, identities := nodesFixture(t, count)
	peerInfos, errs := peerInfosFromIDs(identities)
	assert.Len(t, errs, 0)
	defer stopNodes(t, nodes)

	// add nodes two and three to the first node as its peers
	for _, pInfo := range peerInfos[1:] {
		require.NoError(t, nodes[0].AddPeer(ctx, pInfo))
	}

	// check if all other nodes have been added as peers to the first node
	assert.Len(t, nodes[0].host.Network().Peers(), count-1)

	// disconnect from each peer and assert that the connection no longer exists
	for _, pInfo := range peerInfos[1:] {
		require.NoError(t, nodes[0].RemovePeer(pInfo.ID))
		assert.Equal(t, network.NotConnected, nodes[0].host.Network().Connectedness(pInfo.ID))
	}
}

// TestPing tests that a node can ping another node
func TestPing(t *testing.T) {

	// creates two nodes
	nodes, identities := nodesFixture(t, 2)
	defer stopNodes(t, nodes)

	node1 := nodes[0]
	node2 := nodes[1]
	node1Id := *identities[0]
	node2Id := *identities[1]

	_, expectedVersion, expectedHeight, expectedView := mockPingInfoProvider()

	// test node1 can ping node 2
	testPing(t, node1, node2Id, expectedVersion, expectedHeight, expectedView)

	// test node 2 can ping node 1
	testPing(t, node2, node1Id, expectedVersion, expectedHeight, expectedView)
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

func TestConnectionGatingBootstrap(t *testing.T) {
	// Create a Node with AllowList = false
	node, identity := nodesFixture(t, 1)
	node1 := node[0]
	node1Id := identity[0]
	defer stopNode(t, node1)
	node1Info, err := PeerAddressInfo(*node1Id)
	assert.NoError(t, err)

	t.Run("updating allowlist of node w/o ConnGater does not crash", func(t *testing.T) {
		// node1 allowlists node1
		node1.UpdateAllowList(peer.IDSlice{node1Info.ID})
	})
}
