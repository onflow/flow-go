package p2p

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

type PubSubTestSuite struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc // used to cancel the context
}

// TestLibP2PNodesTestSuite runs all the test methods in this test suit
func TestPubSubTestSuite(t *testing.T) {
	suite.Run(t, new(PubSubTestSuite))
}

// SetupTests initiates the test setups prior to each test
func (suite *PubSubTestSuite) SetupTest() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
}

// TestPubSub checks if nodes can subscribe to a topic and send and receive a message on that topic. The DHT discovery
// mechanism is used for nodes to find each other.
func (suite *PubSubTestSuite) TestPubSubWithDHTDiscovery() {
	defer suite.cancel()
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
		s, err := n.Subscribe(suite.ctx, topic)
		require.NoError(suite.T(), err)

		// kick off the reader
		go subReader(s)

	}

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
			for _, n := range dhtClientNodes {
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
func (suite *PubSubTestSuite) CreateNodes(count int, dhtServer bool) (nodes []*Node) {

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

	// creating nodes
	for i := 1; i <= count; i++ {
		_, key := generateNetworkingAndLibP2PKeys(suite.T())

		libP2PHostOptions, err := DefaultLibP2POptions("0.0.0.0:0", key, false)
		require.NoError(suite.T(), err)

		libP2PHost, err := libp2p.New(suite.ctx, libP2PHostOptions...)
		require.NoError(suite.T(), err)

		pingInfoProvider, _, _ := MockPingInfoProvider()

		var dhtDiscovery *discovery.RoutingDiscovery
		if dhtServer {
			dhtDiscovery, err = NewDHTServer(suite.ctx, libP2PHost)
			require.NoError(suite.T(), err)
		} else {
			dhtDiscovery, err = NewDHTClient(suite.ctx, libP2PHost)
			require.NoError(suite.T(), err)
		}

		psOption := pubsub.WithDiscovery(dhtDiscovery)

		options := []NodeOption{
			WithLibP2PHost(libP2PHost),
			WithDefaultPubSub(psOption),
			WithDefaultPingService(rootBlockID, pingInfoProvider),
		}

		n, err := NewLibP2PNode(flow.Identifier{}, rootBlockID, logger, options...)
		require.NoError(suite.T(), err)
		n.SetFlowProtocolStreamHandler(handlerFunc)

		require.Eventuallyf(suite.T(), func() bool {
			ip, p, err := n.GetIPPort()
			return err == nil && ip != "" && p != ""
		}, 3*time.Second, tickForAssertEventually, fmt.Sprintf("could not start node %d", i))
		nodes = append(nodes, n)
	}
	return nodes
}

// StopNodes stop all nodes in the input slice
func (suite *PubSubTestSuite) StopNodes(nodes []*Node) {
	for _, n := range nodes {
		done, err := n.Stop()
		assert.NoError(suite.T(), err)
		<-done
	}
}
