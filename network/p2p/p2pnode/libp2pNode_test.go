package p2pnode_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/internal/testutils"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/utils"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMultiAddress evaluates correct translations from
// dns and ip4 to libp2p multi-address
func TestMultiAddress(t *testing.T) {
	key := p2pfixtures.NetworkingKeyFixtures(t)

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
		ip, port, _, err := p2putils.NetworkingInfo(*tc.identity)
		require.NoError(t, err)

		actualAddress := utils.MultiAddressStr(ip, port)
		assert.Equal(t, tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

}

// TestSingleNodeLifeCycle evaluates correct lifecycle translation from start to stop the node
func TestSingleNodeLifeCycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	node, _ := p2ptest.NodeFixture(
		t,
		unittest.IdentifierFixture(),
		"test_single_node_life_cycle",
	)

	node.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, node)

	cancel()
	unittest.RequireComponentsDoneBefore(t, 100*time.Millisecond, node)
}

// TestGetPeerInfo evaluates the deterministic translation between the nodes address and
// their libp2p info. It generates an address, and checks whether repeated translations
// yields the same info or not.
func TestGetPeerInfo(t *testing.T) {
	for i := 0; i < 10; i++ {
		key := p2pfixtures.NetworkingKeyFixtures(t)

		// creates node-i identity
		identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("1.1.1.1:0"))

		// translates node-i address into info
		info, err := utils.PeerAddressInfo(*identity)
		require.NoError(t, err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := utils.PeerAddressInfo(*identity)
			require.NoError(t, err)
			assert.Equal(t, rinfo.String(), info.String(), "inconsistent id generated")
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func TestAddPeers(t *testing.T) {
	count := 3
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// create nodes
	nodes, identities := p2ptest.NodesFixture(t, unittest.IdentifierFixture(), "test_add_peers", count)
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	// add the remaining nodes to the first node as its set of peers
	for _, identity := range identities[1:] {
		peerInfo, err := utils.PeerAddressInfo(*identity)
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
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// create nodes
	nodes, identities := p2ptest.NodesFixture(t, unittest.IdentifierFixture(), "test_remove_peers", count)
	peerInfos, errs := utils.PeerInfosFromIDs(identities)
	assert.Len(t, errs, 0)

	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

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
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()

	node1Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node1, identity1 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		p2ptest.WithConnectionGater(testutils.NewConnectionGater(func(pid peer.ID) error {
			if !node1Peers.Has(pid) {
				return fmt.Errorf("peer id not found: %s", pid.String())
			}
			return nil
		})))

	p2ptest.StartNode(t, signalerCtx, node1, 100*time.Millisecond)
	defer p2ptest.StopNode(t, node1, cancel, 100*time.Millisecond)

	node1Info, err := utils.PeerAddressInfo(identity1)
	assert.NoError(t, err)

	node2Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node2, identity2 := p2ptest.NodeFixture(
		t,
		sporkID, t.Name(),
		p2ptest.WithConnectionGater(testutils.NewConnectionGater(func(pid peer.ID) error {
			if !node2Peers.Has(pid) {
				return fmt.Errorf("id not found: %s", pid.String())
			}
			return nil
		})))
	p2ptest.StartNode(t, signalerCtx, node2, 100*time.Millisecond)
	defer p2ptest.StopNode(t, node2, cancel, 100*time.Millisecond)

	node2Info, err := utils.PeerAddressInfo(identity2)
	assert.NoError(t, err)

	node1.Host().Peerstore().AddAddrs(node2Info.ID, node2Info.Addrs, peerstore.PermanentAddrTTL)
	node2.Host().Peerstore().AddAddrs(node1Info.ID, node1Info.Addrs, peerstore.PermanentAddrTTL)

	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.Error(t, err, "connection should not be possible")

	_, err = node2.CreateStream(ctx, node1Info.ID)
	assert.Error(t, err, "connection should not be possible")

	node1Peers.Add(node2Info.ID, struct{}{})
	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.Error(t, err, "connection should not be possible")

	node2Peers.Add(node1Info.ID, struct{}{})
	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.NoError(t, err, "connection should not be blocked")
}

// TestNode_HasSubscription checks that when a node subscribes to a topic HasSubscription should return true.
func TestNode_HasSubscription(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()
	node, _ := p2ptest.NodeFixture(t, sporkID, "test_has_subscription")

	p2ptest.StartNode(t, signalerCtx, node, 100*time.Millisecond)
	defer p2ptest.StopNode(t, node, cancel, 100*time.Millisecond)

	logger := unittest.Logger()
	met := mock.NewNetworkMetrics(t)

	topicValidator := validator.TopicValidator(logger, unittest.NetworkCodec(), unittest.NetworkSlashingViolationsConsumer(logger, met), func(id peer.ID) error {
		return nil
	})

	// create test topic
	topic := channels.TopicFromChannel(channels.TestNetworkChannel, unittest.IdentifierFixture())
	_, err := node.Subscribe(topic, topicValidator)
	require.NoError(t, err)

	require.True(t, node.HasSubscription(topic))

	// create topic with no subscription
	topic = channels.TopicFromChannel(channels.ConsensusCommittee, unittest.IdentifierFixture())
	require.False(t, node.HasSubscription(topic))
}
