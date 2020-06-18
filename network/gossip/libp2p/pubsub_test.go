package libp2p

import (
	"context"
	"fmt"
	"os"
	"strings"
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
func (p *PubSubTestSuite) SetupTest() {
	p.ctx, p.cancel = context.WithCancel(context.Background())
}

type mockDiscovery struct {
	peers []peer.AddrInfo
}

func (s *mockDiscovery) SetPeers(peers []peer.AddrInfo) {
	s.peers = peers
}

func (s *mockDiscovery) Advertise(_ context.Context, _ string, _ ...discovery.Option) (time.Duration, error) {
	return time.Second, nil
}

func (s *mockDiscovery) FindPeers(_ context.Context, _ string, _ ...discovery.Option) (<-chan peer.AddrInfo, error) {
	count := len(s.peers)
	ch := make(chan peer.AddrInfo, count)
	for _, reg := range s.peers {
		ch <- reg
	}
	close(ch)
	return ch, nil
}

// TestPubSub checks if nodes can subscribe to a topic and send and receive a message
func (p *PubSubTestSuite) TestPubSub() {
	defer p.cancel()
	topic := "testtopic"
	count := 4
	golog.SetAllLoggers(golog.LevelError)

	// Step 1: Creates nodes
	d := &mockDiscovery{}

	nodes := p.CreateNodes(count, d)
	defer p.StopNodes(nodes)

	// Step 2: Subscribe to a Flow topic
	// A node will receive its own message (https://github.com/libp2p/go-libp2p-pubsub/issues/65)
	// hence expect count and not count - 1 messages to be received (one by each node, including the sender)
	ch := make(chan string, count)
	for _, n := range nodes {
		m := n.name
		// defines a func to read from the subscription
		subReader := func(s *pubsub.Subscription) {
			msg, err := s.Next(p.ctx)
			require.NoError(p.Suite.T(), err)
			require.NotNil(p.Suite.T(), msg)
			assert.Equal(p.Suite.T(), []byte("hello"), msg.Data)
			ch <- m
		}

		// Subscribes to the test topic
		s, err := n.Subscribe(p.ctx, topic)
		require.NoError(p.Suite.T(), err)

		// kick off the reader
		go subReader(s)

	}

	// Step 3: Now setup discovery to allow nodes to find each other
	var pInfos []peer.AddrInfo
	for _, n := range nodes {
		id := n.libP2PHost.ID()
		addrs := n.libP2PHost.Addrs()
		pInfos = append(pInfos, peer.AddrInfo{ID: id, Addrs: addrs})
	}
	// set the common discovery object shared by all nodes with the list of all peer.AddrInfos
	d.SetPeers(pInfos)

	// let the nodes discover each other
	time.Sleep(2 * time.Second)

	// Step 4: publish a message to the topic
	require.NoError(p.Suite.T(), nodes[0].Publish(p.ctx, topic, []byte("hello")))

	// Step 5: By now, all peers would have been discovered and the message should have been successfully published
	// A hash set to keep track of the nodes who received the message
	recv := make(map[string]bool, count)
	for i := 0; i < count; i++ {
		select {
		case res := <-ch:
			recv[res] = true
		case <-time.After(3 * time.Second):
			missing := make([]string, 0)
			for _, n := range nodes {
				if _, found := recv[n.name]; !found {
					missing = append(missing, n.name)
				}
			}
			assert.Fail(p.Suite.T(), " messages not received by nodes: "+strings.Join(missing, ", "))
			break
		}
	}

	// Step 6: unsubscribes all nodes from the topic
	for _, n := range nodes {
		assert.NoError(p.Suite.T(), n.UnSubscribe(topic))
	}
}

// CreateNode creates a number of libp2pnodes equal to the count with the given callback function for stream handling
// it also asserts the correctness of nodes creations
// a single error in creating one node terminates the entire test
func (psts *PubSubTestSuite) CreateNodes(count int, d *mockDiscovery) (nodes []*P2PNode) {
	// keeps track of errors on creating a node
	var err error
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			psts.StopNodes(nodes)
		}
	}()

	handlerFunc := func(network.Stream) {}

	// creating nodes
	for i := 1; i <= count; i++ {

		name := fmt.Sprintf("node%d", i)
		pkey, err := generateNetworkingKey(name)
		require.NoError(psts.Suite.T(), err)

		n := &P2PNode{}
		nodeID := NodeAddress{
			Name:   name,
			IP:     "0.0.0.0",        // localhost
			Port:   "0",              // random Port number
			PubKey: pkey.GetPublic(), // the networking public key
		}

		psOption := pubsub.WithDiscovery(d)
		err = n.Start(psts.ctx, nodeID, logger, pkey, handlerFunc, psOption)
		require.NoError(psts.Suite.T(), err)
		require.Eventuallyf(psts.Suite.T(), func() bool {
			ip, p := n.GetIPPort()
			return ip != "" && p != ""
		}, 3*time.Second, tickForAssertEventually, fmt.Sprintf("could not start node %d", i))
		nodes = append(nodes, n)
	}
	return nodes
}

// StopNodes stop all nodes in the input slice
func (psts *PubSubTestSuite) StopNodes(nodes []*P2PNode) {
	for _, n := range nodes {
		done, err := n.Stop()
		assert.NoError(psts.Suite.T(), err)
		<-done
	}
}
