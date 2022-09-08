package p2p_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

// Tests in this file evaluate tests that the network layer behaves as expected after a spork.
// All network related sporking requirements can be supported via libp2p directly,
// without needing any additional support in Flow other than providing the new root block ID.
// Sporking can be supported by two ways:
// 1. Updating the network key of a node after it is moved from the old chain to the new chain
// 2. Updating the Flow Libp2p protocol ID suffix to prevent one-to-one messaging across sporks and
//    updating the channel suffix to prevent one-to-K messaging (PubSub) across sporks.
// 1 and 2 both can independently ensure that nodes from the old chain cannot communicate with nodes on the new chain
// These tests are just to reconfirm the network behaviour and provide a test bed for future tests for sporking, if needed

// TestCrosstalkPreventionOnNetworkKeyChange tests that a node from the old chain cannot talk to a node in the new chain
// if it's network key is updated while the libp2p protocol ID remains the same
func TestCrosstalkPreventionOnNetworkKeyChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx1, cancel1 := context.WithCancel(ctx)
	signalCtx1, errChan1 := irrecoverable.WithSignaler(ctx1)
	go unittest.NoIrrecoverableError(ctx1, t, errChan1, "unexpected irrecoverable on errchan1")

	ctx2, cancel2 := context.WithCancel(ctx)
	signalCtx2, errChan2 := irrecoverable.WithSignaler(ctx2)
	go unittest.NoIrrecoverableError(ctx2, t, errChan2, "unexpected irrecoverable on errchan2")

	ctx2a, cancel2a := context.WithCancel(ctx)
	signalCtx2a, errChan2a := irrecoverable.WithSignaler(ctx2a)
	go unittest.NoIrrecoverableError(ctx2a, t, errChan2a, "unexpected irrecoverable on errchan2a")

	// create and start node 1 on localhost and random port
	node1key := generateNetworkingKey(t)
	sporkId := unittest.IdentifierFixture()

	node1, id1 := nodeFixture(t,
		sporkId,
		"test_crosstalk_prevention_on_network_key_change",
		withNetworkingPrivateKey(node1key),
	)

	startNode(t, signalCtx1, node1, 100*time.Millisecond)
	defer stopNode(t, node1, cancel1, 100*time.Millisecond)

	t.Logf(" %s node started on %s", id1.NodeID.String(), id1.Address)
	t.Logf("libp2p ID for %s: %s", id1.NodeID.String(), node1.Host().ID())

	// create and start node 2 on localhost and random port
	node2key := generateNetworkingKey(t)
	node2, id2 := nodeFixture(t,
		sporkId,
		"test_crosstalk_prevention_on_network_key_change",
		withNetworkingPrivateKey(node2key),
	)
	startNode(t, signalCtx2, node2, 100*time.Millisecond)

	peerInfo2, err := p2p.PeerAddressInfo(id2)
	require.NoError(t, err)

	// create stream from node 1 to node 2
	testOneToOneMessagingSucceeds(t, node1, peerInfo2)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain

	// stop node 2 and start it again with a different networking key but on the same IP and port
	stopNode(t, node2, cancel2, 100*time.Millisecond)

	// start node2 with the same name, ip and port but with the new key
	node2keyNew := generateNetworkingKey(t)
	assert.False(t, node2key.Equals(node2keyNew))
	node2, id2New := nodeFixture(t,
		sporkId,
		"test_crosstalk_prevention_on_network_key_change",
		withNetworkingPrivateKey(node2keyNew),
		withNetworkingAddress(id2.Address),
	)

	startNode(t, signalCtx2a, node2, 100*time.Millisecond)
	defer stopNode(t, node2, cancel2a, 100*time.Millisecond)

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(t, id2New.Address, id2.Address)

	// attempt to create a stream from node 1 (old chain) to node 2 (new chain)
	// this time it should fail since node 2 is using a different public key
	// (and therefore has a different libp2p node id)
	testOneToOneMessagingFails(t, node1, peerInfo2)
}

// TestOneToOneCrosstalkPrevention tests that a node from the old chain cannot talk directly to a node in the new chain
// if the Flow libp2p protocol ID is updated while the network keys are kept the same.
func TestOneToOneCrosstalkPrevention(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx1, cancel1 := context.WithCancel(ctx)
	signalCtx1, errChan1 := irrecoverable.WithSignaler(ctx1)
	go unittest.NoIrrecoverableError(ctx1, t, errChan1, "unexpected irrecoverable on errchan1")

	ctx2, cancel2 := context.WithCancel(ctx)
	signalCtx2, errChan2 := irrecoverable.WithSignaler(ctx2)
	go unittest.NoIrrecoverableError(ctx2, t, errChan2, "unexpected irrecoverable on errchan2")

	ctx2a, cancel2a := context.WithCancel(ctx)
	signalCtx2a, errChan2a := irrecoverable.WithSignaler(ctx2a)
	go unittest.NoIrrecoverableError(ctx2a, t, errChan2a, "unexpected irrecoverable on errchan2a")

	sporkId1 := unittest.IdentifierFixture()

	// create and start node 1 on localhost and random port
	node1, id1 := nodeFixture(t, sporkId1, "test_one_to_one_crosstalk_prevention")

	startNode(t, signalCtx1, node1, 100*time.Millisecond)
	defer stopNode(t, node1, cancel1, 100*time.Millisecond)

	peerInfo1, err := p2p.PeerAddressInfo(id1)
	require.NoError(t, err)

	// create and start node 2 on localhost and random port
	node2, id2 := nodeFixture(t, sporkId1, "test_one_to_one_crosstalk_prevention")

	startNode(t, signalCtx2, node2, 100*time.Millisecond)

	// create stream from node 2 to node 1
	testOneToOneMessagingSucceeds(t, node2, peerInfo1)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain
	// stop node 2 and start it again with a different libp2p protocol id to listen for
	stopNode(t, node2, cancel2, time.Second)

	// start node2 with the same address and root key but different root block id
	node2, id2New := nodeFixture(t,
		unittest.IdentifierFixture(), // update the flow root id for node 2. node1 is still listening on the old protocol
		"test_one_to_one_crosstalk_prevention",
		withNetworkingAddress(id2.Address),
	)

	startNode(t, signalCtx2a, node2, 100*time.Millisecond)
	defer stopNode(t, node2, cancel2a, 100*time.Millisecond)

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(t, id2New.Address, id2.Address)

	// attempt to create a stream from node 2 (new chain) to node 1 (old chain)
	// this time it should fail since node 2 is listening on a different protocol
	testOneToOneMessagingFails(t, node2, peerInfo1)
}

// TestOneToKCrosstalkPrevention tests that a node from the old chain cannot talk to a node in the new chain via PubSub
// if the channel is updated while the network keys are kept the same.
func TestOneToKCrosstalkPrevention(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx1, cancel1 := context.WithCancel(ctx)
	signalCtx1, errChan1 := irrecoverable.WithSignaler(ctx1)
	go unittest.NoIrrecoverableError(ctx1, t, errChan1, "unexpected irrecoverable on errchan1")

	ctx2, cancel2 := context.WithCancel(ctx)
	signalCtx2, errChan2 := irrecoverable.WithSignaler(ctx2)
	go unittest.NoIrrecoverableError(ctx2, t, errChan2, "unexpected irrecoverable on errchan2")

	// root id before spork
	previousSporkId := unittest.IdentifierFixture()

	// create and start node 1 on localhost and random port
	node1, _ := nodeFixture(t,
		previousSporkId,
		"test_one_to_k_crosstalk_prevention",
	)

	startNode(t, signalCtx1, node1, 100*time.Millisecond)
	defer stopNode(t, node1, cancel1, 100*time.Millisecond)

	// create and start node 2 on localhost and random port with the same root block ID
	node2, id2 := nodeFixture(t,
		previousSporkId,
		"test_one_to_k_crosstalk_prevention",
	)

	startNode(t, signalCtx2, node2, 100*time.Millisecond)
	defer stopNode(t, node2, cancel2, 100*time.Millisecond)

	pInfo2, err := p2p.PeerAddressInfo(id2)
	require.NoError(t, err)

	// spork topic is derived by suffixing the channel with the root block ID
	topicBeforeSpork := channels.TopicFromChannel(channels.TestNetworkChannel, previousSporkId)

	// both nodes are initially on the same spork and subscribed to the same topic
	_, err = node1.Subscribe(topicBeforeSpork, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)
	sub2, err := node2.Subscribe(topicBeforeSpork, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)

	// add node 2 as a peer of node 1
	err = node1.AddPeer(ctx, pInfo2)
	require.NoError(t, err)

	// let the two nodes form the mesh
	time.Sleep(time.Second)

	// assert that node 1 can successfully send a message to node 2 via PubSub
	testOneToKMessagingSucceeds(ctx, t, node1, sub2, topicBeforeSpork)

	// new root id after spork
	rootIDAfterSpork := unittest.IdentifierFixture()

	// topic after the spork
	topicAfterSpork := channels.TopicFromChannel(channels.TestNetworkChannel, rootIDAfterSpork)

	// mimic that node1 now is now part of the new spork while node2 remains on the old spork
	// by unsubscribing node1 from 'topicBeforeSpork' and subscribing it to 'topicAfterSpork'
	// and keeping node2 subscribed to topic 'topicBeforeSpork'
	err = node1.UnSubscribe(topicBeforeSpork)
	require.NoError(t, err)
	_, err = node1.Subscribe(topicAfterSpork, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)

	// assert that node 1 can no longer send a message to node 2 via PubSub
	testOneToKMessagingFails(ctx, t, node1, sub2, topicAfterSpork)
}

func testOneToOneMessagingSucceeds(t *testing.T, sourceNode *p2p.Node, peerInfo peer.AddrInfo) {
	// create stream from node 1 to node 2
	sourceNode.Host().Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.AddressTTL)
	s, err := sourceNode.CreateStream(context.Background(), peerInfo.ID)
	// assert that stream creation succeeded
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func testOneToOneMessagingFails(t *testing.T, sourceNode *p2p.Node, peerInfo peer.AddrInfo) {
	// create stream from source node to destination address
	sourceNode.Host().Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.AddressTTL)
	_, err := sourceNode.CreateStream(context.Background(), peerInfo.ID)
	// assert that stream creation failed
	assert.Error(t, err)
	// assert that it failed with the expected error
	assert.Regexp(t, ".*failed to negotiate security protocol.*|.*protocol not supported.*", err)
}

func testOneToKMessagingSucceeds(ctx context.Context,
	t *testing.T,
	sourceNode *p2p.Node,
	dstnSub *pubsub.Subscription,
	topic channels.Topic) {

	payload := createTestMessage(t)

	// send a 1-k message from source node to destination node
	err := sourceNode.Publish(ctx, topic, payload)
	require.NoError(t, err)

	// assert that the message is received by the destination node
	unittest.AssertReturnsBefore(t, func() {
		msg, err := dstnSub.Next(ctx)
		require.NoError(t, err)
		assert.Equal(t, payload, msg.Data)
	},
		// libp2p hearbeats every second, so at most the message should take 1 second
		2*time.Second)
}

func testOneToKMessagingFails(ctx context.Context,
	t *testing.T,
	sourceNode *p2p.Node,
	dstnSub *pubsub.Subscription,
	topic channels.Topic) {

	payload := createTestMessage(t)

	// send a 1-k message from source node to destination node
	err := sourceNode.Publish(ctx, topic, payload)
	require.NoError(t, err)

	// assert that the message is never received by the destination node
	_ = unittest.RequireNeverReturnBefore(t, func() {
		_, _ = dstnSub.Next(ctx)
	},
		// libp2p hearbeats every second, so at most the message should take 1 second
		2*time.Second,
		"nodes on different sporks were able to communicate")
}

func createTestMessage(t *testing.T) []byte {
	b, err := unittest.NetworkCodec().Encode(&libp2pmessage.TestMessage{
		Text: "hello",
	})
	require.NoError(t, err)

	msg := &message.Message{
		Payload: b,
	}
	payload, err := msg.Marshal()
	require.NoError(t, err)

	return payload
}
