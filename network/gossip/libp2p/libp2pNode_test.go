package libp2p

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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
		actualAddress := multiaddressStr(tc.address)
		assert.Equal(l.Suite.T(), tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

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
	require.NoError(l.Suite.T(), nodes[0].AddPeers(l.ctx, ids...))
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

// TestCreateStreams checks if an existing stream is reused instead of creating a new streams each time when CreateStream is called
func (l *LibP2PNodeTestSuite) TestCreateStream() {
	defer l.cancel()
	count := 2

	// Creates nodes
	nodes := l.CreateNodes(count)
	defer l.StopNodes(nodes)

	// Create target NodeAddress
	ip2, port2 := nodes[1].GetIPPort()
	name2 := nodes[1].name
	address2 := NodeAddress{IP: ip2, Port: port2, Name: name2}

	// Assert that there is no outbound stream to the target yet
	require.Equal(l.T(), 0, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), FlowLibP2PProtocolID, network.DirOutbound))

	// Create the outbound stream by calling CreateStream
	firstStream, err := nodes[0].CreateStream(context.Background(), address2)
	// Assert the stream creation was successful
	require.NoError(l.T(), err)
	require.NotNil(l.T(), firstStream)
	require.Equal(l.T(), 1, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), FlowLibP2PProtocolID, network.DirOutbound))

	// Assert that the stream can be written to without error
	n, err := firstStream.Write([]byte("bkjbjbkjbk"))
	require.NoError(l.T(), err)
	require.Greater(l.T(), n, 0)

	// Now attempt to create another 100 outbound stream to the same destination by calling CreateStream
	var streams []network.Stream
	for i := 0; i < 100; i++ {
		anotherStream, err := nodes[0].CreateStream(context.Background(), address2)
		// Assert that a stream was returned without error
		require.NoError(l.T(), err)
		require.NotNil(l.T(), anotherStream)
		// Assert that the stream count within libp2p is still 1 (i.e. No new stream was created)
		require.Equal(l.T(), 1, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), FlowLibP2PProtocolID, network.DirOutbound))
		// Cannot assert that firstStream == anotherStream since the underlying objects are different
		// In other words, require.Equal(l.T(), firstStream, anotherStream) fails
		//(https://discuss.libp2p.io/t/how-to-check-if-a-stream-is-already-open-with-peer/249/7?u=vishal)
		// However, the counts reported by libp2p should prove that no new stream was created.
		streams = append(streams, anotherStream)
	}

	// Close the first stream
	err = firstStream.Reset()
	require.NoError(l.T(), err)
	// This should also close the second stream. Assert that libp2p reports the correct stream count
	require.Equal(l.T(), 0, CountStream(nodes[0].libP2PHost, nodes[1].libP2PHost.ID(), FlowLibP2PProtocolID, network.DirOutbound))

	// Write to each of the other stream (this is another way of confirming that the other streams are indeed the same as firstStream)
	for _, s := range streams {
		_, err = s.Write([]byte("bkjbjbkjbk"))
		require.Error(l.T(), err)
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
	peers := l.CreateNodes(count, handler)
	defer l.StopNodes(peers)

	// Create source NodeAddress
	ip1, port1 := peers[0].GetIPPort()
	addr1 := NodeAddress{IP: ip1, Port: port1, Name: peers[0].name}

	// Create target NodeAddress
	ip2, port2 := peers[1].GetIPPort()
	addr2 := NodeAddress{IP: ip2, Port: port2, Name: peers[1].name}

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

// libp2p.CreateStream() reuses an existing stream if it exists. This test checks if the reused stream works as expected
func (l *LibP2PNodeTestSuite) TestStreamReuse() {
	defer l.cancel()
	ch := make(chan string)
	done := make(chan struct{})

	// Create the handler function
	handler := func(s network.Stream) {
		go func(s network.Stream) {
			rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
			for {
				str, err := rw.ReadString('\n')
				select {
				case <-done:
					return
				default:
					assert.NoError(l.T(), err)
					ch <- str
				}
			}
		}(s)
	}

	// Creates peers
	peers := l.CreateNodes(2, handler)
	defer l.StopNodes(peers)
	defer close(done)

	// Create target NodeAddress
	ip2, port2 := peers[1].GetIPPort()
	na2 := NodeAddress{IP: ip2, Port: port2, Name: peers[1].name}

	for i := 0; i < 10; i++ {
		// Create stream from node 1 to node 2 (reuse if one already exists)
		s, err := peers[0].CreateStream(context.Background(), na2)
		assert.NoError(l.T(), err)
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

		// Send message from node 1 to 2
		msg := fmt.Sprintf("hello%d\n", i)
		_, err = rw.WriteString(msg)
		assert.NoError(l.T(), err)

		// Flush the stream
		assert.NoError(l.T(), rw.Flush())

		// Wait for the message to be received
		select {
		case rcv := <-ch:
			require.Equal(l.T(), msg, rcv)
		case <-time.After(10 * time.Second):
			assert.Fail(l.T(), fmt.Sprintf("message %s not received", msg))
		}
	}
}

// CreateNodes creates a number of libp2pnodes equal to the count with the given callback function for stream handling
// it also asserts the correctness of nodes creations
// a single error in creating one node terminates the entire test
func (l *LibP2PNodeTestSuite) CreateNodes(count int, handler ...network.StreamHandler) (nodes []*P2PNode) {
	// keeps track of errors on creating a node
	var err error
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			l.StopNodes(nodes)
		}
	}()

	var handlerFunc network.StreamHandler
	if len(handler) > 0 {
		// use the callback that has been passed in
		handlerFunc = handler[0]
	} else {
		// use a default call back
		handlerFunc = func(network.Stream) {}
	}

	// creating nodes
	for i := 1; i <= count; i++ {
		n := &P2PNode{}
		nodeID := NodeAddress{
			Name: fmt.Sprintf("node%d", i),
			IP:   "0.0.0.0", // localhost
			Port: "0",       // random Port number
		}

		err := n.Start(l.ctx, nodeID, logger, handlerFunc)
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
	for _, n := range nodes {
		assert.NoError(l.Suite.T(), n.Stop())
	}
}
