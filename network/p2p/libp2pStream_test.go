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
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestStreamClosing tests 1-1 communication with streams closed using libp2p2 handler.FullClose
func TestStreamClosing(t *testing.T) {
	count := 10
	ctx, cancel := context.WithCancel(context.Background())
	var msgRegex = regexp.MustCompile("^hello[0-9]")

	handler, streamCloseWG := mockStreamHandlerForMessages(t, ctx, count, msgRegex)

	// Creates nodes
	nodes, identities := NodesFixtureWithHandler(t, 2, handler, false)
	defer StopNodes(t, nodes)
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
						fmt.Printf("[debug] stream closing: %v -> %v \n", s.Conn().LocalMultiaddr(), s.Conn().RemoteMultiaddr())
						err := s.Close()
						fmt.Printf("[debug] stream closed: %v -> %v\n", s.Conn().LocalMultiaddr(), s.Conn().RemoteMultiaddr())
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
	nodes, identities := NodesFixtureWithHandler(t, count, nil, false)
	defer StopNodes(t, nodes)

	id2 := identities[1]

	flowProtocolID := FlowProtocolID(rootBlockID)
	// Assert that there is no outbound stream to the target yet
	require.Equal(t, 0, CountStream(nodes[0].host, nodes[1].host.ID(), flowProtocolID, network.DirOutbound))

	// Now attempt to create another 100 outbound stream to the same destination by calling CreateStream
	var streams []network.Stream
	for i := 0; i < 100; i++ {
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
	for i := 99; i >= 0; i-- {
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
