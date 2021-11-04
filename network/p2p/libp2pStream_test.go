package p2p

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
	// suite.T().Skip("QUARANTINED as FLAKY: closing network.Stream.Close() often errors in handler function, thereby failing this test")

	count := 10
	ch := make(chan string, count)
	defer close(ch)
	done := make(chan struct{})
	defer close(done)

	// Create the handler function
	handler := func(t *testing.T) network.StreamHandler {
		h := func(s network.Stream) {
			go func(s network.Stream) {
				rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
				for {
					str, err := rw.ReadString('\n')
					if err != nil {
						if errors.Is(err, io.EOF) {
							err := s.Close()
							assert.NoError(t, err)
							return
						}
						assert.Fail(t, fmt.Sprintf("received error %v", err))
						err = s.Reset()
						assert.NoError(t, err)
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
		return h
	}

	// Creates nodes
	nodes, identities := NodesFixtureWithHandler(t, 2, handler, false)
	defer StopNodes(t, nodes)
	nodeInfo1, err := PeerAddressInfo(*identities[1])
	require.NoError(t, err)

	for i := 0; i < count; i++ {
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
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func(s network.Stream) {
			defer wg.Done()
			// close the stream
			err := s.Close()
			require.NoError(t, err)
		}(s)
		// wait for stream to be closed
		unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not close stream")

		// wait for the message to be received
		unittest.RequireReturnsBefore(t,
			func() {
				rcv := <-ch
				require.Equal(t, msg, rcv)
			},
			10*time.Second,
			fmt.Sprintf("message %s not received", msg))
	}
}
