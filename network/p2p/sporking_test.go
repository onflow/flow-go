package p2p

import (
	"context"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

// SporkingTestSuite tests that the network layer behaves as expected after a spork.
// All network related sporking requirements can be supported via libp2p directly,
// without needing any additional support in Flow other than providing the new root block ID.
// Sporking can be supported by two ways:
// 1. Updating the network key of a node after it is moved from the old chain to the new chain
// 2. Updating the Flow Libp2p protocol ID suffix to prevent one-to-one messaging across sporks and
//    updating the channel suffix to prevent one-to-K messaging (PubSub) across sporks.
// 1 and 2 both can independently ensure that nodes from the old chain cannot communicate with nodes on the new chain
// These tests are just to reconfirm the network behaviour and provide a test bed for future tests for sporking, if needed
type SporkingTestSuite struct {
	suite.Suite
	logger zerolog.Logger
}

func TestSporkingTestSuite(t *testing.T) {
	golog.SetAllLoggers(golog.LevelError)
	suite.Run(t, new(SporkingTestSuite))
}

// TestCrosstalkPreventionOnNetworkKeyChange tests that a node from the old chain cannot talk to a node in the new chain
// if it's network key is updated while the libp2p protocol ID remains the same
func (suite *SporkingTestSuite) TestCrosstalkPreventionOnNetworkKeyChange() {
	// create and start node 1 on localhost and random port
	node1key := generateNetworkingKey(suite.T())
	rootBlockId := unittest.IdentifierFixture()
	node1, id1 := nodeFixture(suite.T(),
		withNetworkingPrivateKey(node1key),
		withRootBlockId(rootBlockId))

	defer stopNode(suite.T(), node1)
	suite.T().Logf(" %s node started on %s", id1.NodeID.String(), id1.Address)
	suite.T().Logf("libp2p ID for %s: %s", node1.id.String(), node1.host.ID())

	// create and start node 2 on localhost and random port
	node2key := generateNetworkingKey(suite.T())
	node2, id2 := nodeFixture(suite.T(),
		withNetworkingPrivateKey(node2key),
		withRootBlockId(rootBlockId))

	peerInfo2, err := PeerAddressInfo(id2)
	require.NoError(suite.T(), err)

	// create stream from node 1 to node 2
	testOneToOneMessagingSucceeds(suite.T(), node1, peerInfo2)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain

	// stop node 2 and start it again with a different networking key but on the same IP and port
	stopNode(suite.T(), node2)

	// start node2 with the same name, ip and port but with the new key
	node2keyNew := generateNetworkingKey(suite.T())
	assert.False(suite.T(), node2key.Equals(node2keyNew))
	node2, id2New := nodeFixture(suite.T(),
		withNetworkingPrivateKey(node2keyNew),
		withRootBlockId(rootBlockId),
		withNetworkingAddress(id2.Address))

	defer stopNode(suite.T(), node2)

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(suite.T(), id2New.Address, id2.Address)

	// attempt to create a stream from node 1 (old chain) to node 2 (new chain)
	// this time it should fail since node 2 is using a different public key
	// (and therefore has a different libp2p node id)
	testOneToOneMessagingFails(suite.T(), node1, peerInfo2)
}

// TestOneToOneCrosstalkPrevention tests that a node from the old chain cannot talk directly to a node in the new chain
// if the Flow libp2p protocol ID is updated while the network keys are kept the same.
func (suite *SporkingTestSuite) TestOneToOneCrosstalkPrevention() {

	// root id before spork
	rootID1 := unittest.IdentifierFixture()

	// create and start node 1 on localhost and random port
	node1key := generateNetworkingKey(suite.T())
	node1, id1 := nodeFixture(suite.T(),
		withNetworkingPrivateKey(node1key),
		withRootBlockId(rootID1))

	defer stopNode(suite.T(), node1)
	peerInfo1, err := PeerAddressInfo(id1)
	require.NoError(suite.T(), err)

	// create and start node 2 on localhost and random port
	node2key := generateNetworkingKey(suite.T())
	node2, id2 := nodeFixture(suite.T(),
		withNetworkingPrivateKey(node2key),
		withRootBlockId(rootID1))

	// create stream from node 2 to node 1
	testOneToOneMessagingSucceeds(suite.T(), node2, peerInfo1)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain

	// stop node 2 and start it again with a different libp2p protocol id to listen for
	stopNode(suite.T(), node2)

	// update the flow root id for node 2. node1 is still listening on the old protocol
	rootID2 := unittest.IdentifierFixture()

	// start node2 with the same address and root key but different root block id
	node2, id2New := nodeFixture(suite.T(),
		withNetworkingPrivateKey(node2key),
		withRootBlockId(rootID2),
		withNetworkingAddress(id2.Address))

	defer stopNode(suite.T(), node2)

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(suite.T(), id2New.Address, id2.Address)

	// attempt to create a stream from node 2 (new chain) to node 1 (old chain)
	// this time it should fail since node 2 is listening on a different protocol
	testOneToOneMessagingFails(suite.T(), node2, peerInfo1)
}

// TestOneToKCrosstalkPrevention tests that a node from the old chain cannot talk to a node in the new chain via PubSub
// if the channel is updated while the network keys are kept the same.
func (suite *SporkingTestSuite) TestOneToKCrosstalkPrevention() {

	// root id before spork
	rootIDBeforeSpork := unittest.IdentifierFixture()

	// create and start node 1 on localhost and random port
	node1key := generateNetworkingKey(suite.T())
	node1, _ := nodeFixture(suite.T(),
		withNetworkingPrivateKey(node1key),
		withRootBlockId(rootIDBeforeSpork))

	defer stopNode(suite.T(), node1)

	// create and start node 2 on localhost and random port with the same root block ID
	node2key := generateNetworkingKey(suite.T())
	node2, id2 := nodeFixture(suite.T(),
		withNetworkingPrivateKey(node2key),
		withRootBlockId(rootIDBeforeSpork))

	pInfo2, err := PeerAddressInfo(id2)
	defer stopNode(suite.T(), node2)
	require.NoError(suite.T(), err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// spork topic is derived by suffixing the channel with the root block ID
	topicBeforeSpork := engine.TopicFromChannel(engine.TestNetwork, rootIDBeforeSpork)

	// both nodes are initially on the same spork and subscribed to the same topic
	_, err = node1.Subscribe(topicBeforeSpork)
	require.NoError(suite.T(), err)
	sub2, err := node2.Subscribe(topicBeforeSpork)
	require.NoError(suite.T(), err)

	// add node 2 as a peer of node 1
	err = node1.AddPeer(ctx, pInfo2)
	require.NoError(suite.T(), err)

	// let the two nodes form the mesh
	time.Sleep(time.Second)

	// assert that node 1 can successfully send a message to node 2 via PubSub
	testOneToKMessagingSucceeds(ctx, suite.T(), node1, sub2, topicBeforeSpork)

	// new root id after spork
	rootIDAfterSpork := unittest.IdentifierFixture()

	// topic after the spork
	topicAfterSpork := engine.TopicFromChannel(engine.TestNetwork, rootIDAfterSpork)

	// mimic that node1 now is now part of the new spork while node2 remains on the old spork
	// by unsubscribing node1 from 'topicBeforeSpork' and subscribing it to 'topicAfterSpork'
	// and keeping node2 subscribed to topic 'topicBeforeSpork'
	err = node1.UnSubscribe(topicBeforeSpork)
	require.NoError(suite.T(), err)
	_, err = node1.Subscribe(topicAfterSpork)
	require.NoError(suite.T(), err)

	// assert that node 1 can no longer send a message to node 2 via PubSub
	testOneToKMessagingFails(ctx, suite.T(), node1, sub2, topicAfterSpork)
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
