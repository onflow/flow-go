package libp2p

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	golog "github.com/ipfs/go-log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gologging "github.com/whyrusleeping/go-logging"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const tickForAssertEventually = 100 * time.Millisecond
var setupOnce sync.Once
var nodes []*P2PNode
var totalNodes = 2

func TestLibP2PNode_Start_Stop(t *testing.T) {
	t.Skip(" A libp2p issue causes this test to fail once in a while. Ignoring test")
	createLibP2PNodes(context.Background(), t)
	//assert.NoError(t, nodes[0].Stop())
}

// TestLibP2PNode_GetPeerInfo checks that given a node name, the corresponding node id is consistently generated
// e.g. Node name: "node1" always generates the libp2p node id QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N
func TestLibP2PNode_GetPeerInfo(t *testing.T) {
	t.Skip(" A libp2p issue causes this test to fail once in a while. Ignoring test")
	var nodes []NodeAddress
	var ps []peer.AddrInfo
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			nodes = append(nodes, NodeAddress{name: fmt.Sprintf("node%d", j), ip: "1.1.1.1", port: "0"})
			p, err := GetPeerInfo(nodes[j])
			require.NoError(t, err)
			if i == 0 {
				ps = append(ps, p)
			} else {
				assert.Equal(t, ps[j].ID.String(), p.ID.String(), fmt.Sprintf(" node ids not consistently generated"))
			}
		}
	}
}

// TestLibP2PNode_AddPeers checks if nodes can be added as peers to a given node
func TestLibP2PNode_AddPeers(t *testing.T) {
	//t.Skip(" A libp2p issue causes this test to fail once in a while. Ignoring test")
	// A longer timeout is needed to overcome timeouts - https://github.com/ipfs/go-ipfs/issues/5800
	ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Minute)
	defer cancel()
	golog.SetAllLoggers(gologging.DEBUG)

	// count value of 10 runs into this issue on localhost https://github.com/libp2p/go-libp2p-pubsub/issues/96
	// since localhost connection have short deadlines

	// Create nodes
	createLibP2PNodes(ctx, t)
	var ids []NodeAddress
	// Get actual ip and port numbers on which the nodes were started
	for _, n := range nodes[1:] {
		_, p := n.GetIPPort()
		ids = append(ids, NodeAddress{name: n.name, ip: "127.0.0.1", port: p})
	}
	// To the 1st node add the remaining 9 nodes as peers.
	require.NoError(t, nodes[0].AddPeers(ctx, ids))
	actual := nodes[0].libP2PHost.Peerstore().Peers().Len()
	// Check if all 9 nodes have been added as peers to the first node.
	assert.Equal(t, totalNodes, actual, "peers expected: %d, found: %d", totalNodes, actual)

	// Check if libp2p reports node 1 is connected to the other nodes
	for _, a := range nodes[0].libP2PHost.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].libP2PHost.ID().String() == a.String() {
			continue
		}
		assert.Eventuallyf(t, func() bool {
			return network.Connected == nodes[0].libP2PHost.Network().Connectedness(a)
		}, 3*time.Second, tickForAssertEventually, fmt.Sprintf(" node 0 not connected with %s", a.String()))
	}
}

// TestLibP2PNode_PubSub checks if nodes can subscribe to a topic and send and receive a message.
func TestLibP2PNode_PubSub(t *testing.T) {
	t.Skip(" A libp2p issue causes this test to fail once in a while. Ignoring test")
	ctx := context.Background()

	golog.SetAllLoggers(gologging.DEBUG)

	// Step 1: Create nodes
	createLibP2PNodes(ctx, t)


	// Step 2: Subscribe to a flow topic
	// A node will receive it's own message (https://github.com/libp2p/go-libp2p-pubsub/issues/65)
	// hence expect count and not count - 1 messages to be received (one by each node, including the sender)
	ch := make(chan string, totalNodes)
	for _, n := range nodes {
		m := n.name
		// callback that is registered
		cb := func(msg []byte) {
			assert.Equal(t, []byte("hello"), msg)
			ch <- m
		}
		require.NoError(t, n.Subscribe(ctx, Consensus, cb))
	}

	// Step 3: Connect a node to it's neighbour (Daisy chain them (1->2, 2->3...9->10))
	for i := 0; i < totalNodes-1; i++ {
		s := nodes[i]
		d := nodes[i+1]
		dip, dport := d.GetIPPort()
		nd := &NodeAddress{name: d.name, ip: dip, port: dport}
		require.NoError(t, s.AddPeers(ctx, []NodeAddress{*nd}))
		assert.Eventuallyf(t, func() bool {
			return network.Connected == s.libP2PHost.Network().Connectedness(d.libP2PHost.ID())
		}, 3*time.Second, tickForAssertEventually, fmt.Sprintf(" %s not connected with %s", s.name, d.name))
		//e := 2
		//if i%totalNodes == 0 {
		//	e = 1
		//}
		//assert.Equal(t, totalNodes, len(s.ps.ListPeers(string(Consensus))))
	}

	// Step 4: Wait for nodes to hearbeat
	time.Sleep(2 * time.Second)

	// Step 5: Publish a message from the first node and verify all nodes get it.
	// All nodes including node 0 - the sender, should receive it
	nodes[0].Publish(ctx, Consensus, []byte("hello"))
	recv := make(map[string]bool, totalNodes)
	for i := 0; i < totalNodes; i++ {
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
			assert.Fail(t, " messages not received by nodes: "+strings.Join(missing, ", "))
			break
		}
	}

	// Step 6: Unsubscribe all nodes from the topic
	for _, n := range nodes {
		assert.NoError(t, n.UnSubscribe(Consensus))
	}
}

// TestLibP2PNode_P2P tests end-to-end a P2P message sending and receiving between two nodes
func TestLibP2PNode_P2P(t *testing.T) {
	// TODO: Issue#1966
	golog.SetAllLoggers(gologging.DEBUG)
	t.Skip(" A libp2p issue causes this test to fail once in a while. Ignoring test")
	ctx := context.Background()

	createLibP2PNodes(ctx, t)


	// Peer 1 will be sending a message to Peer 2
	peer1 := nodes[0]
	peer2 := nodes[1]

	var ids []NodeAddress
	// Get actual ip and port numbers on which the nodes were started
	for _, n := range nodes {
		ip, p := n.GetIPPort()
		ids = append(ids, NodeAddress{name: n.name, ip: ip, port: p})
	}

	// Add the second node as a peer to the first node
	require.NoError(t, peer1.AddPeers(ctx, ids[1:]))

	// Create and register engines for each of the nodes
	te1 := &StubEngine{t: t}
	conduit1, err := peer1.Register(1, te1)
	require.NoError(t, err)
	te2 := &StubEngine{t: t, ch: make(chan struct{})}
	_, err = peer2.Register(1, te2)
	require.NoError(t, err)

	// Create target byte array from the node name "node2" -> []byte
	var target [32]byte
	copy(target[:], ids[1].name)
	targetID := flow.Identifier(target)

	// Send the message to peer 2 using the conduit of peer 1
	require.NoError(t, conduit1.Submit([]byte("hello"), targetID))

	select {
	case <-te2.ch:
		// Assert that the message was received by peer 2
		require.NotNil(t, te2.id)
		require.NotNil(t, te2.event)
		senderID := bytes.Trim(te2.id[:], "\x00")
		senderIDStr := string(senderID)
		assert.Equal(t, peer1.name, senderIDStr)
		assert.Equal(t, "hello", fmt.Sprintf("%s", te2.event))
	case <-time.After(3 * time.Second):
		assert.Fail(t, "peer 1 failed to send a message to peer 2")
	}
}

func createLibP2PNodes(ctx context.Context, t *testing.T) {
	setupOnce.Do(func() {
		var err error
		defer func() {
			if err != nil && nodes != nil {
				for _, n := range nodes {
					n.Stop()
				}
			}
		}()
		l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
		for i := 1; i <= totalNodes; i++ {
			var n = &P2PNode{}
			var nodeID = NodeAddress{name: fmt.Sprintf("node%d", i), ip: "0.0.0.0", port: "0"}
			err := n.Start(ctx, nodeID, l)
			require.NoError(t, err)
			require.Eventuallyf(t, func() bool {
				ip, p := n.GetIPPort()
				return ip != "" && p != ""
			}, 5*time.Second, tickForAssertEventually, fmt.Sprintf("node%d didn't start", i))
			nodes = append(nodes, n)
		}
		time.Sleep(time.Second * 2)
		require.Len(t, nodes, totalNodes, " node counts not as expected")
	})
}

type StubEngine struct {
	t     *testing.T
	id    flow.Identifier
	event interface{}
	ch    chan struct{}
}

func (te *StubEngine) SubmitLocal(event interface{}) {
	require.Fail(te.t, "not implemented")
}

func (te *StubEngine) Submit(originID flow.Identifier, event interface{}) {
	require.Fail(te.t, "not implemented")
}

func (te *StubEngine) ProcessLocal(event interface{}) error {
	require.Fail(te.t, "not implemented")
	return fmt.Errorf(" unexpected method called")
}

func (te *StubEngine) Process(originID flow.Identifier, event interface{}) error {
	te.id = originID
	te.event = event
	te.ch <- struct{}{}
	return nil
}
