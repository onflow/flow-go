package p2p

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	addrutil "github.com/libp2p/go-addr-util"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const tickForAssertEventually = 100 * time.Millisecond

// Creating a node fixture with defaultAddress lets libp2p runs it on an
// allocated port by OS. So after fixture created, its address would be
// "0.0.0.0:<selected-port-by-os>
const defaultAddress = "0.0.0.0:0"

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
	log.SetAllLoggers(log.LevelError)
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
}

func (suite *LibP2PNodeTestSuite) TearDownTest() {
	suite.cancel()
}

// TestMultiAddress evaluates correct translations from
// dns and ip4 to libp2p multi-address
func (suite *LibP2PNodeTestSuite) TestMultiAddress() {
	key := generateNetworkingKey(suite.T())

	tt := []struct {
		identity     *flow.Identity
		multiaddress string
	}{
		{ // ip4 test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("172.16.254.1:72")),
			multiaddress: "/ip4/172.16.254.1/tcp/72",
		},
		{ // dns test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("consensus:2222")),
			multiaddress: "/dns4/consensus/tcp/2222",
		},
		{ // dns test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("flow.com:3333")),
			multiaddress: "/dns4/flow.com/tcp/3333",
		},
	}

	for _, tc := range tt {
		ip, port, _, err := networkingInfo(*tc.identity)
		require.NoError(suite.T(), err)

		actualAddress := MultiAddressStr(ip, port)
		assert.Equal(suite.T(), tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

}

func (suite *LibP2PNodeTestSuite) TestSingleNodeLifeCycle() {
	// creates a single
	key := generateNetworkingKey(suite.T())
	node, _ := NodeFixture(suite.T(), suite.logger, key, rootBlockID, nil, false, defaultAddress)

	// stops the created node
	done, err := node.Stop()
	assert.NoError(suite.T(), err)
	<-done
}

// TestGetPeerInfo evaluates the deterministic translation between the nodes address and
// their libp2p info. It generates an address, and checks whether repeated translations
// yields the same info or not.
func (suite *LibP2PNodeTestSuite) TestGetPeerInfo() {
	for i := 0; i < 10; i++ {
		key := generateNetworkingKey(suite.T())

		// creates node-i identity
		identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("1.1.1.1:0"))

		// translates node-i address into info
		info, err := PeerAddressInfo(*identity)
		require.NoError(suite.T(), err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := PeerAddressInfo(*identity)
			require.NoError(suite.T(), err)
			assert.True(suite.T(), rinfo.String() == info.String(), "inconsistent id generated")
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func (suite *LibP2PNodeTestSuite) TestAddPeers() {
	count := 3

	// create nodes
	nodes, identities := suite.NodesFixture(count, nil, false)
	defer StopNodes(suite.T(), nodes)

	// add the remaining nodes to the first node as its set of peers
	for _, identity := range identities[1:] {
		require.NoError(suite.T(), nodes[0].AddPeer(suite.ctx, *identity))
	}

	// Checks if all 3 nodes have been added as peers to the first node
	assert.Len(suite.T(), nodes[0].host.Peerstore().Peers(), count)

	// Checks whether the first node is connected to the rest
	for _, peer := range nodes[0].host.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].host.ID().String() == peer.String() {
			continue
		}
		assert.Eventuallyf(suite.T(), func() bool {
			return network.Connected == nodes[0].host.Network().Connectedness(peer)
		}, 2*time.Second, tickForAssertEventually, fmt.Sprintf(" first node is not connected to %s", peer.String()))
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func (suite *LibP2PNodeTestSuite) TestRemovePeers() {

	count := 3

	// create nodes
	nodes, identities := suite.NodesFixture(count, nil, false)
	defer StopNodes(suite.T(), nodes)

	// add nodes two and three to the first node as its peers
	for _, identity := range identities[1:] {
		require.NoError(suite.T(), nodes[0].AddPeer(suite.ctx, *identity))
	}

	// check if all 3 nodes have been added as peers to the first node
	assert.Len(suite.T(), nodes[0].host.Peerstore().Peers(), count)

	// check whether the first node is connected to the rest
	for _, peer := range nodes[0].host.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].host.ID().String() == peer.String() {
			continue
		}
		assert.Eventually(suite.T(), func() bool {
			return network.Connected == nodes[0].host.Network().Connectedness(peer)
		}, 2*time.Second, tickForAssertEventually)
	}

	// disconnect from each peer and assert that the connection no longer exists
	for _, identity := range identities[1:] {
		require.NoError(suite.T(), nodes[0].RemovePeer(suite.ctx, *identity))
		pInfo, err := PeerAddressInfo(*identity)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), network.NotConnected, nodes[0].host.Network().Connectedness(pInfo.ID))
	}
}

// TestCreateStreams checks if a new streams is created each time when CreateStream is called and an existing stream is not reused
func (suite *LibP2PNodeTestSuite) TestCreateStream() {

	count := 2

	// Creates nodes
	nodes, identities := suite.NodesFixture(count, nil, false)
	defer StopNodes(suite.T(), nodes)

	id2 := identities[1]

	flowProtocolID := generateProtocolID(rootBlockID)
	// Assert that there is no outbound stream to the target yet
	require.Equal(suite.T(), 0, CountStream(nodes[0].host, nodes[1].host.ID(), flowProtocolID, network.DirOutbound))

	// Now attempt to create another 100 outbound stream to the same destination by calling CreateStream
	var streams []network.Stream
	for i := 0; i < 100; i++ {
		anotherStream, err := nodes[0].CreateStream(context.Background(), *id2)
		// Assert that a stream was returned without error
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), anotherStream)
		// assert that the stream count within libp2p incremented (a new stream was created)
		require.Equal(suite.T(), i+1, CountStream(nodes[0].host, nodes[1].host.ID(), flowProtocolID, network.DirOutbound))
		// assert that the same connection is reused
		require.Len(suite.T(), nodes[0].host.Network().Conns(), 1)
		streams = append(streams, anotherStream)
	}

	// reverse loop to close all the streams
	for i := 99; i >= 0; i-- {
		s := streams[i]
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			err := s.Close()
			assert.NoError(suite.T(), err)
			wg.Done()
		}()
		wg.Wait()
		// assert that the stream count within libp2p decremented
		require.Equal(suite.T(), i, CountStream(nodes[0].host, nodes[1].host.ID(), flowProtocolID, network.DirOutbound))
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

	// Creates nodes
	nodes, identities := suite.NodesFixture(count, handler, false)
	defer StopNodes(suite.T(), nodes)
	require.Len(suite.T(), identities, count)

	id1 := *identities[0]
	id2 := *identities[1]

	// Create stream from node 1 to node 2
	s1, err := nodes[0].CreateStream(context.Background(), id2)
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
	s2, err := nodes[1].CreateStream(context.Background(), id1)
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
	nodes, identities := suite.NodesFixture(1, nil, false)
	defer StopNodes(suite.T(), nodes)
	require.Len(suite.T(), identities, 1)

	// create a silent node which never replies
	listener, silentNodeId := suite.silentNodeFixture()
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
			_, err = nodes[0].CreateStream(ctx, silentNodeId)
		},
		DefaultUnicastTimeout+grace)
	assert.Error(suite.T(), err)
}

// TestCreateStreamIsConcurrent tests that CreateStream calls can be made concurrently such that one blocked call
// does not block another concurrent call.
func (suite *LibP2PNodeTestSuite) TestCreateStreamIsConcurrent() {
	// create two regular node
	goodNodes, goodNodeIds := suite.NodesFixture(2, nil, false)
	defer StopNodes(suite.T(), goodNodes)
	require.Len(suite.T(), goodNodeIds, 2)

	// create a silent node which never replies
	listener, silentNodeId := suite.silentNodeFixture()
	defer func() {
		require.NoError(suite.T(), listener.Close())
	}()

	// creates a stream to unresponsive node and makes sure that the stream creation is blocked
	blockedCallCh := unittest.RequireNeverReturnBefore(suite.T(),
		func() {
			_, _ = goodNodes[0].CreateStream(suite.ctx, silentNodeId) // this call will block
		},
		1*time.Second,
		"CreateStream attempt to the unresponsive peer did not block")

	// requires same peer can still connect to the other regular peer without being blocked
	unittest.RequireReturnsBefore(suite.T(),
		func() {
			_, err := goodNodes[0].CreateStream(suite.ctx, *goodNodeIds[1])
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
	nodes, identities := suite.NodesFixture(2, nil, false)
	defer StopNodes(suite.T(), nodes)
	require.Len(suite.T(), identities, 2)

	wg := sync.WaitGroup{}

	// create a gate which gates the call to CreateStream for all concurrent go routines
	gate := make(chan struct{})

	createStream := func() {
		<-gate
		_, err := nodes[0].CreateStream(suite.ctx, *identities[1])
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
						err := s.Close()
						assert.NoError(suite.T(), err)
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

	// Creates nodes
	nodes, identities := suite.NodesFixture(2, handler, false)
	defer StopNodes(suite.T(), nodes)

	for i := 0; i < count; i++ {
		// Create stream from node 1 to node 2 (reuse if one already exists)
		s, err := nodes[0].CreateStream(context.Background(), *identities[1])
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
			err := s.Close()
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
	nodes, identities := suite.NodesFixture(2, nil, false)
	defer StopNodes(suite.T(), nodes)

	node1 := nodes[0]
	node2 := nodes[1]
	node1Id := *identities[0]
	node2Id := *identities[1]

	// test node1 can ping node 2
	_, err := node1.Ping(suite.ctx, node2Id)
	require.NoError(suite.T(), err)

	// test node 2 can ping node 1
	_, err = node2.Ping(suite.ctx, node1Id)
	require.NoError(suite.T(), err)
}

// TestConnectionGating tests node allow listing by peer.ID
func (suite *LibP2PNodeTestSuite) TestConnectionGating() {

	// create 2 nodes
	nodes, identities := suite.NodesFixture(2, nil, true)

	node1 := nodes[0]
	node1Id := *identities[0]
	defer StopNode(suite.T(), node1)

	node2 := nodes[1]
	node2Id := *identities[1]
	defer StopNode(suite.T(), node2)

	requireError := func(err error) {
		require.Error(suite.T(), err)
		require.True(suite.T(), errors.Is(err, swarm.ErrGaterDisallowedConnection))
	}

	suite.Run("outbound connection to a not-allowed node is rejected", func() {
		// node1 and node2 both have no allowListed peers
		_, err := node1.CreateStream(suite.ctx, node2Id)
		requireError(err)
		_, err = node2.CreateStream(suite.ctx, node1Id)
		requireError(err)
	})

	suite.Run("inbound connection from an allowed node is rejected", func() {

		// node1 allowlists node2 but node2 does not allowlists node1
		err := node1.UpdateAllowList(flow.IdentityList{&node2Id})
		require.NoError(suite.T(), err)

		// node1 attempts to connect to node2
		// node2 should reject the inbound connection
		_, err = node1.CreateStream(suite.ctx, node2Id)
		require.Error(suite.T(), err)
	})

	suite.Run("outbound connection to an approved node is allowed", func() {

		// node1 allowlists node2
		err := node1.UpdateAllowList(flow.IdentityList{&node2Id})
		require.NoError(suite.T(), err)
		// node2 allowlists node1
		err = node2.UpdateAllowList(flow.IdentityList{&node1Id})
		require.NoError(suite.T(), err)

		// node1 should be allowed to connect to node2
		_, err = node1.CreateStream(suite.ctx, node2Id)
		require.NoError(suite.T(), err)
		// node2 should be allowed to connect to node1
		_, err = node2.CreateStream(suite.ctx, node1Id)
		require.NoError(suite.T(), err)
	})
}

// NodesFixture creates a number of LibP2PNodes with the given callback function for stream handling.
// It returns the nodes and their identities.
func (suite *LibP2PNodeTestSuite) NodesFixture(count int, handler network.StreamHandler, allowList bool) ([]*Node, flow.IdentityList) {
	// keeps track of errors on creating a node
	var err error
	var nodes []*Node

	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			StopNodes(suite.T(), nodes)
		}
	}()

	// creating nodes
	var identities flow.IdentityList
	for i := 0; i < count; i++ {
		// create a node on localhost with a random port assigned by the OS
		key := generateNetworkingKey(suite.T())
		node, identity := NodeFixture(suite.T(), suite.logger, key, rootBlockID, handler, allowList, defaultAddress)
		nodes = append(nodes, node)
		identities = append(identities, &identity)
	}
	return nodes, identities
}

// NodeFixture creates a single LibP2PNodes with the given key, root block id, and callback function for stream handling.
// It returns the nodes and their identities.
func NodeFixture(t *testing.T, log zerolog.Logger, key fcrypto.PrivateKey, rootID string, handler network.StreamHandler, allowList bool, address string) (*Node, flow.Identity) {

	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress(address))

	var handlerFunc network.StreamHandler
	if handler != nil {
		// use the callback that has been passed in
		handlerFunc = handler
	} else {
		// use a default call back
		handlerFunc = func(network.Stream) {}
	}

	noopMetrics := metrics.NewNoopCollector()
	n, err := NewLibP2PNode(log,
		identity.NodeID,
		identity.Address,
		NewConnManager(log, noopMetrics),
		key,
		allowList,
		rootID)
	require.NoError(t, err)
	n.SetStreamHandler(handlerFunc)

	require.Eventuallyf(t, func() bool {
		ip, p, err := n.GetIPPort()
		return err == nil && ip != "" && p != ""
	}, 3*time.Second, tickForAssertEventually, fmt.Sprintf("could not start node %s", identity.NodeID.String()))

	// get the actual IP and port that have been assigned by the subsystem
	ip, port, err := n.GetIPPort()
	require.NoError(t, err)
	identity.Address = ip + ":" + port

	return n, *identity
}

// StopNodes stop all nodes in the input slice
func StopNodes(t *testing.T, nodes []*Node) {
	for _, n := range nodes {
		StopNode(t, n)
	}
}

func StopNode(t *testing.T, node *Node) {
	done, err := node.Stop()
	assert.NoError(t, err)
	<-done
}

// generateNetworkingKey is a test helper that generates a ECDSA flow key pair.
func generateNetworkingKey(t *testing.T) fcrypto.PrivateKey {
	seed := unittest.SeedFixture(48)
	key, err := fcrypto.GeneratePrivateKey(fcrypto.ECDSASecp256k1, seed)
	require.NoError(t, err)
	return key
}

// generateNetworkingAndLibP2PKeys is a test helper that generates a ECDSA flow key pairs, and translate it to
// libp2p key pairs. It returns both generated pairs of keys.
func generateNetworkingAndLibP2PKeys(t *testing.T) (crypto.PrivKey, fcrypto.PrivateKey) {
	// generates flow key
	key := generateNetworkingKey(t)

	// translates flow key into libp2p key
	libP2Pkey, err := privKey(key)
	require.NoError(t, err)

	return libP2Pkey, key
}

// silentNodeFixture returns a TCP listener and a node which never replies
func (suite *LibP2PNodeTestSuite) silentNodeFixture() (net.Listener, flow.Identity) {
	key := generateNetworkingKey(suite.T())

	lst, err := net.Listen("tcp4", ":0")
	require.NoError(suite.T(), err)

	addr, err := manet.FromNetAddr(lst.Addr())
	require.NoError(suite.T(), err)

	addrs := []multiaddr.Multiaddr{addr}
	addrs, err = addrutil.ResolveUnspecifiedAddresses(addrs, nil)
	require.NoError(suite.T(), err)

	go suite.acceptAndHang(lst)

	ip, port, err := IPPortFromMultiAddress(addrs...)
	require.NoError(suite.T(), err)

	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress(ip+":"+port))
	return lst, *identity
}

func (suite *LibP2PNodeTestSuite) acceptAndHang(l net.Listener) {
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
		require.NoError(suite.T(), c.Close())
	}
}
