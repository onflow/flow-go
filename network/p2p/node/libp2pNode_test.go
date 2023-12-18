package p2pnode_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/p2p/test"
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
		{
			// ip4 test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("172.16.254.1:72")),
			multiaddress: "/ip4/172.16.254.1/tcp/72",
		},
		{
			// dns test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("consensus:2222")),
			multiaddress: "/dns4/consensus/tcp/2222",
		},
		{
			// dns test case
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
	idProvider := mockmodule.NewIdentityProvider(t)
	node, _ := p2ptest.NodeFixture(t, unittest.IdentifierFixture(), "test_single_node_life_cycle", idProvider)

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
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})

	nodes, identities := p2ptest.NodesFixture(t, unittest.IdentifierFixture(), "test_add_peers", count, idProvider)
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	// add the remaining nodes to the first node as its set of peers
	for _, identity := range identities[1:] {
		peerInfo, err := utils.PeerAddressInfo(*identity)
		require.NoError(t, err)
		require.NoError(t, nodes[0].ConnectToPeer(ctx, peerInfo))
	}

	// Checks if both of the other nodes have been added as peers to the first node
	assert.Len(t, nodes[0].Host().Network().Peers(), count-1)
}

// TestRemovePeers checks if nodes can be removed as peers from a given node
func TestRemovePeers(t *testing.T) {
	count := 3
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	// create nodes
	nodes, identities := p2ptest.NodesFixture(t, unittest.IdentifierFixture(), "test_remove_peers", count, idProvider)
	peerInfos, errs := utils.PeerInfosFromIDs(identities)
	assert.Len(t, errs, 0)

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	// add nodes two and three to the first node as its peers
	for _, pInfo := range peerInfos[1:] {
		require.NoError(t, nodes[0].ConnectToPeer(ctx, pInfo))
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
	idProvider := mockmodule.NewIdentityProvider(t)

	node1Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node1, identity1 := p2ptest.NodeFixture(t, sporkID, t.Name(), idProvider, p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
		if !node1Peers.Has(pid) {
			return fmt.Errorf("peer id not found: %s", p2plogging.PeerId(pid))
		}
		return nil
	})))
	idProvider.On("ByPeerID", node1.ID()).Return(&identity1, true).Maybe()

	p2ptest.StartNode(t, signalerCtx, node1)
	defer p2ptest.StopNode(t, node1, cancel)

	node1Info, err := utils.PeerAddressInfo(identity1)
	assert.NoError(t, err)

	node2Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node2, identity2 := p2ptest.NodeFixture(t, sporkID, t.Name(), idProvider, p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
		if !node2Peers.Has(pid) {
			return fmt.Errorf("id not found: %s", p2plogging.PeerId(pid))
		}
		return nil
	})))
	idProvider.On("ByPeerID", node2.ID()).Return(&identity2,

		true).Maybe()

	p2ptest.StartNode(t, signalerCtx, node2)
	defer p2ptest.StopNode(t, node2, cancel)

	node2Info, err := utils.PeerAddressInfo(identity2)
	assert.NoError(t, err)

	node1.Host().Peerstore().AddAddrs(node2Info.ID, node2Info.Addrs, peerstore.PermanentAddrTTL)
	node2.Host().Peerstore().AddAddrs(node1Info.ID, node1Info.Addrs, peerstore.PermanentAddrTTL)

	err = node1.OpenAndWriteOnStream(ctx, node2Info.ID, t.Name(), func(stream network.Stream) error {
		// no-op, as the connection should not be possible
		return nil
	})
	require.ErrorContains(t, err, "target node is not on the approved list of nodes")

	err = node2.OpenAndWriteOnStream(ctx, node1Info.ID, t.Name(), func(stream network.Stream) error {
		// no-op, as the connection should not be possible
		return nil
	})
	require.ErrorContains(t, err, "target node is not on the approved list of nodes")

	node1Peers.Add(node2Info.ID, struct{}{})
	err = node1.OpenAndWriteOnStream(ctx, node2Info.ID, t.Name(), func(stream network.Stream) error {
		// no-op, as the connection should not be possible
		return nil
	})
	require.Error(t, err)

	node2Peers.Add(node1Info.ID, struct{}{})
	err = node1.OpenAndWriteOnStream(ctx, node2Info.ID, t.Name(), func(stream network.Stream) error {
		// no-op, as the connection should not be possible
		return nil
	})
	require.NoError(t, err)
}

// TestNode_HasSubscription checks that when a node subscribes to a topic HasSubscription should return true.
func TestNode_HasSubscription(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	sporkID := unittest.IdentifierFixture()
	node, _ := p2ptest.NodeFixture(t, sporkID, "test_has_subscription", idProvider)

	p2ptest.StartNode(t, signalerCtx, node)
	defer p2ptest.StopNode(t, node, cancel)

	logger := unittest.Logger()

	topicValidator := validator.TopicValidator(logger, func(id peer.ID) error {
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

// TestCreateStream_SinglePairwiseConnection ensures that despite the number of concurrent streams created from peer -> peer, only a single
// connection will ever be created between two peers on initial peer dialing and subsequent streams will reuse that connection.
func TestCreateStream_SinglePairwiseConnection(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	nodeCount := 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	nodes, ids := p2ptest.NodesFixture(t, sporkId, "test_create_stream_single_pairwise_connection", nodeCount, idProvider, p2ptest.WithDefaultResourceManager())
	idProvider.SetIdentities(ids)

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	numOfStreamsPerNode := 100 // create large number of streamChan per node per connection to ensure the resource manager does not cause starvation of resources
	expectedTotalNumOfStreams := 600

	// create a number of streamChan concurrently between each node
	streamChan := make(chan network.Stream, expectedTotalNumOfStreams)

	go createConcurrentStreams(t, ctxWithTimeout, nodes, ids, numOfStreamsPerNode, streamChan, done)
	unittest.RequireCloseBefore(t, done, 5*time.Second, "could not create streamChan on time")
	require.Len(t,
		streamChan,
		expectedTotalNumOfStreams,
		fmt.Sprintf("expected %d total number of streamChan created got %d", expectedTotalNumOfStreams, len(streamChan)))

	// ensure only a single connection exists between all nodes
	ensureSinglePairwiseConnection(t, nodes)
	close(streamChan)
}

// createStreams will attempt to create n number of streams concurrently between each combination of node pairs.
func createConcurrentStreams(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, ids flow.IdentityList, n int, streams chan network.Stream, done chan struct{}) {
	defer close(done)
	var wg sync.WaitGroup
	for _, this := range nodes {
		for i, other := range nodes {
			if this == other {
				continue
			}

			pInfo, err := utils.PeerAddressInfo(*ids[i])
			require.NoError(t, err)
			this.Host().Peerstore().AddAddrs(pInfo.ID, pInfo.Addrs, peerstore.AddressTTL)

			for j := 0; j < n; j++ {
				wg.Add(1)
				go func(sender p2p.LibP2PNode) {
					defer wg.Done()
					err := sender.OpenAndWriteOnStream(ctx, pInfo.ID, t.Name(), func(stream network.Stream) error {
						streams <- stream

						// wait for the done signal to close the stream
						<-ctx.Done()
						return nil
					})
					require.NoError(t, err)
				}(this)
			}
		}
		// brief sleep to prevent sender and receiver dialing each other at the same time if separate goroutines resulting
		// in 2 connections 1 created by each node, this happens because we are calling CreateStream concurrently.
		time.Sleep(500 * time.Millisecond)
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 3*time.Second, "could not create streams on time")
}

// ensureSinglePairwiseConnection ensure each node in the list has exactly one connection to every other node in the list.
func ensureSinglePairwiseConnection(t *testing.T, nodes []p2p.LibP2PNode) {
	for _, this := range nodes {
		for _, other := range nodes {
			if this == other {
				continue
			}
			require.Len(t, this.Host().Network().ConnsToPeer(other.ID()), 1)
		}
	}
}
