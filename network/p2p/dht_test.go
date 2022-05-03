package p2p_test

import (
	"context"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFindPeerWithDHT checks that if a node is configured to participate in the DHT, it is
// able to create new streams with peers even without knowing their address info beforehand.
func TestFindPeerWithDHT(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	count := 10
	golog.SetAllLoggers(golog.LevelFatal) // change this to Debug if libp2p logs are needed

	sporkId := unittest.IdentifierFixture()
	dhtServerNodes, _ := nodesFixture(t, ctx, sporkId, "dht_test", 2, withDHTOptions(p2p.AsServer()))
	require.Len(t, dhtServerNodes, 2)

	dhtClientNodes, _ := nodesFixture(t, ctx, sporkId, "dht_test", count-2, withDHTOptions(p2p.AsClient()))

	nodes := append(dhtServerNodes, dhtClientNodes...)
	defer stopNodes(t, nodes)

	getDhtServerAddr := func(i uint) peer.AddrInfo {
		return peer.AddrInfo{ID: dhtServerNodes[i].Host().ID(), Addrs: dhtServerNodes[i].Host().Addrs()}
	}

	// connect even numbered clients to the first DHT server, and odd number clients to the second
	for i, clientNode := range dhtClientNodes {
		err := clientNode.Host().Connect(ctx, getDhtServerAddr(uint(i%2)))
		require.NoError(t, err)
	}

	// wait for clients to connect to DHT servers and update their routing tables
	require.Eventually(t, func() bool {
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
	require.Eventually(t, func() bool {
		return dhtServerNodes[0].RoutingTable().Find(getDhtServerAddr(1).ID) != ""
	}, time.Second*5, ticksForAssertEventually, "dht servers failed to connect")

	// check that all even numbered clients can create streams with all odd numbered clients
	for i := 0; i < len(dhtClientNodes); i += 2 {
		for j := 1; j < len(dhtClientNodes); j += 2 {
			// client i should not yet know the address of client j, but we clear any addresses
			// here just in case.
			dhtClientNodes[i].Host().Peerstore().ClearAddrs(dhtClientNodes[j].Host().ID())

			// Try to create a stream from client i to client j. This should resort to a DHT
			// lookup since client i does not know client j's address.
			unittest.RequireReturnsBefore(t, func() {
				_, err = dhtClientNodes[i].CreateStream(ctx, dhtClientNodes[j].Host().ID())
				require.NoError(t, err)
			}, 1*time.Second, "could not create stream on time")
		}
	}
}

// TestPubSub checks if nodes can subscribe to a topic and send and receive a message on that topic. The DHT discovery
// mechanism is used for nodes to find each other.
func TestPubSubWithDHTDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topic := flownet.Topic("/flow/" + unittest.IdentifierFixture().String())
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
	// create one node running the DHT Server (mimicking the staked AN)
	dhtServerNodes, _ := nodesFixture(t, ctx, sporkId, "dht_test", 1, withDHTOptions(p2p.AsServer()))
	require.Len(t, dhtServerNodes, 1)
	dhtServerNode := dhtServerNodes[0]

	// crate other nodes running the DHT Client (mimicking the unstaked ANs)
	dhtClientNodes, _ := nodesFixture(t, ctx, sporkId, "dht_test", count-1, withDHTOptions(p2p.AsClient()))

	nodes := append(dhtServerNodes, dhtClientNodes...)
	defer stopNodes(t, nodes)

	// Step 2: Connect all nodes running a DHT client to the node running the DHT server
	// This has to be done before subscribing to any topic, otherwise the node gives up on advertising
	// its topics of interest and becomes undiscoverable by other nodes
	// (see: https://github.com/libp2p/go-libp2p-pubsub/issues/442)
	dhtServerAddr := peer.AddrInfo{ID: dhtServerNode.Host().ID(), Addrs: dhtServerNode.Host().Addrs()}
	for _, clientNode := range dhtClientNodes {
		err := clientNode.Host().Connect(ctx, dhtServerAddr)
		require.NoError(t, err)
	}

	// Step 3: Subscribe to the test topic
	// A node will receive its own message (https://github.com/libp2p/go-libp2p-pubsub/issues/65)
	// hence expect count and not count - 1 messages to be received (one by each node, including the sender)
	ch := make(chan peer.ID, count)

	msg := &message.Message{
		Payload: []byte("hello"),
	}

	data, err := msg.Marshal()
	require.NoError(t, err)

	for _, n := range nodes {
		// defines a func to read from the subscription
		subReader := func(s *pubsub.Subscription) {
			msg, err := s.Next(ctx)
			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, data, msg.Data)
			ch <- n.Host().ID()
		}

		// Subscribes to the test topic
		s, err := n.Subscribe(topic)
		require.NoError(t, err)

		// kick off the reader
		go subReader(s)

	}

	// fullyConnectedGraph checks that each node is directly connected to all the other nodes
	fullyConnectedGraph := func() bool {
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				if nodes[i].Host().Network().Connectedness(nodes[j].Host().ID()) == network.NotConnected {
					return false
				}
			}
		}
		return true
	}
	// assert that the graph is fully connected
	require.Eventually(t, fullyConnectedGraph, time.Second*5, ticksForAssertEventually, "nodes failed to discover each other")

	// Step 4: publish a message to the topic
	require.NoError(t, dhtServerNode.Publish(ctx, topic, data))

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
				if _, found := recv[n.Host().ID()]; !found {
					missing = append(missing, n.Host().ID())
				}
			}
			assert.Fail(t, "messages not received by some nodes", "%v", missing)
			break loop
		}
	}

	// Step 6: unsubscribes all nodes from the topic
	for _, n := range nodes {
		assert.NoError(t, n.UnSubscribe(topic))
	}
}
