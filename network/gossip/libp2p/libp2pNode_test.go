package libp2p

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	gologging "github.com/whyrusleeping/go-logging"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const tickForAssertEventually = 100 * time.Millisecond

type LibP2PNodeTestSuite struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc // used to cancel the context
}

// TestLibP2PNodesTestSuite runs all the test methods in this test suit
func TestLibP2PNodesTestSuite(t *testing.T) {
	suite.Run(t, new(LibP2PNodeTestSuite))
}

// SetupTests initiates the test setups prior to each test
func (l *LibP2PNodeTestSuite) SetupTest() {
	l.ctx, l.cancel = context.WithCancel(context.Background())
}

func (l *LibP2PNodeTestSuite) TestSingleNodeLifeCycle() {
	defer l.cancel()

	// creates a single
	nodes := l.CreateNodes(1)

	// stops the created node
	assert.NoError(l.Suite.T(), nodes[0].Stop())
}

// TestGetPeerInfo evaluates the deterministic translation between the nodes address and
// their libp2p info. It generates an address, and checks whether repeated translations
// yields the same info or not.
func (l *LibP2PNodeTestSuite) TestGetPeerInfo() {
	for i := 0; i < 10; i++ {
		// creates node-i address
		address := NodeAddress{
			Name: fmt.Sprintf("node%d", i),
			IP:   "1.1.1.1",
			Port: "0",
		}

		// translates node-i address into info
		info, err := GetPeerInfo(address)
		require.NoError(l.Suite.T(), err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := GetPeerInfo(address)
			require.NoError(l.Suite.T(), err)
			assert.True(l.Suite.T(), rinfo.String() == info.String(), fmt.Sprintf("inconsistent id generated"))
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func (l *LibP2PNodeTestSuite) TestAddPeers() {
	defer l.cancel()

	// count value of 10 runs into this issue on localhost
	// https://github.com/libp2p/go-libp2p-pubsub/issues/96
	// since localhost connection have short deadlines
	count := 3

	// Creates nodes
	nodes := l.CreateNodes(count)
	defer l.StopNodes(nodes)

	ids := make([]NodeAddress, 0)
	// Get actual IP and Port numbers on which the nodes were started
	for _, n := range nodes[1:] {
		ip, p := n.GetIPPort()
		ids = append(ids, NodeAddress{Name: n.name, IP: ip, Port: p})
	}

	// Adds the remaining nodes to the first node as its set of peers
	require.NoError(l.Suite.T(), nodes[0].AddPeers(l.ctx, ids))
	actual := nodes[0].libP2PHost.Peerstore().Peers().Len()

	// Checks if all 9 nodes have been added as peers to the first node
	assert.True(l.Suite.T(), count == actual, "inconsistent peers number expected: %d, found: %d", count, actual)

	// Checks whether the first node is connected to the rest
	for _, peer := range nodes[0].libP2PHost.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].libP2PHost.ID().String() == peer.String() {
			continue
		}
		assert.Eventuallyf(l.Suite.T(), func() bool {
			return network.Connected == nodes[0].libP2PHost.Network().Connectedness(peer)
		}, 3*time.Second, tickForAssertEventually, fmt.Sprintf(" first node is not connected to %s", peer.String()))
	}
}

// TestPubSub checks if nodes can subscribe to a topic and send and receive a message
func (l *LibP2PNodeTestSuite) TestPubSub() {
	defer l.cancel()
	count := 5
	golog.SetAllLoggers(gologging.INFO)

	// Step 1: Creates nodes
	nodes := l.CreateNodes(count)
	defer l.StopNodes(nodes)

	// Step 2: Subscribes to a Flow topic
	// A node will receive its own message (https://github.com/libp2p/go-libp2p-pubsub/issues/65)
	// hence expect count and not count - 1 messages to be received (one by each node, including the sender)
	ch := make(chan string, count)
	for _, n := range nodes {
		m := n.name
		// Defines a callback to be called whenever a message is received
		callback := func(msg []byte) {
			assert.Equal(l.Suite.T(), []byte("hello"), msg)
			ch <- m
		}

		// Subscribes to "Consensus" topic with the defined callback
		require.NoError(l.Suite.T(), n.Subscribe(l.ctx, Consensus, callback))
	}

	// Step 3: Connects each node i to its subsequent node i+1 in a chain
	for i := 0; i < count-1; i++ {
		// defines this node on the chain
		this := nodes[i]

		// defines next node to this on the chain
		next := nodes[i+1]
		nextIP, nextPort := next.GetIPPort()
		nextAddr := &NodeAddress{
			Name: next.name,
			IP:   nextIP,
			Port: nextPort,
		}

		// adds next node as the peer to this node and verifies their connection
		require.NoError(l.Suite.T(), this.AddPeers(l.ctx, []NodeAddress{*nextAddr}))
		assert.Eventuallyf(l.Suite.T(), func() bool {
			return network.Connected == this.libP2PHost.Network().Connectedness(next.libP2PHost.ID())
		}, 3*time.Second, tickForAssertEventually, fmt.Sprintf(" %s not connected with %s", this.name, next.name))

		// Number of connected peers on the chain should be always 2 except for the
		// first and last nodes that should be one
		peerNum := 2
		if i == 0 || i == count {
			peerNum = 1
		}
		assert.Equal(l.Suite.T(), peerNum, len(this.ps.ListPeers(string(Consensus))))
	}

	// Step 4: Waits for nodes to heartbeat each other
	time.Sleep(2 * time.Second)

	// Step 5: Publish a message from the first node on the chain
	// and verify all nodes get it.
	// All nodes including node 0 - the sender, should receive it
	require.NoError(l.Suite.T(), nodes[0].Publish(l.ctx, Consensus, []byte("hello")))

	// A hash set to keep track of the nodes who received the message
	recv := make(map[string]bool, count)
	for i := 0; i < count; i++ {
		select {
		case res := <-ch:
			recv[res] = true
		case <-time.After(10 * time.Second):
			missing := make([]string, 0)
			for _, n := range nodes {
				if _, found := recv[n.name]; !found {
					missing = append(missing, n.name)
				}
			}
			assert.Fail(l.Suite.T(), " messages not received by nodes: "+strings.Join(missing, ", "))
			break
		}
	}

	// Step 6: Unsubscribes all nodes from the topic
	for _, n := range nodes {
		assert.NoError(l.Suite.T(), n.UnSubscribe(Consensus))
	}
}

// CreateNodes creates a number of libp2pnodes equal to the count
// it also asserts the correctness of nodes creations
// a single error in creating one node terminates the entire test
func (l *LibP2PNodeTestSuite) CreateNodes(count int) (nodes []*P2PNode) {
	// keeps track of errors on creating a node
	var err error
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			l.StopNodes(nodes)
		}
	}()

	// creating nodes
	for i := 1; i <= count; i++ {
		n := &P2PNode{}
		nodeID := NodeAddress{
			Name: fmt.Sprintf("node%d", i),
			IP:   "0.0.0.0", // localhost
			Port: "0",       // random Port number
		}
		err := n.Start(l.ctx, nodeID, logger, func(stream network.Stream) {
		})
		require.NoError(l.Suite.T(), err)
		require.Eventuallyf(l.Suite.T(), func() bool {
			ip, p := n.GetIPPort()
			return ip != "" && p != ""
		}, 3*time.Second, tickForAssertEventually, fmt.Sprintf("could not start node %d", i))
		nodes = append(nodes, n)
	}
	return nodes
}

// StopNodes stop all nodes in the input slice
func (l *LibP2PNodeTestSuite) StopNodes(nodes []*P2PNode) {
	if nodes != nil {
		for _, n := range nodes {
			assert.NoError(l.Suite.T(), n.Stop())
		}
	}
}
