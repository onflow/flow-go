package p2p

import (
	"context"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/utils/unittest"
)

// SporkingTestSuite tests that the network layer behaves as expected after a spork.
// All network related sporking requirements can be supported via libp2p directly,
// without needing any additional support in Flow other than providing the new root block ID.
// Sporking can be supported by two ways:
// 1. Updating the network key of a node after it is moved from the old chain to the new chain
// 2. Updating the Flow Libp2p protocol ID suffix to prevent one-to-one messaging across sporks and
//    updating the channel ID suffix to prevent one-to-K messaging (PubSub) across sporks.
// 1 and 2 both can independently ensure that nodes from the old chain cannot communicate with nodes on the new chain
// These tests are just to reconfirm the network behaviour and provide a test bed for future tests for sporking, if needed
type SporkingTestSuite struct {
	LibP2PNodeTestSuite
}

func TestSporkingTestSuite(t *testing.T) {
	golog.SetAllLoggers(golog.LevelDebug)
	suite.Run(t, new(SporkingTestSuite))
}

// TestCrosstalkPreventionOnNetworkKeyChange tests that a node from the old chain cannot talk to a node in the new chain
// if it's network key is updated while the libp2p protocol ID remains the same
func (h *SporkingTestSuite) TestCrosstalkPreventionOnNetworkKeyChange() {

	// create and start node 1 on localhost and random port
	node1key, err := generateNetworkingKey("abc")
	require.NoError(h.T(), err)
	node1, address1 := h.CreateNode("node1", node1key, "0.0.0.0", "0", rootBlockID, nil, false)
	defer h.StopNode(node1)
	h.T().Logf(" %s node started on %s:%s", node1.nodeAddress.Name, address1.IP, address1.Port)
	h.T().Logf("libp2p ID for %s: %s", node1.nodeAddress.Name, node1.libP2PHost.Host().ID())

	// create and start node 2 on localhost and random port
	node2key, err := generateNetworkingKey("def")
	require.NoError(h.T(), err)
	node2, address2 := h.CreateNode("node2", node2key, "0.0.0.0", "0", rootBlockID, nil, false)

	// create stream from node 1 to node 2
	testOneToOneMessagingSucceeds(h.T(), node1, address2)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain

	// stop node 2 and start it again with a different networking key but on the same IP and port
	h.StopNode(node2)

	// generate a new key
	node2keyNew, err := generateNetworkingKey("ghi")
	require.NoError(h.T(), err)
	assert.False(h.T(), node2key.Equals(node2keyNew))

	// start node2 with the same name, ip and port but with the new key
	node2, address2New := h.CreateNode(node2.nodeAddress.Name, node2keyNew, address2.IP, address2.Port, rootBlockID, nil, false)
	defer h.StopNode(node2)

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(h.T(), address2.IP, address2New.IP)
	assert.Equal(h.T(), address2.Port, address2New.Port)

	// attempt to create a stream from node 1 (old chain) to node 2 (new chain)
	// this time it should fail since node 2 is using a different public key
	// (and therefore has a different libp2p node id)
	testOneToOneMessagingFails(h.T(), node1, address2)
}

// TestOneToOneCrosstalkPrevention tests that a node from the old chain cannot talk directly to a node in the new chain
// if the Flow libp2p protocol ID is updated while the network keys are kept the same.
func (h *SporkingTestSuite) TestOneToOneCrosstalkPrevention() {

	// root id before spork
	rootID1 := unittest.BlockFixture().ID().String()

	// create and start node 1 on localhost and random port
	node1key, err := generateNetworkingKey("abc")
	require.NoError(h.T(), err)
	node1, address1 := h.CreateNode("node1", node1key, "0.0.0.0", "0", rootID1, nil, false)
	defer h.StopNode(node1)

	// create and start node 2 on localhost and random port
	node2key, err := generateNetworkingKey("def")
	require.NoError(h.T(), err)
	node2, address2 := h.CreateNode("node2", node2key, "0.0.0.0", "0", rootID1, nil, false)

	// create stream from node 2 to node 1
	testOneToOneMessagingSucceeds(h.T(), node2, address1)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain

	// stop node 2 and start it again with a different libp2p protocol id to listen for
	h.StopNode(node2)

	// update the flow root id for node 2. node1 is still listening on the old protocol
	rootID2 := unittest.BlockFixture().ID().String()

	// start node2 with the same name, ip and port but with the new key
	node2, address2New := h.CreateNode(node2.nodeAddress.Name, node2key, address2.IP, address2.Port, rootID2, nil, false)
	defer h.StopNode(node2)

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(h.T(), address2.IP, address2New.IP)
	assert.Equal(h.T(), address2.Port, address2New.Port)

	// attempt to create a stream from node 2 (new chain) to node 1 (old chain)
	// this time it should fail since node 2 is listening on a different protocol
	testOneToOneMessagingFails(h.T(), node2, address1)
}

// TestOneToKCrosstalkPrevention tests that a node from the old chain cannot talk to a node in the new chain via PubSub
// if the channel ID is updated while the network keys are kept the same.
func (h *SporkingTestSuite) TestOneToKCrosstalkPrevention() {

	// root id before spork
	rootIDBeforeSpork := unittest.BlockFixture().ID().String()

	// create and start node 1 on localhost and random port
	node1key, err := generateNetworkingKey("abc")
	require.NoError(h.T(), err)
	node1, _ := h.CreateNode("node1", node1key, "0.0.0.0", "0", rootIDBeforeSpork, nil, false)
	defer h.StopNode(node1)

	// create and start node 2 on localhost and random port with the same root block ID
	node2key, err := generateNetworkingKey("def")
	require.NoError(h.T(), err)
	node2, addr2 := h.CreateNode("node1", node2key, "0.0.0.0", "0", rootIDBeforeSpork, nil, false)
	defer h.StopNode(node2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// spork topic is derived by suffixing the channelID with the root block ID
	topicBeforeSpork := engine.FullyQualifiedChannelName(engine.TestNetwork, rootIDBeforeSpork)

	// both nodes are initially on the same spork and subscribed to the same topic
	_, err = node1.Subscribe(ctx, topicBeforeSpork)
	require.NoError(h.T(), err)
	sub2, err := node2.Subscribe(ctx, topicBeforeSpork)
	require.NoError(h.T(), err)

	// add node 2 as a peer of node 1
	err = node1.AddPeer(ctx, addr2)
	require.NoError(h.T(), err)

	// let the two nodes form the mesh
	time.Sleep(time.Second)

	// assert that node 1 can successfully send a message to node 2 via PubSub
	testOneToKMessagingSucceeds(ctx, h.T(), node1, sub2, topicBeforeSpork)

	// new root id after spork
	rootIDAfterSpork := unittest.BlockFixture().ID().String()

	// topic after the spork
	topicAfterSpork := engine.FullyQualifiedChannelName(engine.TestNetwork, rootIDAfterSpork)

	// mimic that node1 now is now part of the new spork while node2 remains on the old spork
	// by unsubscribing node1 from 'topicBeforeSpork' and subscribing it to 'topicAfterSpork'
	// and keeping node2 subscribed to topic 'topicBeforeSpork'
	err = node1.UnSubscribe(topicBeforeSpork)
	require.NoError(h.T(), err)
	_, err = node1.Subscribe(ctx, topicAfterSpork)
	require.NoError(h.T(), err)

	// assert that node 1 can no longer send a message to node 2 via PubSub
	testOneToKMessagingFails(ctx, h.T(), node1, sub2, topicAfterSpork)
}

func testOneToOneMessagingSucceeds(t *testing.T, sourceNode *Node, dstnAddress NodeAddress) {
	// create stream from node 1 to node 2
	s, err := sourceNode.CreateStream(context.Background(), dstnAddress)
	// assert that stream creation succeeded
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func testOneToOneMessagingFails(t *testing.T, sourceNode *Node, dstnAddress NodeAddress) {
	// create stream from source node to destination address
	_, err := sourceNode.CreateStream(context.Background(), dstnAddress)
	// assert that stream creation failed
	assert.Error(t, err)
	// assert that it failed with the expected error
	assert.Regexp(t, ".*failed to negotiate security protocol.*|.*protocol not supported.*", err)
}

func testOneToKMessagingSucceeds(ctx context.Context,
	t *testing.T,
	sourceNode *Node,
	dstnSub *pubsub.Subscription,
	topic string) {

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
	topic string) {

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
