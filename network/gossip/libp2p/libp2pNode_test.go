package libp2p

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/network"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/utils/unittest"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const tickForAssertEventually = 100 * time.Millisecond

var rootBlockID = unittest.IdentifierFixture().String()

type LibP2PNodeTestSuite struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc // used to cancel the context
	logger zerolog.Logger
}

// TestLibP2PNodesTestSuite runs all the test methods in this test suit
func TestLibP2PNodesTestSuite(t *testing.T) {
	suite.Run(t, new(LibP2PNodeTestSuite))
}

// SetupTests initiates the test setups prior to each test
func (l *LibP2PNodeTestSuite) SetupTest() {
	l.logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	l.ctx, l.cancel = context.WithCancel(context.Background())
	golog.SetAllLoggers(golog.LevelWarn)
}

// TestMultiAddress evaluates correct translations from
// dns and ip4 to libp2p multi-address
func (l *LibP2PNodeTestSuite) TestMultiAddress() {
	defer l.cancel()
	tt := []struct {
		address      NodeAddress
		multiaddress string
	}{
		{ // ip4 test case
			address: NodeAddress{
				Name: "ip4-node",
				IP:   "172.16.254.1",
				Port: "72",
			},
			multiaddress: "/ip4/172.16.254.1/tcp/72",
		},
		{ // dns test case
			address: NodeAddress{
				Name: "dns-node-1",
				IP:   "consensus",
				Port: "2222",
			},
			multiaddress: "/dns4/consensus/tcp/2222",
		},
		{ // dns test case
			address: NodeAddress{
				Name: "dns-node-2",
				IP:   "flow.com",
				Port: "3333",
			},
			multiaddress: "/dns4/flow.com/tcp/3333",
		},
	}

	for _, tc := range tt {
		actualAddress := MultiaddressStr(tc.address)
		assert.Equal(l.Suite.T(), tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

}

func (l *LibP2PNodeTestSuite) TestSingleNodeLifeCycle() {
	defer l.cancel()

	// creates a single
	nodes, _ := l.CreateNodes(1, nil, false)

	// stops the created node
	done, err := nodes[0].Stop()
	assert.NoError(l.Suite.T(), err)
	<-done
}

// TestGetPeerInfo evaluates the deterministic translation between the nodes address and
// their libp2p info. It generates an address, and checks whether repeated translations
// yields the same info or not.
func (l *LibP2PNodeTestSuite) TestGetPeerInfo() {
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("node%d", i)
		key, err := generateNetworkingKey(name)
		require.NoError(l.Suite.T(), err)

		// creates node-i address
		address := NodeAddress{
			Name:   name,
			IP:     "1.1.1.1",
			Port:   "0",
			PubKey: key.GetPublic(),
		}

		// translates node-i address into info
		info, err := GetPeerInfo(address)
		require.NoError(l.Suite.T(), err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := GetPeerInfo(address)
			require.NoError(l.Suite.T(), err)
			assert.True(l.Suite.T(), rinfo.String() == info.String(), "inconsistent id generated")
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func (l *LibP2PNodeTestSuite) TestAddPeers() {
	defer l.cancel()

	count := 3

	// create nodes
	nodes, addrs := l.CreateNodes(count, nil, false)
	defer l.StopNodes(nodes)

	// add the remaining nodes to the first node as its set of peers
	for _, p := range addrs[1:] {
		require.NoError(l.Suite.T(), nodes[0].AddPeer(l.ctx, p))
	}

	// Checks if all 3 nodes have been added as peers to the first node
	assert.Len(l.Suite.T(), nodes[0].libP2PHost.Peerstore().Peers(), count)

	// Checks whether the first node is connected to the rest
	for _, peer := range nodes[0].libP2PHost.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].libP2PHost.ID().String() == peer.String() {
			continue
		}
		assert.Eventuallyf(l.Suite.T(), func() bool {
			return network.Connected == nodes[0].libP2PHost.Network().Connectedness(peer)
		}, 2*time.Second, tickForAssertEventually, fmt.Sprintf(" first node is not connected to %s", peer.String()))
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func (l *LibP2PNodeTestSuite) TestRemovePeers() {
	defer l.cancel()

	count := 3

	// create nodes
	nodes, addrs := l.CreateNodes(count, nil, false)
	defer l.StopNodes(nodes)

	// add nodes two and three to the first node as its peers
	for _, p := range addrs[1:] {
		require.NoError(l.Suite.T(), nodes[0].AddPeer(l.ctx, p))
	}

	// check if all 3 nodes have been added as peers to the first node
	assert.Len(l.Suite.T(), nodes[0].libP2PHost.Peerstore().Peers(), count)

	// check whether the first node is connected to the rest
	for _, peer := range nodes[0].libP2PHost.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].libP2PHost.ID().String() == peer.String() {
			continue
		}
		assert.Eventually(l.Suite.T(), func() bool {
			return network.Connected == nodes[0].libP2PHost.Network().Connectedness(peer)
		}, 2*time.Second, tickForAssertEventually)
	}

	// disconnect from each peer and assert that the connection no longer exists
	for _, p := range addrs[1:] {
		require.NoError(l.Suite.T(), nodes[0].RemovePeer(l.ctx, p))
		pInfo, err := GetPeerInfo(p)
		assert.NoError(l.Suite.T(), err)
		assert.Equal(l.Suite.T(), network.NotConnected, nodes[0].libP2PHost.Network().Connectedness(pInfo.ID))
	}
}

// TestCreateStreams checks if a new streams is created each time when CreateStream is called and an existing stream is not reused
func (l *LibP2PNodeTestSuite) TestCreateStream() {
	defer l.cancel()
	count := 2

	// Creates nodes
	nodes, addrs := l.CreateNodes(count, nil, false)
	defer l.StopNodes(nodes)

	address2 := addrs[1]

	flowProtocolID := generateProtocolID(rootBlockID)
	// Assert that there is no outbound stream to the target yet
	require.Equal(l.T(), 0, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), flowProtocolID, network.DirOutbound))

	// Now attempt to create another 100 outbound stream to the same destination by calling CreateStream
	var streams []network.Stream
	for i := 0; i < 100; i++ {
		anotherStream, err := nodes[0].CreateStream(context.Background(), address2)
		// Assert that a stream was returned without error
		require.NoError(l.T(), err)
		require.NotNil(l.T(), anotherStream)
		// assert that the stream count within libp2p incremented (a new stream was created)
		require.Equal(l.T(), i+1, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), flowProtocolID, network.DirOutbound))
		// assert that the same connection is reused
		require.Len(l.T(), nodes[0].libP2PHost.Network().Conns(), 1)
		streams = append(streams, anotherStream)
	}

	// reverse loop to close all the streams
	for i := 99; i >= 0; i-- {
		s := streams[i]
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			helpers.FullClose(s)
			wg.Done()
		}()
		wg.Wait()
		// assert that the stream count within libp2p decremented
		require.Equal(l.T(), i, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), flowProtocolID, network.DirOutbound))
	}
}

// TestOneToOneComm sends a message from node 1 to node 2 and then from node 2 to node 1
func (l *LibP2PNodeTestSuite) TestOneToOneComm() {
	defer l.cancel()
	count := 2
	ch := make(chan string, count)

	// Create the handler function
	handler := func(s network.Stream) {
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		str, err := rw.ReadString('\n')
		assert.NoError(l.T(), err)
		ch <- str
	}

	// Creates peers
	peers, addrs := l.CreateNodes(count, handler, false)
	defer l.StopNodes(peers)
	require.Len(l.T(), addrs, count)

	addr1 := addrs[0]
	addr2 := addrs[1]

	// Create stream from node 1 to node 2
	s1, err := peers[0].CreateStream(context.Background(), addr2)
	assert.NoError(l.T(), err)
	rw := bufio.NewReadWriter(bufio.NewReader(s1), bufio.NewWriter(s1))

	// Send message from node 1 to 2
	msg := "hello\n"
	_, err = rw.WriteString(msg)
	assert.NoError(l.T(), err)

	// Flush the stream
	assert.NoError(l.T(), rw.Flush())

	// Wait for the message to be received
	select {
	case rcv := <-ch:
		require.Equal(l.T(), msg, rcv)
	case <-time.After(1 * time.Second):
		assert.Fail(l.T(), "message not received")
	}

	// Create stream from node 2 to node 1
	s2, err := peers[1].CreateStream(context.Background(), addr1)
	assert.NoError(l.T(), err)
	rw = bufio.NewReadWriter(bufio.NewReader(s2), bufio.NewWriter(s2))

	// Send message from node 2 to 1
	msg = "hey\n"
	_, err = rw.WriteString(msg)
	assert.NoError(l.T(), err)

	// Flush the stream
	assert.NoError(l.T(), rw.Flush())

	select {
	case rcv := <-ch:
		require.Equal(l.T(), msg, rcv)
	case <-time.After(3 * time.Second):
		assert.Fail(l.T(), "message not received")
	}
}

// TestStreamClosing tests 1-1 communication with streams closed using libp2p2 handler.FullClose
func (l *LibP2PNodeTestSuite) TestStreamClosing() {
	defer l.cancel()
	count := 10
	ch := make(chan string, count)
	defer close(ch)
	done := make(chan struct{})
	defer close(done)

	// Create the handler function
	handler := func(s network.Stream) {
		go func(s network.Stream) {
			rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
			for {
				str, err := rw.ReadString('\n')
				if err != nil {
					if errors.Is(err, io.EOF) {
						s.Close()
						return
					}
					assert.Fail(l.T(), fmt.Sprintf("received error %v", err))
					err = s.Reset()
					assert.NoError(l.T(), err)
					return
				}
				select {
				case <-done:
					return
				default:
					ch <- str
				}
			}
		}(s)
	}

	// Creates peers
	peers, addrs := l.CreateNodes(2, handler, false)
	defer l.StopNodes(peers)

	for i := 0; i < count; i++ {
		// Create stream from node 1 to node 2 (reuse if one already exists)
		s, err := peers[0].CreateStream(context.Background(), addrs[1])
		assert.NoError(l.T(), err)
		w := bufio.NewWriter(s)

		// Send message from node 1 to 2
		msg := fmt.Sprintf("hello%d\n", i)
		_, err = w.WriteString(msg)
		assert.NoError(l.T(), err)

		// Flush the stream
		assert.NoError(l.T(), w.Flush())
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func(s network.Stream) {
			defer wg.Done()
			// close the stream
			err := helpers.FullClose(s)
			require.NoError(l.T(), err)
		}(s)
		// wait for stream to be closed
		wg.Wait()

		// wait for the message to be received
		select {
		case rcv := <-ch:
			require.Equal(l.T(), msg, rcv)
		case <-time.After(10 * time.Second):
			require.Fail(l.T(), fmt.Sprintf("message %s not received", msg))
			break
		}
	}
}

// TestPing tests that a node can ping another node
func (l *LibP2PNodeTestSuite) TestPing() {
	defer l.cancel()

	// creates two nodes
	nodes, nodeAddr := l.CreateNodes(2, nil, false)
	defer l.StopNodes(nodes)

	node1 := nodes[0]
	node2 := nodes[1]
	node1Addr := nodeAddr[0]
	node2Addr := nodeAddr[1]

	// test node1 can ping node 2
	_, err := node1.Ping(l.ctx, node2Addr)
	require.NoError(l.T(), err)

	// test node 2 can ping node 1
	_, err = node2.Ping(l.ctx, node1Addr)
	require.NoError(l.T(), err)
}

// TestConnectionGating tests node allow listing by peer.ID
func (l *LibP2PNodeTestSuite) TestConnectionGating() {
	defer l.cancel()

	// create 2 nodes
	nodes, nodeAddrs := l.CreateNodes(2, nil, true)

	node1 := nodes[0]
	node1Addr := nodeAddrs[0]
	defer l.StopNode(node1)

	node2 := nodes[1]
	node2Addr := nodeAddrs[1]
	defer l.StopNode(node2)

	requireError := func(err error) {
		require.Error(l.T(), err)
		require.True(l.T(), errors.Is(err, swarm.ErrGaterDisallowedConnection))
	}

	l.Run("outbound connection to a not-allowed node is rejected", func() {
		// node1 and node2 both have no allowListed peers
		_, err := node1.CreateStream(l.ctx, node2Addr)
		requireError(err)
		_, err = node2.CreateStream(l.ctx, node1Addr)
		requireError(err)
	})

	l.Run("inbound connection from an allowed node is rejected", func() {

		// node1 allowlists node2 but node2 does not allowlists node1
		err := node1.UpdateAllowlist([]NodeAddress{node2Addr}...)
		require.NoError(l.T(), err)

		// node1 attempts to connect to node2
		// node2 should reject the inbound connection
		_, err = node1.CreateStream(l.ctx, node2Addr)
		require.Error(l.T(), err)
	})

	l.Run("outbound connection to an approved node is allowed", func() {

		// node1 allowlists node2
		err := node1.UpdateAllowlist([]NodeAddress{node2Addr}...)
		require.NoError(l.T(), err)
		// node2 allowlists node1
		err = node2.UpdateAllowlist([]NodeAddress{node1Addr}...)
		require.NoError(l.T(), err)

		// node1 should be allowed to connect to node2
		_, err = node1.CreateStream(l.ctx, node2Addr)
		require.NoError(l.T(), err)
		// node2 should be allowed to connect to node1
		_, err = node2.CreateStream(l.ctx, node1Addr)
		require.NoError(l.T(), err)
	})
}

// CreateNodes creates a number of libp2pnodes equal to the count with the given callback function for stream handling
// it also asserts the correctness of nodes creations
// a single error in creating one node terminates the entire test
func (l *LibP2PNodeTestSuite) CreateNodes(count int, handler network.StreamHandler, allowList bool) ([]*P2PNode, []NodeAddress) {
	// keeps track of errors on creating a node
	var err error
	var nodes []*P2PNode

	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			l.StopNodes(nodes)
		}
	}()

	// creating nodes
	var nodeAddrs []NodeAddress
	for i := 0; i < count; i++ {

		name := fmt.Sprintf("node%d", i+1)
		pkey, err := generateNetworkingKey(name)
		require.NoError(l.Suite.T(), err)

		// create a node on localhost with a random port assigned by the OS
		n, nodeID := l.CreateNode(name, pkey, "0.0.0.0", "0", rootBlockID, handler, allowList)
		nodes = append(nodes, n)
		nodeAddrs = append(nodeAddrs, nodeID)
	}
	return nodes, nodeAddrs
}

func (l *LibP2PNodeTestSuite) CreateNode(name string, key crypto.PrivKey, ip string, port string, rootID string,
	handler network.StreamHandler, allowList bool) (*P2PNode, NodeAddress) {
	n := &P2PNode{}
	nodeID := NodeAddress{
		Name:   name,
		IP:     ip,
		Port:   port,
		PubKey: key.GetPublic(),
	}

	var handlerFunc network.StreamHandler
	if handler != nil {
		// use the callback that has been passed in
		handlerFunc = handler
	} else {
		// use a default call back
		handlerFunc = func(network.Stream) {}
	}

	err := n.Start(l.ctx, nodeID, l.logger, key, handlerFunc, rootID, allowList, nil)
	require.NoError(l.T(), err)
	require.Eventuallyf(l.T(), func() bool {
		ip, p, err := n.GetIPPort()
		return err == nil && ip != "" && p != ""
	}, 3*time.Second, tickForAssertEventually, fmt.Sprintf("could not start node %s", name))

	// get the actual IP and port that have been assigned by the subsystem
	nodeID.IP, nodeID.Port, err = n.GetIPPort()
	require.NoError(l.T(), err)

	return n, nodeID
}

// StopNodes stop all nodes in the input slice
func (l *LibP2PNodeTestSuite) StopNodes(nodes []*P2PNode) {
	for _, n := range nodes {
		l.StopNode(n)
	}
}

func (l *LibP2PNodeTestSuite) StopNode(node *P2PNode) {
	done, err := node.Stop()
	assert.NoError(l.Suite.T(), err)
	<-done
}

// generateNetworkingKey generates a ECDSA key pair using the given seed
func generateNetworkingKey(seed string) (crypto.PrivKey, error) {
	seedB := make([]byte, 100)
	copy(seedB, seed)
	var r io.Reader = bytes.NewReader(seedB)
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.ECDSA, 0, r)
	return prvKey, err
}
