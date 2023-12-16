package dht_test

import (
	"context"
	"testing"
	"time"

	golog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	libp2pmsg "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/dht"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const ticksForAssertEventually = 10 * time.Millisecond

// TestFindPeerWithDHT checks that if a node is configured to participate in the DHT, it is
// able to create new streams with peers even without knowing their address info beforehand.
func TestFindPeerWithDHT(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	count := 10
	golog.SetAllLoggers(golog.LevelFatal) // change this to Debug if libp2p logs are needed

	sporkId := unittest.IdentifierFixture()
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	dhtServerNodes, serverIDs := p2ptest.NodesFixture(
		t,
		sporkId,
		"dht_test",
		2,
		idProvider,
		p2ptest.WithRole(flow.RoleExecution),
		p2ptest.WithDHTOptions(dht.AsServer()))
	require.Len(t, dhtServerNodes, 2)

	dhtClientNodes, clientIDs := p2ptest.NodesFixture(
		t,
		sporkId,
		"dht_test",
		count-2,
		idProvider,
		p2ptest.WithRole(flow.RoleExecution),
		p2ptest.WithDHTOptions(dht.AsClient()))

	nodes := append(dhtServerNodes, dhtClientNodes...)
	idProvider.SetIdentities(append(serverIDs, clientIDs...))
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	getDhtServerAddr := func(i uint) peer.AddrInfo {
		return peer.AddrInfo{ID: dhtServerNodes[i].ID(), Addrs: dhtServerNodes[i].Host().Addrs()}
	}

	// connect even numbered clients to the first DHT server, and odd number clients to the second
	for i, clientNode := range dhtClientNodes {
		err := clientNode.Host().Connect(ctx, getDhtServerAddr(uint(i%2)))
		require.NoError(t, err)
	}

	// wait for clients to connect to DHT servers and update their routing tables
	require.Eventually(
		t, func() bool {
			for i, clientNode := range dhtClientNodes {
				if clientNode.RoutingTable().Find(getDhtServerAddr(uint(i%2)).ID) == "" {
					return false
				}
			}
			return true
		}, time.Second*5, ticksForAssertEventually, "nodes failed to connect")

	// connect the two DHT servers to each other
	err := dhtServerNodes[0].Host().Connect(ctx, getDhtServerAddr(1))
	require.NoError(t, err)

	// wait for the first server to connect to the second and update its routing table
	require.Eventually(
		t, func() bool {
			return dhtServerNodes[0].RoutingTable().Find(getDhtServerAddr(1).ID) != ""
		}, time.Second*5, ticksForAssertEventually, "dht servers failed to connect")

	// check that all even numbered clients can create streams with all odd numbered clients
	for i := 0; i < len(dhtClientNodes); i += 2 {
		for j := 1; j < len(dhtClientNodes); j += 2 {
			// client i should not yet know the address of client j, but we clear any addresses
			// here just in case.
			dhtClientNodes[i].Host().Peerstore().ClearAddrs(dhtClientNodes[j].ID())

			// Try to create a stream from client i to client j. This should resort to a DHT
			// lookup since client i does not know client j's address.
			unittest.RequireReturnsBefore(
				t, func() {
					err = dhtClientNodes[i].OpenAndWriteOnStream(
						ctx, dhtClientNodes[j].ID(), t.Name(), func(stream network.Stream) error {
							// do nothing
							require.NotNil(t, stream)
							return nil
						})
					require.NoError(t, err)
				}, 1*time.Second, "could not create stream on time")
		}
	}
}

// TestPubSub checks if nodes can subscribe to a topic and send and receive a message on that topic. The DHT discovery
// mechanism is used for nodes to find each other.
func TestPubSubWithDHTDiscovery(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "failing on CI")

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	count := 5
	golog.SetAllLoggers(golog.LevelFatal) // change this to Debug if libp2p logs are needed

	// Step 1: Creates nodes
	// Nodes will be connected in a hub and spoke configuration where one node will act as the DHT server,
	// while the other nodes will act as the client.
	// The hub-spoke configuration should eventually converge to a fully connected graph as all nodes discover
	// each other via the central node.
	// We have less than 6 nodes in play, hence the full mesh. LibP2P would limit max connections to 12 if there were
	// more nodes.
	//
	//  Initial configuration  =>  Final/expected configuration
	//   N2      N3                     N2-----N3
	//      \  /                        | \   / |
	//       N1             =>          |   N1  |
	//     /   \                        | /   \ |
	//   N4     N5                      N4-----N5

	sporkId := unittest.IdentifierFixture()
	topic := channels.TopicFromChannel(channels.TestNetworkChannel, sporkId)
	idProvider := mockmodule.NewIdentityProvider(t)
	// create one node running the DHT Server (mimicking the staked AN)
	dhtServerNodes, serverIDs := p2ptest.NodesFixture(
		t,
		sporkId,
		"dht_test",
		1,
		idProvider,
		p2ptest.WithRole(flow.RoleExecution),
		p2ptest.WithDHTOptions(dht.AsServer()))
	require.Len(t, dhtServerNodes, 1)
	dhtServerNode := dhtServerNodes[0]

	// crate other nodes running the DHT Client (mimicking the unstaked ANs)
	dhtClientNodes, clientIDs := p2ptest.NodesFixture(
		t,
		sporkId,
		"dht_test",
		count-1,
		idProvider,
		p2ptest.WithRole(flow.RoleExecution),
		p2ptest.WithDHTOptions(dht.AsClient()))

	ids := append(serverIDs, clientIDs...)
	nodes := append(dhtServerNodes, dhtClientNodes...)
	for i, node := range nodes {
		idProvider.On("ByPeerID", node.ID()).Return(&ids[i], true).Maybe()

	}
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	// Step 2: Connect all nodes running a DHT client to the node running the DHT server
	// This has to be done before subscribing to any topic, otherwise the node gives up on advertising
	// its topics of interest and becomes undiscoverable by other nodes
	// (see: https://github.com/libp2p/go-libp2p-pubsub/issues/442)
	dhtServerAddr := peer.AddrInfo{ID: dhtServerNode.ID(), Addrs: dhtServerNode.Host().Addrs()}
	for _, clientNode := range dhtClientNodes {
		err := clientNode.Host().Connect(ctx, dhtServerAddr)
		require.NoError(t, err)
	}

	// Step 3: Subscribe to the test topic
	// A node will receive its own message (https://github.com/libp2p/go-libp2p-pubsub/issues/65)
	// hence expect count and not count - 1 messages to be received (one by each node, including the sender)
	ch := make(chan peer.ID, count)

	messageScope, err := message.NewOutgoingScope(
		ids.NodeIDs(),
		topic,
		&libp2pmsg.TestMessage{},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypePubSub)
	require.NoError(t, err)

	logger := unittest.Logger()
	topicValidator := flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter())
	for _, n := range nodes {
		s, err := n.Subscribe(topic, topicValidator)
		require.NoError(t, err)

		go func(s p2p.Subscription, nodeID peer.ID) {
			msg, err := s.Next(ctx)
			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, messageScope.Proto().Payload, msg.Data)
			ch <- nodeID
		}(s, n.ID())
	}

	// fullyConnectedGraph checks that each node is directly connected to all the other nodes
	fullyConnectedGraph := func() bool {
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				if nodes[i].Host().Network().Connectedness(nodes[j].ID()) == network.NotConnected {
					return false
				}
			}
		}
		return true
	}
	// assert that the graph is fully connected
	require.Eventually(t, fullyConnectedGraph, time.Second*5, ticksForAssertEventually, "nodes failed to discover each other")

	// Step 4: publish a message to the topic
	require.NoError(t, dhtServerNode.Publish(ctx, messageScope))

	// Step 5: By now, all peers would have been discovered and the message should have been successfully published
	// A hash set to keep track of the nodes who received the message
	recv := make(map[peer.ID]bool, count)

loop:
	for i := 0; i < count; i++ {
		select {
		case res := <-ch:
			recv[res] = true
		case <-time.After(3 * time.Second):
			var missing peer.IDSlice
			for _, n := range nodes {
				if _, found := recv[n.ID()]; !found {
					missing = append(missing, n.ID())
				}
			}
			assert.Failf(t, "messages not received by some nodes", "%+v", missing)
			break loop
		}
	}

	// Step 6: unsubscribes all nodes from the topic
	for _, n := range nodes {
		assert.NoError(t, n.Unsubscribe(topic))
	}
}
