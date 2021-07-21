package p2p

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/discovery"
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

type mockDiscovery struct {
	peerLock sync.Mutex
	peers    []peer.AddrInfo
}

func (s *mockDiscovery) SetPeers(peers []peer.AddrInfo) {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	s.peers = peers
}

func (s *mockDiscovery) Advertise(_ context.Context, _ string, _ ...discovery.Option) (time.Duration, error) {
	return time.Second, nil
}

func (s *mockDiscovery) FindPeers(_ context.Context, _ string, _ ...discovery.Option) (<-chan peer.AddrInfo, error) {
	defer s.peerLock.Unlock()
	s.peerLock.Lock()
	count := len(s.peers)
	ch := make(chan peer.AddrInfo, count)
	for _, reg := range s.peers {
		ch <- reg
	}
	close(ch)
	return ch, nil
}

// TestPubSub checks if nodes can subscribe to a topic and send and receive a message
func (suite *PubSubTestSuite) TestPubSub() {
	defer suite.cancel()
	topic := flownet.Topic("testtopic/" + unittest.IdentifierFixture().String())
	count := 4
	golog.SetAllLoggers(golog.LevelError)

	// Step 1: Creates nodes
	d := &mockDiscovery{}

	nodes := suite.CreateNodes(count, d)
	defer suite.StopNodes(nodes)

	// Step 2: Subscribe to a Flow topic
	// A node will receive its own message (https://github.com/libp2p/go-libp2p-pubsub/issues/65)
	// hence expect count and not count - 1 messages to be received (one by each node, including the sender)
	ch := make(chan flow.Identifier, count)
	for _, n := range nodes {
		m := n.id
		// defines a func to read from the subscription
		subReader := func(s *pubsub.Subscription) {
			msg, err := s.Next(suite.ctx)
			require.NoError(suite.Suite.T(), err)
			require.NotNil(suite.Suite.T(), msg)
			assert.Equal(suite.Suite.T(), []byte("hello"), msg.Data)
			ch <- m
		}

		// Subscribes to the test topic
		s, err := n.Subscribe(suite.ctx, topic)
		require.NoError(suite.Suite.T(), err)

		// kick off the reader
		go subReader(s)

	}

	// Step 3: Now setup discovery to allow nodes to find each other
	var pInfos []peer.AddrInfo
	for _, n := range nodes {
		id := n.host.ID()
		addrs := n.host.Addrs()
		pInfos = append(pInfos, peer.AddrInfo{ID: id, Addrs: addrs})
	}
	// set the common discovery object shared by all nodes with the list of all peer.AddrInfos
	d.SetPeers(pInfos)

	// let the nodes discover each other
	time.Sleep(2 * time.Second)

	// Step 4: publish a message to the topic
	require.NoError(suite.Suite.T(), nodes[0].Publish(suite.ctx, topic, []byte("hello")))

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
			assert.Fail(suite.Suite.T(), " messages not received by nodes: "+strings.Join(missing.Strings(), ","))
			break
		}
	}

	// Step 6: unsubscribes all nodes from the topic
	for _, n := range nodes {
		assert.NoError(suite.Suite.T(), n.UnSubscribe(topic))
	}
}

// CreateNode creates a number of libp2pnodes equal to the count with the given callback function for stream handling
// it also asserts the correctness of nodes creations
// a single error in creating one node terminates the entire test
func (suite *PubSubTestSuite) CreateNodes(count int, d *mockDiscovery) (nodes []*Node) {
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

		noopMetrics := metrics.NewNoopCollector()

		pingInfoProvider, _, _ := MockPingInfoProvider()
		psOption := pubsub.WithDiscovery(d)
		n, err := NewLibP2PNode(logger, flow.Identifier{}, "0.0.0.0:0", NewConnManager(logger, noopMetrics), key, false, rootBlockID, pingInfoProvider, psOption)
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
		assert.NoError(suite.Suite.T(), err)
		<-done
	}
}
