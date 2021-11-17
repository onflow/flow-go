package p2p

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestStreamClosing tests 1-1 communication with streams closed using libp2p2 handler.FullClose
func TestStreamClosing(t *testing.T) {
	count := 10
	ctx, cancel := context.WithCancel(context.Background())
	var msgRegex = regexp.MustCompile("^hello[0-9]")

	handler, streamCloseWG := mockStreamHandlerForMessages(t, ctx, count, msgRegex)

	// Creates nodes
	nodes, identities := nodesFixtureWithHandler(t, 2, handler, false)
	defer stopNodes(t, nodes)
	defer cancel()

	nodeInfo1, err := PeerAddressInfo(*identities[1])
	require.NoError(t, err)

	senderWG := sync.WaitGroup{}
	senderWG.Add(count)
	for i := 0; i < count; i++ {
		go func(i int) {
			// Create stream from node 1 to node 2 (reuse if one already exists)
			nodes[0].host.Peerstore().AddAddrs(nodeInfo1.ID, nodeInfo1.Addrs, peerstore.AddressTTL)
			s, err := nodes[0].CreateStream(context.Background(), nodeInfo1.ID)
			assert.NoError(t, err)
			w := bufio.NewWriter(s)

			// Send message from node 1 to 2
			msg := fmt.Sprintf("hello%d\n", i)
			_, err = w.WriteString(msg)
			assert.NoError(t, err)

			// Flush the stream
			assert.NoError(t, w.Flush())

			// close the stream
			err = s.Close()
			require.NoError(t, err)

			senderWG.Done()
		}(i)
	}

	// wait for stream to be closed
	unittest.RequireReturnsBefore(t, senderWG.Wait, 1*time.Second, "could not send messages on time")
	unittest.RequireReturnsBefore(t, streamCloseWG.Wait, 1*time.Second, "could not close stream at receiver side")
}

// mockStreamHandlerForMessages creates a stream handler that expects receiving `msgCount` unique messages that match the input regexp.
// The returned wait group will be unlocked when all messages are completely received and associated streams are closed.
func mockStreamHandlerForMessages(t *testing.T, ctx context.Context, msgCount int, msgRegexp *regexp.Regexp) (network.StreamHandler, *sync.WaitGroup) {
	streamCloseWG := &sync.WaitGroup{}
	streamCloseWG.Add(msgCount)

	h := func(s network.Stream) {
		go func(s network.Stream) {
			rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
			for {
				str, err := rw.ReadString('\n')
				if err != nil {
					if errors.Is(err, io.EOF) {
						err := s.Close()
						require.NoError(t, err)

						streamCloseWG.Done()
						return
					}
					require.Fail(t, fmt.Sprintf("received error %v", err))
					err = s.Reset()
					require.NoError(t, err)
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
					require.True(t, msgRegexp.MatchString(str), str)
				}
			}
		}(s)

	}
	return h, streamCloseWG
}

// TestCreateStreams checks if a new streams is created each time when CreateStream is called and an existing stream is not reused
func TestCreateStream(t *testing.T) {
	count := 2

	// Creates nodes
	nodes, identities := nodesFixtureWithHandler(t, count, nil, false)
	defer stopNodes(t, nodes)

	id2 := identities[1]

	flowProtocolID := unicast.FlowProtocolID(rootBlockID)
	// Assert that there is no outbound stream to the target yet
	require.Equal(t, 0, CountStream(nodes[0].host, nodes[1].host.ID(), flowProtocolID, network.DirOutbound))

	// Now attempt to create another 100 outbound stream to the same destination by calling CreateStream
	streamCount := 100
	var streams []network.Stream
	for i := 0; i < streamCount; i++ {
		pInfo, err := PeerAddressInfo(*id2)
		require.NoError(t, err)
		nodes[0].host.Peerstore().AddAddrs(pInfo.ID, pInfo.Addrs, peerstore.AddressTTL)
		anotherStream, err := nodes[0].CreateStream(context.Background(), pInfo.ID)
		// Assert that a stream was returned without error
		require.NoError(t, err)
		require.NotNil(t, anotherStream)
		// assert that the stream count within libp2p incremented (a new stream was created)
		require.Equal(t, i+1, CountStream(nodes[0].host, nodes[1].host.ID(), flowProtocolID, network.DirOutbound))
		// assert that the same connection is reused
		require.Len(t, nodes[0].host.Network().Conns(), 1)
		streams = append(streams, anotherStream)
	}

	// reverse loop to close all the streams
	for i := streamCount - 1; i >= 0; i-- {
		s := streams[i]
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			err := s.Close()
			assert.NoError(t, err)
			wg.Done()
		}()
		wg.Wait()
		// assert that the stream count within libp2p decremented
		require.Equal(t, i, CountStream(nodes[0].host, nodes[1].host.ID(), flowProtocolID, network.DirOutbound))
	}
}

// TestCreateStreamIsConcurrencySafe tests that the CreateStream is concurrency safe
func TestCreateStreamIsConcurrencySafe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create two nodes
	nodes, identities := nodesFixtureWithHandler(t, 2, nil, false)
	defer stopNodes(t, nodes)
	require.Len(t, identities, 2)
	nodeInfo1, err := PeerAddressInfo(*identities[1])
	require.NoError(t, err)

	wg := sync.WaitGroup{}

	// create a gate which gates the call to CreateStream for all concurrent go routines
	gate := make(chan struct{})

	createStream := func() {
		<-gate
		nodes[0].host.Peerstore().AddAddrs(nodeInfo1.ID, nodeInfo1.Addrs, peerstore.AddressTTL)
		_, err := nodes[0].CreateStream(ctx, nodeInfo1.ID)
		assert.NoError(t, err) // assert that stream was successfully created
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
	unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second)
}

// TestNoBackoffWhenCreateStream checks that backoff is not enabled between attempts to connect to a remote peer
// for one-to-one direct communication.
func TestNoBackoffWhenCreatingStream(t *testing.T) {

	count := 2
	// Creates nodes
	nodes, identities := nodesFixtureWithHandler(t, count, nil, false)
	node1 := nodes[0]
	node2 := nodes[1]

	// stop node 2 immediately
	stopNode(t, node2)
	defer stopNode(t, node1)

	id2 := identities[1]
	pInfo, err := PeerAddressInfo(*id2)
	require.NoError(t, err)
	nodes[0].host.Peerstore().AddAddrs(pInfo.ID, pInfo.Addrs, peerstore.AddressTTL)
	maxTimeToWait := maxConnectAttempt * unicast.MaxConnectAttemptSleepDuration * time.Millisecond

	// need to add some buffer time so that RequireReturnsBefore waits slightly longer than maxTimeToWait to avoid
	// a race condition
	someGraceTime := 100 * time.Millisecond
	totalWaitTime := maxTimeToWait + someGraceTime

	//each CreateStream() call may try to connect up to maxConnectAttempt (3) times.

	//there are 2 scenarios that we need to account for:
	//
	//1. machines where a timeout occurs on the first connection attempt - this can be due to local firewall rules or other processes running on the machine.
	//   In this case, we need to create a scenario where a backoff would have normally occured. This is why we initiate a second connection attempt.
	//   Libp2p remembers the peer we are trying to connect to between CreateStream() calls and would have initiated a backoff if backoff wasn't turned off.
	//   The second CreateStream() call will make a second connection attempt maxConnectAttempt times and that should never result in a backoff error.
	//
	//2. machines where a timeout does NOT occur on the first connection attempt - this is on CI machines and some local dev machines without a firewall / too many other processes.
	//   In this case, there will be maxConnectAttempt (3) connection attempts on the first CreateStream() call and maxConnectAttempt (3) attempts on the second CreateStream() call.

	// make two separate stream creation attempt and assert that no connection back off happened
	for i := 0; i < 2; i++ {

		// limit the maximum amount of time to wait for a connection to be established by using a context that times out
		ctx, cancel := context.WithTimeout(context.Background(), maxTimeToWait)

		unittest.RequireReturnsBefore(t, func() {
			_, err = node1.CreateStream(ctx, pInfo.ID)
		}, totalWaitTime, fmt.Sprintf("create stream did not error within %s", totalWaitTime.String()))
		require.Error(t, err)
		require.NotContainsf(t, err.Error(), swarm.ErrDialBackoff.Error(), "swarm dialer unexpectedly did a back off for a one-to-one connection")
		cancel()
	}
}

// TestOneToOneComm sends a message from node 1 to node 2 and then from node 2 to node 1
func TestOneToOneComm(t *testing.T) {
	count := 2
	ch := make(chan string, count)

	// Create the handler function
	streamHandler := func(s network.Stream) {
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		str, err := rw.ReadString('\n')
		assert.NoError(t, err)
		ch <- str
	}

	// Creates nodes
	nodes, identities := nodesFixtureWithHandler(t, count, streamHandler, false)
	defer stopNodes(t, nodes)
	require.Len(t, identities, count)

	id1 := *identities[0]
	id2 := *identities[1]
	pInfo1, err := PeerAddressInfo(id1)
	require.NoError(t, err)
	pInfo2, err := PeerAddressInfo(id2)
	require.NoError(t, err)

	// Create stream from node 1 to node 2
	nodes[0].host.Peerstore().AddAddrs(pInfo2.ID, pInfo2.Addrs, peerstore.AddressTTL)
	s1, err := nodes[0].CreateStream(context.Background(), pInfo2.ID)
	assert.NoError(t, err)
	rw := bufio.NewReadWriter(bufio.NewReader(s1), bufio.NewWriter(s1))

	// Send message from node 1 to 2
	msg := "hello\n"
	_, err = rw.WriteString(msg)
	assert.NoError(t, err)

	// Flush the stream
	assert.NoError(t, rw.Flush())

	// Wait for the message to be received
	select {
	case rcv := <-ch:
		require.Equal(t, msg, rcv)
	case <-time.After(1 * time.Second):
		assert.Fail(t, "message not received")
	}

	// Create stream from node 2 to node 1
	nodes[1].host.Peerstore().AddAddrs(pInfo1.ID, pInfo1.Addrs, peerstore.AddressTTL)
	s2, err := nodes[1].CreateStream(context.Background(), pInfo1.ID)
	assert.NoError(t, err)
	rw = bufio.NewReadWriter(bufio.NewReader(s2), bufio.NewWriter(s2))

	// Send message from node 2 to 1
	msg = "hey\n"
	_, err = rw.WriteString(msg)
	assert.NoError(t, err)

	// Flush the stream
	assert.NoError(t, rw.Flush())

	select {
	case rcv := <-ch:
		require.Equal(t, msg, rcv)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "message not received")
	}
}

// TestCreateStreamTimeoutWithUnresponsiveNode tests that the CreateStream call does not block longer than the
// timeout interval
func TestCreateStreamTimeoutWithUnresponsiveNode(t *testing.T) {

	// creates a regular node
	nodes, identities := nodesFixtureWithHandler(t, 1, nil, false)
	defer stopNodes(t, nodes)
	require.Len(t, identities, 1)

	// create a silent node which never replies
	listener, silentNodeId := silentNodeFixture(t)
	defer func() {
		require.NoError(t, listener.Close())
	}()

	silentNodeInfo, err := PeerAddressInfo(silentNodeId)
	require.NoError(t, err)

	timeout := 1 * time.Second

	// setup the context to expire after the default timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// attempt to create a stream from node 1 to node 2 and assert that it fails after timeout
	grace := 100 * time.Millisecond
	unittest.AssertReturnsBefore(t,
		func() {
			nodes[0].host.Peerstore().AddAddrs(silentNodeInfo.ID, silentNodeInfo.Addrs, peerstore.AddressTTL)
			_, err = nodes[0].CreateStream(ctx, silentNodeInfo.ID)
		},
		timeout+grace)
	assert.Error(t, err)
}

// TestCreateStreamIsConcurrent tests that CreateStream calls can be made concurrently such that one blocked call
// does not block another concurrent call.
func TestCreateStreamIsConcurrent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create two regular node
	goodNodes, goodNodeIds := nodesFixtureWithHandler(t, 2, nil, false)
	defer stopNodes(t, goodNodes)
	require.Len(t, goodNodeIds, 2)
	goodNodeInfo1, err := PeerAddressInfo(*goodNodeIds[1])
	require.NoError(t, err)

	// create a silent node which never replies
	listener, silentNodeId := silentNodeFixture(t)
	defer func() {
		require.NoError(t, listener.Close())
	}()
	silentNodeInfo, err := PeerAddressInfo(silentNodeId)
	require.NoError(t, err)

	// creates a stream to unresponsive node and makes sure that the stream creation is blocked
	blockedCallCh := unittest.RequireNeverReturnBefore(t,
		func() {
			goodNodes[0].host.Peerstore().AddAddrs(silentNodeInfo.ID, silentNodeInfo.Addrs, peerstore.AddressTTL)
			_, _ = goodNodes[0].CreateStream(ctx, silentNodeInfo.ID) // this call will block
		},
		1*time.Second,
		"CreateStream attempt to the unresponsive peer did not block")

	// requires same peer can still connect to the other regular peer without being blocked
	unittest.RequireReturnsBefore(t,
		func() {
			goodNodes[0].host.Peerstore().AddAddrs(goodNodeInfo1.ID, goodNodeInfo1.Addrs, peerstore.AddressTTL)
			_, err := goodNodes[0].CreateStream(ctx, goodNodeInfo1.ID)
			require.NoError(t, err)
		},
		1*time.Second, "creating stream to a responsive node failed while concurrently blocked on unresponsive node")

	// requires the CreateStream call to the unresponsive node was blocked while we attempted the CreateStream to the
	// good address
	unittest.RequireNeverClosedWithin(t, blockedCallCh, 1*time.Millisecond,
		"CreateStream attempt to the unresponsive peer did not block after connecting to good node")

}

// TestConnectionGating tests node allow listing by peer.ID
func TestConnectionGating(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create 2 nodes
	nodes, identities := nodesFixtureWithHandler(t, 2, nil, true)

	node1 := nodes[0]
	node1Id := *identities[0]
	defer stopNode(t, node1)
	node1Info, err := PeerAddressInfo(node1Id)
	assert.NoError(t, err)

	node2 := nodes[1]
	node2Id := *identities[1]
	defer stopNode(t, node2)
	node2Info, err := PeerAddressInfo(node2Id)
	assert.NoError(t, err)

	requireError := func(err error) {
		require.Error(t, err)
		require.True(t, errors.Is(err, swarm.ErrGaterDisallowedConnection))
	}

	t.Run("outbound connection to a not-allowed node is rejected", func(t *testing.T) {
		// node1 and node2 both have no allowListed peers
		node1.host.Peerstore().AddAddrs(node2Info.ID, node2Info.Addrs, peerstore.AddressTTL)
		_, err := node1.CreateStream(ctx, node2Info.ID)
		requireError(err)
		node2.host.Peerstore().AddAddrs(node1Info.ID, node1Info.Addrs, peerstore.AddressTTL)
		_, err = node2.CreateStream(ctx, node1Info.ID)
		requireError(err)
	})

	t.Run("inbound connection from an allowed node is rejected", func(t *testing.T) {

		// node1 allowlists node2 but node2 does not allowlists node1
		node1.UpdateAllowList(peer.IDSlice{node2Info.ID})

		// node1 attempts to connect to node2
		// node2 should reject the inbound connection
		node1.host.Peerstore().AddAddrs(node2Info.ID, node2Info.Addrs, peerstore.AddressTTL)
		_, err = node1.CreateStream(ctx, node2Info.ID)
		require.Error(t, err)
	})

	t.Run("outbound connection to an approved node is allowed", func(t *testing.T) {

		// node1 allowlists node2
		node1.UpdateAllowList(peer.IDSlice{node2Info.ID})
		// node2 allowlists node1
		node2.UpdateAllowList(peer.IDSlice{node1Info.ID})

		// node1 should be allowed to connect to node2
		node1.host.Peerstore().AddAddrs(node2Info.ID, node2Info.Addrs, peerstore.AddressTTL)
		_, err = node1.CreateStream(ctx, node2Info.ID)
		require.NoError(t, err)
		// node2 should be allowed to connect to node1
		node2.host.Peerstore().AddAddrs(node1Info.ID, node1Info.Addrs, peerstore.AddressTTL)
		_, err = node2.CreateStream(ctx, node1Info.ID)
		require.NoError(t, err)
	})
}
