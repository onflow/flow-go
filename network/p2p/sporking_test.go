package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCrosstalkPreventionOnNetworkKeyChange tests that a node from the old chain cannot talk to a node in the new chain
// if it's network key is updated while the libp2p protocol ID remains the same
func TestCrosstalkPreventionOnNetworkKeyChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create and start node 1 on localhost and random port
	node1key := generateNetworkingKey(t)
	sporkId := unittest.IdentifierFixture()

	node1, id1 := nodeFixture(t,
		ctx,
		sporkId,
		withNetworkingPrivateKey(node1key))
	defer stopNode(t, node1)
	t.Logf(" %s node started on %s", id1.NodeID.String(), id1.Address)
	t.Logf("libp2p ID for %s: %s", node1.id.String(), node1.host.ID())

	// create and start node 2 on localhost and random port
	node2key := generateNetworkingKey(t)
	node2, id2 := nodeFixture(t,
		ctx,
		sporkId,
		withNetworkingPrivateKey(node2key))
	peerInfo2, err := PeerAddressInfo(id2)
	require.NoError(t, err)

	// create stream from node 1 to node 2
	testOneToOneMessagingSucceeds(t, node1, peerInfo2)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain

	// stop node 2 and start it again with a different networking key but on the same IP and port
	stopNode(t, node2)

	// start node2 with the same name, ip and port but with the new key
	node2keyNew := generateNetworkingKey(t)
	assert.False(t, node2key.Equals(node2keyNew))
	node2, id2New := nodeFixture(t,
		ctx,
		sporkId,
		withNetworkingPrivateKey(node2keyNew),
		withNetworkingAddress(id2.Address))
	defer stopNode(t, node2)

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

	sporkId1 := unittest.IdentifierFixture()

	// create and start node 1 on localhost and random port
	node1, id1 := nodeFixture(t, ctx, sporkId1)

	defer stopNode(t, node1)
	peerInfo1, err := PeerAddressInfo(id1)
	require.NoError(t, err)

	// create and start node 2 on localhost and random port
	node2, id2 := nodeFixture(t, ctx, sporkId1)

	// create stream from node 2 to node 1
	testOneToOneMessagingSucceeds(t, node2, peerInfo1)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain
	// stop node 2 and start it again with a different libp2p protocol id to listen for
	stopNode(t, node2)

	// start node2 with the same address and root key but different root block id
	node2, id2New := nodeFixture(t,
		ctx,
		unittest.IdentifierFixture(), // update the flow root id for node 2. node1 is still listening on the old protocol
		withNetworkingAddress(id2.Address))

	defer stopNode(t, node2)

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

	// root id before spork
	rootIDBeforeSpork := unittest.IdentifierFixture()

	// create and start node 1 on localhost and random port
	node1key := generateNetworkingKey(t)
	node1, _ := nodeFixture(t,
		ctx,
		rootIDBeforeSpork,
		withNetworkingPrivateKey(node1key))

	defer stopNode(t, node1)

	// create and start node 2 on localhost and random port with the same root block ID
	node2key := generateNetworkingKey(t)
	node2, id2 := nodeFixture(t,
		ctx,
		rootIDBeforeSpork,
		withNetworkingPrivateKey(node2key))

	pInfo2, err := PeerAddressInfo(id2)
	defer stopNode(t, node2)
	require.NoError(t, err)

	// spork topic is derived by suffixing the channel with the root block ID
	topicBeforeSpork := engine.TopicFromChannel(engine.TestNetwork, rootIDBeforeSpork)

	// both nodes are initially on the same spork and subscribed to the same topic
	_, err = node1.Subscribe(topicBeforeSpork)
	require.NoError(t, err)
	sub2, err := node2.Subscribe(topicBeforeSpork)
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
	topicAfterSpork := engine.TopicFromChannel(engine.TestNetwork, rootIDAfterSpork)

	// mimic that node1 now is now part of the new spork while node2 remains on the old spork
	// by unsubscribing node1 from 'topicBeforeSpork' and subscribing it to 'topicAfterSpork'
	// and keeping node2 subscribed to topic 'topicBeforeSpork'
	err = node1.UnSubscribe(topicBeforeSpork)
	require.NoError(t, err)
	_, err = node1.Subscribe(topicAfterSpork)
	require.NoError(t, err)

	// assert that node 1 can no longer send a message to node 2 via PubSub
	testOneToKMessagingFails(ctx, t, node1, sub2, topicAfterSpork)
}

func testOneToOneMessagingSucceeds(t *testing.T, sourceNode *Node, peerInfo peer.AddrInfo) {
	// create stream from node 1 to node 2
	sourceNode.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.AddressTTL)
	s, err := sourceNode.CreateStream(context.Background(), peerInfo.ID)
	// assert that stream creation succeeded
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func testOneToOneMessagingFails(t *testing.T, sourceNode *Node, peerInfo peer.AddrInfo) {
	// create stream from source node to destination address
	sourceNode.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.AddressTTL)
	_, err := sourceNode.CreateStream(context.Background(), peerInfo.ID)
	// assert that stream creation failed
	assert.Error(t, err)
	// assert that it failed with the expected error
	assert.Regexp(t, ".*failed to negotiate security protocol.*|.*protocol not supported.*", err)
}

func testOneToKMessagingSucceeds(ctx context.Context,
	t *testing.T,
	sourceNode *Node,
	dstnSub *pubsub.Subscription,
	topic network.Topic) {

	// send a 1-k message from source node to destination node
	payload := []byte("hello")
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
	sourceNode *Node,
	dstnSub *pubsub.Subscription,
	topic network.Topic) {

	// send a 1-k message from source node to destination node
	payload := []byte("hello")
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
