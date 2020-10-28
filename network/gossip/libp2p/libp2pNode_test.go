package libp2p

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	addrutil "github.com/libp2p/go-addr-util"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/network"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
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
func (suite *LibP2PNodeTestSuite) SetupTest() {
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	// golog.SetAllLoggers(golog.LevelError)
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
}

func (suite *LibP2PNodeTestSuite) TearDownTest() {
	suite.cancel()
}

// TestMultiAddress evaluates correct translations from
// dns and ip4 to libp2p multi-address
func (suite *LibP2PNodeTestSuite) TestMultiAddress() {
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
		assert.Equal(suite.T(), tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

}

func (suite *LibP2PNodeTestSuite) TestSingleNodeLifeCycle() {
	// creates a single
	nodes, _ := suite.CreateNodes(1, nil, false)

	// stops the created node
	done, err := nodes[0].Stop()
	assert.NoError(suite.T(), err)
	<-done
}

// TestGetPeerInfo evaluates the deterministic translation between the nodes address and
// their libp2p info. It generates an address, and checks whether repeated translations
// yields the same info or not.
func (suite *LibP2PNodeTestSuite) TestGetPeerInfo() {
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("node%d", i)
		key, err := generateNetworkingKey(name)
		require.NoError(suite.T(), err)

		// creates node-i address
		address := NodeAddress{
			Name:   name,
			IP:     "1.1.1.1",
			Port:   "0",
			PubKey: key.GetPublic(),
		}

		// translates node-i address into info
		info, err := GetPeerInfo(address)
		require.NoError(suite.T(), err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := GetPeerInfo(address)
			require.NoError(suite.T(), err)
			assert.True(suite.T(), rinfo.String() == info.String(), "inconsistent id generated")
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func (suite *LibP2PNodeTestSuite) TestAddPeers() {

	count := 3

	// create nodes
	nodes, addrs := suite.CreateNodes(count, nil, false)
	defer suite.StopNodes(nodes)

	// add the remaining nodes to the first node as its set of peers
	for _, p := range addrs[1:] {
		require.NoError(suite.T(), nodes[0].AddPeer(suite.ctx, p))
	}

	// Checks if all 3 nodes have been added as peers to the first node
	assert.Len(suite.T(), nodes[0].libP2PHost.Peerstore().Peers(), count)

	// Checks whether the first node is connected to the rest
	for _, peer := range nodes[0].libP2PHost.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].libP2PHost.ID().String() == peer.String() {
			continue
		}
		assert.Eventuallyf(suite.T(), func() bool {
			return network.Connected == nodes[0].libP2PHost.Network().Connectedness(peer)
		}, 2*time.Second, tickForAssertEventually, fmt.Sprintf(" first node is not connected to %s", peer.String()))
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func (suite *LibP2PNodeTestSuite) TestRemovePeers() {

	count := 3

	// create nodes
	nodes, addrs := suite.CreateNodes(count, nil, false)
	defer suite.StopNodes(nodes)

	// add nodes two and three to the first node as its peers
	for _, p := range addrs[1:] {
		require.NoError(suite.T(), nodes[0].AddPeer(suite.ctx, p))
	}

	// check if all 3 nodes have been added as peers to the first node
	assert.Len(suite.T(), nodes[0].libP2PHost.Peerstore().Peers(), count)

	// check whether the first node is connected to the rest
	for _, peer := range nodes[0].libP2PHost.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].libP2PHost.ID().String() == peer.String() {
			continue
		}
		assert.Eventually(suite.T(), func() bool {
			return network.Connected == nodes[0].libP2PHost.Network().Connectedness(peer)
		}, 2*time.Second, tickForAssertEventually)
	}

	// disconnect from each peer and assert that the connection no longer exists
	for _, p := range addrs[1:] {
		require.NoError(suite.T(), nodes[0].RemovePeer(suite.ctx, p))
		pInfo, err := GetPeerInfo(p)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), network.NotConnected, nodes[0].libP2PHost.Network().Connectedness(pInfo.ID))
	}
}

// TestCreateStreams checks if a new streams is created each time when CreateStream is called and an existing stream is not reused
func (suite *LibP2PNodeTestSuite) TestCreateStream() {

	count := 2

	// Creates nodes
	nodes, addrs := suite.CreateNodes(count, nil, false)
	defer suite.StopNodes(nodes)

	address2 := addrs[1]

	flowProtocolID := generateProtocolID(rootBlockID)
	// Assert that there is no outbound stream to the target yet
	require.Equal(suite.T(), 0, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), flowProtocolID, network.DirOutbound))

	// Now attempt to create another 100 outbound stream to the same destination by calling CreateStream
	var streams []network.Stream
	for i := 0; i < 100; i++ {
		anotherStream, err := nodes[0].CreateStream(context.Background(), address2)
		// Assert that a stream was returned without error
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), anotherStream)
		// assert that the stream count within libp2p incremented (a new stream was created)
		require.Equal(suite.T(), i+1, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), flowProtocolID, network.DirOutbound))
		// assert that the same connection is reused
		require.Len(suite.T(), nodes[0].libP2PHost.Network().Conns(), 1)
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
		require.Equal(suite.T(), i, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), flowProtocolID, network.DirOutbound))
	}
}

// TestOneToOneComm sends a message from node 1 to node 2 and then from node 2 to node 1
func (suite *LibP2PNodeTestSuite) TestOneToOneComm() {

	count := 2
	ch := make(chan string, count)

	// Create the handler function
	handler := func(s network.Stream) {
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		str, err := rw.ReadString('\n')
		assert.NoError(suite.T(), err)
		ch <- str
	}

	// Creates peers
	peers, addrs := suite.CreateNodes(count, handler, false)
	defer suite.StopNodes(peers)
	require.Len(suite.T(), addrs, count)

	addr1 := addrs[0]
	addr2 := addrs[1]

	// Create stream from node 1 to node 2
	s1, err := peers[0].CreateStream(context.Background(), addr2)
	assert.NoError(suite.T(), err)
	rw := bufio.NewReadWriter(bufio.NewReader(s1), bufio.NewWriter(s1))

	// Send message from node 1 to 2
	msg := "hello\n"
	_, err = rw.WriteString(msg)
	assert.NoError(suite.T(), err)

	// Flush the stream
	assert.NoError(suite.T(), rw.Flush())

	// Wait for the message to be received
	select {
	case rcv := <-ch:
		require.Equal(suite.T(), msg, rcv)
	case <-time.After(1 * time.Second):
		assert.Fail(suite.T(), "message not received")
	}

	// Create stream from node 2 to node 1
	s2, err := peers[1].CreateStream(context.Background(), addr1)
	assert.NoError(suite.T(), err)
	rw = bufio.NewReadWriter(bufio.NewReader(s2), bufio.NewWriter(s2))

	// Send message from node 2 to 1
	msg = "hey\n"
	_, err = rw.WriteString(msg)
	assert.NoError(suite.T(), err)

	// Flush the stream
	assert.NoError(suite.T(), rw.Flush())

	select {
	case rcv := <-ch:
		require.Equal(suite.T(), msg, rcv)
	case <-time.After(3 * time.Second):
		assert.Fail(suite.T(), "message not received")
	}
}

// TestCreateStreamTimeoutWithUnresponsiveNode tests that the CreateStream call does not block longer than the default
// unicast timeout interval
func (suite *LibP2PNodeTestSuite) TestCreateStreamTimeoutWithUnresponsiveNode() {

	// creates a regular node
	peers, addrs := suite.CreateNodes(1, nil, false)
	defer suite.StopNodes(peers)
	require.Len(suite.T(), addrs, 1)

	// create a silent node which never replies
	listener, silentNodeAddress := newSilentNode(suite.T())
	defer func() {
		require.NoError(suite.T(), listener.Close())
	}()

	// setup the context to expire after the default timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultUnicastTimeout)
	defer cancel()

	// attempt to create a stream from node 1 to node 2 and assert that it fails after timeout
	grace := 1 * time.Second
	var err error
	unittest.AssertReturnsBefore(suite.T(),
		func() {
			_, err = peers[0].CreateStream(ctx, silentNodeAddress)
		},
		DefaultUnicastTimeout+grace)
	assert.Error(suite.T(), err)
}

// TestCreateStreamIsConcurrent tests that CreateStream calls can be made concurrently such that one blocked call
// does not block another concurrent call.
func (suite *LibP2PNodeTestSuite) TestCreateStreamIsConcurrent() {

	// bump up the unicast timeout to a high value
	unicastTimeout = time.Hour

	// create two regular node
	goodPeers, goodAddrs := suite.CreateNodes(2, nil, false)
	defer suite.StopNodes(goodPeers)
	require.Len(suite.T(), goodAddrs, 2)

	// create a silent node which never replies
	listener, silentNodeAddress := newSilentNode(suite.T())
	defer func() {
		require.NoError(suite.T(), listener.Close())
	}()

	// creates a stream to unresponsive node and makes sure that the stream creation is blocked
	blockedCallCh := unittest.RequireNeverReturnBefore(suite.T(),
		func() {
			_, _ = goodPeers[0].CreateStream(suite.ctx, silentNodeAddress) // this call will block
		},
		1*time.Second,
		"CreateStream attempt to the unresponsive peer did not block")

	// requires same peer can still connect to the other regular peer without being blocked
	unittest.RequireReturnsBefore(suite.T(),
		func() {
			_, err := goodPeers[0].CreateStream(suite.ctx, goodAddrs[1])
			require.NoError(suite.T(), err)
		},
		1*time.Second, "creating stream to a responsive node failed while concurrently blocked on unresponsive node")

	// requires the CreateStream call to the unresponsive node was blocked while we attempted the CreateStream to the
	// good address
	unittest.RequireNeverClosedWithin(suite.T(), blockedCallCh, 1*time.Millisecond,
		"CreateStream attempt to the unresponsive peer did not block after connecting to good node")

}

// TestCreateStreamIsConcurrencySafe tests that the CreateStream is concurrency safe
func (suite *LibP2PNodeTestSuite) TestCreateStreamIsConcurrencySafe() {

	// create two nodes
	peers, addrs := suite.CreateNodes(2, nil, false)
	defer suite.StopNodes(peers)
	require.Len(suite.T(), addrs, 2)

	wg := sync.WaitGroup{}

	// create a gate which gates the call to CreateStream for all concurrent go routines
	gate := make(chan struct{})

	createStream := func() {
		<-gate
		_, err := peers[0].CreateStream(suite.ctx, addrs[1])
		assert.NoError(suite.T(), err) // assert that stream was successfully created
		wg.Done()
	}

	// kick off 10 concurrent calls to CreateStream
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go createStream()
	}
	// open the gate by closing the channel
	close(gate)

	// no call should block
	unittest.AssertReturnsBefore(suite.T(), wg.Wait, 10*time.Second)
}

// TestStreamClosing tests 1-1 communication with streams closed using libp2p2 handler.FullClose
func (suite *LibP2PNodeTestSuite) TestStreamClosing() {

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
					assert.Fail(suite.T(), fmt.Sprintf("received error %v", err))
					err = s.Reset()
					assert.NoError(suite.T(), err)
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
	peers, addrs := suite.CreateNodes(2, handler, false)
	defer suite.StopNodes(peers)

	for i := 0; i < count; i++ {
		// Create stream from node 1 to node 2 (reuse if one already exists)
		s, err := peers[0].CreateStream(context.Background(), addrs[1])
		assert.NoError(suite.T(), err)
		w := bufio.NewWriter(s)

		// Send message from node 1 to 2
		msg := fmt.Sprintf("hello%d\n", i)
		_, err = w.WriteString(msg)
		assert.NoError(suite.T(), err)

		// Flush the stream
		assert.NoError(suite.T(), w.Flush())
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func(s network.Stream) {
			defer wg.Done()
			// close the stream
			err := helpers.FullClose(s)
			require.NoError(suite.T(), err)
		}(s)
		// wait for stream to be closed
		wg.Wait()

		// wait for the message to be received
		unittest.RequireReturnsBefore(suite.T(),
			func() {
				rcv := <-ch
				require.Equal(suite.T(), msg, rcv)
			},
			10*time.Second,
			fmt.Sprintf("message %s not received", msg))
	}
}

// TestPing tests that a node can ping another node
func (suite *LibP2PNodeTestSuite) TestPing() {

	// creates two nodes
	nodes, nodeAddr := suite.CreateNodes(2, nil, false)
	defer suite.StopNodes(nodes)

	node1 := nodes[0]
	node2 := nodes[1]
	node1Addr := nodeAddr[0]
	node2Addr := nodeAddr[1]

	// test node1 can ping node 2
	_, err := node1.Ping(suite.ctx, node2Addr)
	require.NoError(suite.T(), err)

	// test node 2 can ping node 1
	_, err = node2.Ping(suite.ctx, node1Addr)
	require.NoError(suite.T(), err)
}

// TestConnectionGating tests node allow listing by peer.ID
func (suite *LibP2PNodeTestSuite) TestConnectionGating() {

	// create 2 nodes
	nodes, nodeAddrs := suite.CreateNodes(2, nil, true)

	node1 := nodes[0]
	node1Addr := nodeAddrs[0]
	defer suite.StopNode(node1)

	node2 := nodes[1]
	node2Addr := nodeAddrs[1]
	defer suite.StopNode(node2)

	requireError := func(err error) {
		require.Error(suite.T(), err)
		require.True(suite.T(), errors.Is(err, swarm.ErrGaterDisallowedConnection))
	}

	suite.Run("outbound connection to a not-allowed node is rejected", func() {
		// node1 and node2 both have no allowListed peers
		_, err := node1.CreateStream(suite.ctx, node2Addr)
		requireError(err)
		_, err = node2.CreateStream(suite.ctx, node1Addr)
		requireError(err)
	})

	suite.Run("inbound connection from an allowed node is rejected", func() {

		// node1 allowlists node2 but node2 does not allowlists node1
		err := node1.UpdateAllowlist([]NodeAddress{node2Addr}...)
		require.NoError(suite.T(), err)

		// node1 attempts to connect to node2
		// node2 should reject the inbound connection
		_, err = node1.CreateStream(suite.ctx, node2Addr)
		require.Error(suite.T(), err)
	})

	suite.Run("outbound connection to an approved node is allowed", func() {

		// node1 allowlists node2
		err := node1.UpdateAllowlist([]NodeAddress{node2Addr}...)
		require.NoError(suite.T(), err)
		// node2 allowlists node1
		err = node2.UpdateAllowlist([]NodeAddress{node1Addr}...)
		require.NoError(suite.T(), err)

		// node1 should be allowed to connect to node2
		_, err = node1.CreateStream(suite.ctx, node2Addr)
		require.NoError(suite.T(), err)
		// node2 should be allowed to connect to node1
		_, err = node2.CreateStream(suite.ctx, node1Addr)
		require.NoError(suite.T(), err)
	})
}

// CreateNodes creates a number of libp2pnodes equal to the count with the given callback function for stream handling
// it also asserts the correctness of nodes creations
// a single error in creating one node terminates the entire test
func (suite *LibP2PNodeTestSuite) CreateNodes(count int, handler network.StreamHandler, allowList bool) ([]*P2PNode, []NodeAddress) {
	// keeps track of errors on creating a node
	var err error
	var nodes []*P2PNode

	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			suite.StopNodes(nodes)
		}
	}()

	// creating nodes
	var nodeAddrs []NodeAddress
	for i := 0; i < count; i++ {

		name := fmt.Sprintf("node%d", i+1)
		pkey, err := generateNetworkingKey(name)
		require.NoError(suite.T(), err)

		// create a node on localhost with a random port assigned by the OS
		n, nodeID := suite.CreateNode(name, pkey, "0.0.0.0", "0", rootBlockID, handler, allowList)
		nodes = append(nodes, n)
		nodeAddrs = append(nodeAddrs, nodeID)
	}
	return nodes, nodeAddrs
}

func (suite *LibP2PNodeTestSuite) CreateNode(name string, key crypto.PrivKey, ip string, port string, rootID string,
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

	err := n.Start(suite.ctx, nodeID, suite.logger, key, handlerFunc, rootID, allowList, nil)
	require.NoError(suite.T(), err)
	require.Eventuallyf(suite.T(), func() bool {
		ip, p, err := n.GetIPPort()
		return err == nil && ip != "" && p != ""
	}, 3*time.Second, tickForAssertEventually, fmt.Sprintf("could not start node %s", name))

	// get the actual IP and port that have been assigned by the subsystem
	nodeID.IP, nodeID.Port, err = n.GetIPPort()
	require.NoError(suite.T(), err)

	return n, nodeID
}

// StopNodes stop all nodes in the input slice
func (suite *LibP2PNodeTestSuite) StopNodes(nodes []*P2PNode) {
	for _, n := range nodes {
		suite.StopNode(n)
	}
}

func (suite *LibP2PNodeTestSuite) StopNode(node *P2PNode) {
	done, err := node.Stop()
	assert.NoError(suite.T(), err)
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

// newSilentNode returns a TCP listener and a node which never replies
func newSilentNode(t *testing.T) (net.Listener, NodeAddress) {

	name := "silent"
	key, err := generateNetworkingKey(name)
	require.NoError(t, err)

	lst, err := net.Listen("tcp4", ":0")
	if err != nil {
		assert.NoError(t, err)
	}

	addr, err := manet.FromNetAddr(lst.Addr())
	if err != nil {
		assert.NoError(t, err)
	}

	addrs := []multiaddr.Multiaddr{addr}
	addrs, err = addrutil.ResolveUnspecifiedAddresses(addrs, nil)
	if err != nil {
		t.Fatal(err)
	}

	go acceptAndHang(lst)

	ip, port, err := IPPortFromMultiAddress(addrs...)
	if err != nil {
		t.Fatal(err)
	}
	nodeAddress := NodeAddress{
		Name:   name,
		IP:     ip,
		Port:   port,
		PubKey: key.GetPublic(),
	}
	return lst, nodeAddress
}

func acceptAndHang(l net.Listener) {
	conns := make([]net.Conn, 0, 10)
	for {
		c, err := l.Accept()
		if err != nil {
			break
		}
		if c != nil {
			conns = append(conns, c)
		}
	}
	for _, c := range conns {
		c.Close()
	}
}
