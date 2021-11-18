package p2p

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/utils/unittest"
)

type DHTTestSuite struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc // used to cancel the context
}

// TestDHTTestSuite test the libp2p pubsub with DHT for discovery
func TestDHTTestSuite(t *testing.T) {
	suite.Run(t, new(DHTTestSuite))
}

// SetupTests initiates the test setups prior to each test
func (suite *DHTTestSuite) SetupTest() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
}

func (suite *DHTTestSuite) TearDownTest() {
	suite.cancel()
}

// TestFindPeerWithDHT checks that if a node is configured to participate in the DHT, it is
// able to create new streams with peers even without knowing their address info beforehand.
func (suite *DHTTestSuite) TestFindPeerWithDHT() {
	count := 10
	golog.SetAllLoggers(golog.LevelFatal) // change this to Debug if libp2p logs are needed

	dhtServerNodes := suite.CreateNodes(2, true)
	require.Len(suite.T(), dhtServerNodes, 2)

	dhtClientNodes := suite.CreateNodes(count-2, false)

	nodes := append(dhtServerNodes, dhtClientNodes...)
	defer suite.StopNodes(nodes)

	getDhtServerAddr := func(i uint) peer.AddrInfo {
		return peer.AddrInfo{ID: dhtServerNodes[i].host.ID(), Addrs: dhtServerNodes[i].host.Addrs()}
	}

	// connect even numbered clients to the first DHT server, and odd number clients to the second
	for i, clientNode := range dhtClientNodes {
		err := clientNode.host.Connect(suite.ctx, getDhtServerAddr(uint(i%2)))
		require.NoError(suite.T(), err)
	}

	// wait for clients to connect to DHT servers and update their routing tables
	require.Eventually(suite.T(), func() bool {
		for i, clientNode := range dhtClientNodes {
			if clientNode.dht.RoutingTable().Find(getDhtServerAddr(uint(i%2)).ID) == "" {
				return false
			}
		}
		return true
	}, time.Second*5, tickForAssertEventually, "nodes failed to connect")

	// connect the two DHT servers to each other
	err := dhtServerNodes[0].host.Connect(suite.ctx, getDhtServerAddr(1))
	require.NoError(suite.T(), err)

	// wait for the first server to connect to the second and update its routing table
	require.Eventually(suite.T(), func() bool {
		return dhtServerNodes[0].dht.RoutingTable().Find(getDhtServerAddr(1).ID) != ""
	}, time.Second*5, tickForAssertEventually, "dht servers failed to connect")

	// check that all even numbered clients can create streams with all odd numbered clients
	for i := 0; i < len(dhtClientNodes); i += 2 {
		for j := 1; j < len(dhtClientNodes); j += 2 {
			// client i should not yet know the address of client j, but we clear any addresses
			// here just in case.
			dhtClientNodes[i].host.Peerstore().ClearAddrs(dhtClientNodes[j].host.ID())

			// Try to create a stream from client i to client j. This should resort to a DHT
			// lookup since client i does not know client j's address.
			unittest.RequireReturnsBefore(suite.T(), func() {
				_, err = dhtClientNodes[i].CreateStream(suite.ctx, dhtClientNodes[j].host.ID())
				require.NoError(suite.T(), err)
			}, 1*time.Second, "could not create stream on time")
		}
	}
}

// TestPubSub checks if nodes can subscribe to a topic and send and receive a message on that topic. The DHT discovery
// mechanism is used for nodes to find each other.
func (suite *DHTTestSuite) TestPubSubWithDHTDiscovery() {
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

	// create one node running the DHT Server (mimicking the staked AN)
	dhtServerNodes := suite.CreateNodes(1, true)
	require.Len(suite.T(), dhtServerNodes, 1)
	dhtServerNode := dhtServerNodes[0]

	// crate other nodes running the DHT Client (mimicking the unstaked ANs)
	dhtClientNodes := suite.CreateNodes(count-1, false)

	nodes := append(dhtServerNodes, dhtClientNodes...)
	defer suite.StopNodes(nodes)

	// Step 2: Connect all nodes running a DHT client to the node running the DHT server
	// This has to be done before subscribing to any topic, otherwise the node gives up on advertising
	// its topics of interest and becomes undiscoverable by other nodes
	// (see: https://github.com/libp2p/go-libp2p-pubsub/issues/442)
	dhtServerAddr := peer.AddrInfo{ID: dhtServerNode.host.ID(), Addrs: dhtServerNode.host.Addrs()}
	for _, clientNode := range dhtClientNodes {
		err := clientNode.host.Connect(suite.ctx, dhtServerAddr)
		require.NoError(suite.T(), err)
	}

	// Step 3: Subscribe to the test topic
	// A node will receive its own message (https://github.com/libp2p/go-libp2p-pubsub/issues/65)
	// hence expect count and not count - 1 messages to be received (one by each node, including the sender)
	ch := make(chan flow.Identifier, count)
	for _, n := range nodes {
		m := n.id
		// defines a func to read from the subscription
		subReader := func(s *pubsub.Subscription) {
			msg, err := s.Next(suite.ctx)
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), msg)
			assert.Equal(suite.T(), []byte("hello"), msg.Data)
			ch <- m
		}

		// Subscribes to the test topic
		s, err := n.Subscribe(topic)
		require.NoError(suite.T(), err)

		// kick off the reader
		go subReader(s)

	}

	// fullyConnectedGraph checks that each node is directly connected to all the other nodes
	fullyConnectedGraph := func() bool {
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				if nodes[i].host.Network().Connectedness(nodes[j].host.ID()) == network.NotConnected {
					return false
				}
			}
		}
		return true
	}
	// assert that the graph is fully connected
	require.Eventually(suite.T(), fullyConnectedGraph, time.Second*5, tickForAssertEventually, "nodes failed to discover each other")

	// Step 4: publish a message to the topic
	require.NoError(suite.T(), dhtServerNode.Publish(suite.ctx, topic, []byte("hello")))

	// Step 5: By now, all peers would have been discovered and the message should have been successfully published
	// A hash set to keep track of the nodes who received the message
	recv := make(map[flow.Identifier]bool, count)
	for i := 0; i < count; i++ {
		select {
		case res := <-ch:
			recv[res] = true
		case <-time.After(3 * time.Second):
			var missing flow.IdentifierList
			for _, n := range nodes {
				if _, found := recv[n.id]; !found {
					missing = append(missing, n.id)
				}
			}
			assert.Fail(suite.T(), " messages not received by nodes: "+strings.Join(missing.Strings(), ","))
			break
		}
	}

	// Step 6: unsubscribes all nodes from the topic
	for _, n := range nodes {
		assert.NoError(suite.T(), n.UnSubscribe(topic))
	}
}

// CreateNode creates the given number of libp2pnodes
// if dhtServer is true, the DHTServer is used as for Discovery else DHTClient
func (suite *DHTTestSuite) CreateNodes(count int, dhtServer bool) (nodes []*Node) {
	// keeps track of errors on creating a node
	var err error
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			suite.StopNodes(nodes)
		}
	}()

	handlerFunc := func(network.Stream) {}
	rootBlockId := unittest.IdentifierFixture()

	// creating nodes
	for i := 1; i <= count; i++ {
		key := generateNetworkingKey(suite.T())
		noopMetrics := metrics.NewNoopCollector()

		connManager := NewConnManager(logger, noopMetrics)

		pingInfoProvider, _, _, _ := mockPingInfoProvider()

		resolver := dns.NewResolver(noopMetrics)

		n, err := NewDefaultLibP2PNodeBuilder(flow.Identifier{}, "0.0.0.0:0", key).
			SetRootBlockID(rootBlockId).
			SetConnectionManager(connManager).
			SetDHTOptions(AsServer(dhtServer)).
			SetPingInfoProvider(pingInfoProvider).
			SetResolver(resolver).
			SetLogger(logger).
			SetTopicValidation(false).
			Build(suite.ctx)
		require.NoError(suite.T(), err)

		err = n.WithDefaultUnicastProtocol(handlerFunc, nil)
		require.NoError(suite.T(), err)

		nodes = append(nodes, n)
	}
	return nodes
}

// stopNodes stop all nodes in the input slice
func (suite *DHTTestSuite) StopNodes(nodes []*Node) {
	for _, n := range nodes {
		done, err := n.Stop()
		assert.NoError(suite.T(), err)
		<-done
	}
}
