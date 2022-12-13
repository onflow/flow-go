package p2p_test

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMultiAddress evaluates correct translations from
// dns and ip4 to libp2p multi-address
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
		ip, port, _, err := p2p.NetworkingInfo(*tc.identity)
		require.NoError(t, err)

		actualAddress := p2p.MultiAddressStr(ip, port)
		assert.Equal(t, tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

}

// TestSingleNodeLifeCycle evaluates correct lifecycle translation from start to stop the node
func TestSingleNodeLifeCycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node, _ := nodeFixture(
		t,
		ctx,
		unittest.IdentifierFixture(),
		"test_single_node_life_cycle",
	)

	stopNode(t, node)
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
		info, err := p2p.PeerAddressInfo(*identity)
		require.NoError(t, err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := p2p.PeerAddressInfo(*identity)
			require.NoError(t, err)
			assert.Equal(t, rinfo.String(), info.String(), "inconsistent id generated")
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func TestAddPeers(t *testing.T) {
	count := 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create nodes
	nodes, identities := nodesFixture(t, ctx, unittest.IdentifierFixture(), "test_add_peers", count)
	defer stopNodes(t, nodes)

	// add the remaining nodes to the first node as its set of peers
	for _, identity := range identities[1:] {
		peerInfo, err := p2p.PeerAddressInfo(*identity)
		require.NoError(t, err)
		require.NoError(t, nodes[0].AddPeer(ctx, peerInfo))
	}

	// Checks if both of the other nodes have been added as peers to the first node
	assert.Len(t, nodes[0].Host().Network().Peers(), count-1)
}

// TestRemovePeers checks if nodes can be removed as peers from a given node
func TestRemovePeers(t *testing.T) {
	count := 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create nodes
	nodes, identities := nodesFixture(t, ctx, unittest.IdentifierFixture(), "test_remove_peers", count)
	peerInfos, errs := p2p.PeerInfosFromIDs(identities)
	assert.Len(t, errs, 0)
	defer stopNodes(t, nodes)

	// add nodes two and three to the first node as its peers
	for _, pInfo := range peerInfos[1:] {
		require.NoError(t, nodes[0].AddPeer(ctx, pInfo))
	}

	// check if all other nodes have been added as peers to the first node
	assert.Len(t, nodes[0].Host().Network().Peers(), count-1)

	// disconnect from each peer and assert that the connection no longer exists
	for _, pInfo := range peerInfos[1:] {
		require.NoError(t, nodes[0].RemovePeer(pInfo.ID))
		assert.Equal(t, network.NotConnected, nodes[0].Host().Network().Connectedness(pInfo.ID))
	}
}

func TestConnGater(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sporkID := unittest.IdentifierFixture()

	node1Peers := make(map[peer.ID]struct{})
	node1, identity1 := nodeFixture(t, ctx, sporkID, "test_conn_gater", withPeerFilter(func(pid peer.ID) bool {
		_, ok := node1Peers[pid]
		return ok
	}))
	defer stopNode(t, node1)
	node1Info, err := p2p.PeerAddressInfo(identity1)
	assert.NoError(t, err)

	node2Peers := make(map[peer.ID]struct{})
	node2, identity2 := nodeFixture(t, ctx, sporkID, "test_conn_gater", withPeerFilter(func(pid peer.ID) bool {
		_, ok := node2Peers[pid]
		return ok
	}))
	defer stopNode(t, node2)
	node2Info, err := p2p.PeerAddressInfo(identity2)
	assert.NoError(t, err)

	node1.Host().Peerstore().AddAddrs(node2Info.ID, node2Info.Addrs, peerstore.PermanentAddrTTL)
	node2.Host().Peerstore().AddAddrs(node1Info.ID, node1Info.Addrs, peerstore.PermanentAddrTTL)

	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.Error(t, err, "connection should not be possible")

	_, err = node2.CreateStream(ctx, node1Info.ID)
	assert.Error(t, err, "connection should not be possible")

	node1Peers[node2Info.ID] = struct{}{}
	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.Error(t, err, "connection should not be possible")

	node2Peers[node1Info.ID] = struct{}{}
	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.NoError(t, err, "connection should not be blocked")
}
