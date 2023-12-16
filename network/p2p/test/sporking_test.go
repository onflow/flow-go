package p2ptest_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/message"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/test"

	"github.com/onflow/flow-go/network/p2p/utils"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"
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
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx1, cancel1 := context.WithCancel(ctx)
	signalerCtx1 := irrecoverable.NewMockSignalerContext(t, ctx1)

	ctx2, cancel2 := context.WithCancel(ctx)
	signalerCtx2 := irrecoverable.NewMockSignalerContext(t, ctx2)

	ctx2a, cancel2a := context.WithCancel(ctx)
	signalerCtx2a := irrecoverable.NewMockSignalerContext(t, ctx2a)

	// create and start node 1 on localhost and random port
	node1key := p2pfixtures.NetworkingKeyFixtures(t)
	sporkId := unittest.IdentifierFixture()

	node1, id1 := p2ptest.NodeFixture(t,
		sporkId,
		"test_crosstalk_prevention_on_network_key_change",
		idProvider,
		p2ptest.WithNetworkingPrivateKey(node1key),
	)
	idProvider.SetIdentities(flow.IdentityList{&id1})

	p2ptest.StartNode(t, signalerCtx1, node1)
	defer p2ptest.StopNode(t, node1, cancel1)

	t.Logf(" %s node started on %s", id1.NodeID.String(), id1.Address)
	t.Logf("libp2p ID for %s: %s", id1.NodeID.String(), node1.ID())

	// create and start node 2 on localhost and random port
	node2key := p2ptest.NetworkingKeyFixtures(t)
	node2, id2 := p2ptest.NodeFixture(t,
		sporkId,
		"test_crosstalk_prevention_on_network_key_change",
		idProvider,
		p2ptest.WithNetworkingPrivateKey(node2key),
	)
	idProvider.SetIdentities(flow.IdentityList{&id1, &id2})

	p2ptest.StartNode(t, signalerCtx2, node2)

	peerInfo2, err := utils.PeerAddressInfo(id2)
	require.NoError(t, err)

	// create stream from node 1 to node 2
	node1.Host().Peerstore().AddAddrs(peerInfo2.ID, peerInfo2.Addrs, peerstore.AddressTTL)
	err = node1.OpenAndWriteOnStream(context.Background(), peerInfo2.ID, t.Name(), func(stream network.Stream) error {
		require.NotNil(t, stream)
		return nil
	})
	require.NoError(t, err)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain
	// stop node 2 and start it again with a different networking key but on the same IP and port
	p2ptest.StopNode(t, node2, cancel2)

	// start node2 with the same name, ip and port but with the new key
	node2keyNew := p2pfixtures.NetworkingKeyFixtures(t)
	assert.False(t, node2key.Equals(node2keyNew))
	node2, id2New := p2ptest.NodeFixture(t,
		sporkId,
		"test_crosstalk_prevention_on_network_key_change",
		idProvider,
		p2ptest.WithNetworkingPrivateKey(node2keyNew),
		p2ptest.WithNetworkingAddress(id2.Address),
	)
	idProvider.SetIdentities(flow.IdentityList{&id1, &id2New})

	p2ptest.StartNode(t, signalerCtx2a, node2)
	defer p2ptest.StopNode(t, node2, cancel2a)

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
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx1, cancel1 := context.WithCancel(ctx)
	signalerCtx1 := irrecoverable.NewMockSignalerContext(t, ctx1)

	ctx2, cancel2 := context.WithCancel(ctx)
	signalerCtx2 := irrecoverable.NewMockSignalerContext(t, ctx2)

	ctx2a, cancel2a := context.WithCancel(ctx)
	signalerCtx2a := irrecoverable.NewMockSignalerContext(t, ctx2a)

	sporkId1 := unittest.IdentifierFixture()

	// create and start node 1 on localhost and random port
	node1, id1 := p2ptest.NodeFixture(t, sporkId1, "test_one_to_one_crosstalk_prevention", idProvider)

	p2ptest.StartNode(t, signalerCtx1, node1)
	defer p2ptest.StopNode(t, node1, cancel1)

	peerInfo1, err := utils.PeerAddressInfo(id1)
	require.NoError(t, err)

	// create and start node 2 on localhost and random port
	node2, id2 := p2ptest.NodeFixture(t, sporkId1, "test_one_to_one_crosstalk_prevention", idProvider)

	idProvider.SetIdentities(flow.IdentityList{&id1, &id2})
	p2ptest.StartNode(t, signalerCtx2, node2)

	// create stream from node 1 to node 2
	node2.Host().Peerstore().AddAddrs(peerInfo1.ID, peerInfo1.Addrs, peerstore.AddressTTL)
	err = node2.OpenAndWriteOnStream(context.Background(), peerInfo1.ID, t.Name(), func(stream network.Stream) error {
		assert.NotNil(t, stream)
		return nil
	})
	require.NoError(t, err)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain
	// stop node 2 and start it again with a different libp2p protocol id to listen for
	p2ptest.StopNode(t, node2, cancel2)

	// start node2 with the same address and root key but different root block id
	node2, id2New := p2ptest.NodeFixture(t,
		unittest.IdentifierFixture(), // update the flow root id for node 2. node1 is still listening on the old protocol
		"test_one_to_one_crosstalk_prevention",
		idProvider,
		p2ptest.WithNetworkingAddress(id2.Address),
	)
	idProvider.SetIdentities(flow.IdentityList{&id1, &id2New})

	p2ptest.StartNode(t, signalerCtx2a, node2)
	defer p2ptest.StopNode(t, node2, cancel2a)

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(t, id2New.Address, id2.Address)

	// attempt to create a stream from node 2 (new chain) to node 1 (old chain)
	// this time it should fail since node 2 is listening on a different protocol
	testOneToOneMessagingFails(t, node2, peerInfo1)
}

// TestOneToKCrosstalkPrevention tests that a node from the old chain cannot talk to a node in the new chain via PubSub
// if the channel is updated while the network keys are kept the same.
func TestOneToKCrosstalkPrevention(t *testing.T) {
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx1, cancel1 := context.WithCancel(ctx)
	signalerCtx1 := irrecoverable.NewMockSignalerContext(t, ctx1)

	ctx2, cancel2 := context.WithCancel(ctx)
	signalerCtx2 := irrecoverable.NewMockSignalerContext(t, ctx2)

	// root id before spork
	previousSporkId := unittest.IdentifierFixture()

	// create and start node 1 on localhost and random port
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// cross-talk prevention is intrinsically tied to how we encode topics, peer scoring adds another layer of protection by preventing unknown identifiers
	// from joining the mesh. As this test simulates the scenario where a node is moved from the old chain to the new chain, we disable peer scoring
	// to allow the node to join the mesh on the new chain, otherwise the node will be disconnected from the mesh due to peer scoring penalty for unknown identifiers.
	cfg.NetworkConfig.GossipSub.PeerScoringEnabled = false
	node1, id1 := p2ptest.NodeFixture(t,
		previousSporkId,
		"test_one_to_k_crosstalk_prevention",
		idProvider,
		p2ptest.OverrideFlowConfig(cfg),
	)

	p2ptest.StartNode(t, signalerCtx1, node1)
	defer p2ptest.StopNode(t, node1, cancel1)
	idProvider.SetIdentities(flow.IdentityList{&id1})

	// create and start node 2 on localhost and random port with the same root block ID
	node2, id2 := p2ptest.NodeFixture(t,
		previousSporkId,
		"test_one_to_k_crosstalk_prevention",
		idProvider,
	)

	p2ptest.StartNode(t, signalerCtx2, node2)
	defer p2ptest.StopNode(t, node2, cancel2)

	pInfo2, err := utils.PeerAddressInfo(id2)
	require.NoError(t, err)

	// spork topic is derived by suffixing the channel with the root block ID
	topicBeforeSpork := channels.TopicFromChannel(channels.TestNetworkChannel, previousSporkId)

	logger := unittest.Logger()
	topicValidator := flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter())

	// both nodes are initially on the same spork and subscribed to the same topic
	_, err = node1.Subscribe(topicBeforeSpork, topicValidator)
	require.NoError(t, err)
	sub2, err := node2.Subscribe(topicBeforeSpork, topicValidator)
	require.NoError(t, err)

	// add node 2 as a peer of node 1
	err = node1.ConnectToPeer(ctx, pInfo2)
	require.NoError(t, err)

	// let the two nodes form the mesh
	time.Sleep(time.Second)

	// assert that node 1 can successfully send a message to node 2 via PubSub
	outgoingMessageScope, err := message.NewOutgoingScope(
		flow.IdentifierList{unittest.IdentifierFixture()},
		topicBeforeSpork,
		&libp2pmessage.TestMessage{
			Text: string("hello"),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypePubSub)
	require.NoError(t, err)

	expectedReceivedData, err := outgoingMessageScope.Proto().Marshal()
	require.NoError(t, err)

	// send a 1-k message from source node to destination node
	err = node1.Publish(ctx, outgoingMessageScope)
	require.NoError(t, err)

	// assert that the message is received by the destination node
	unittest.AssertReturnsBefore(t, func() {
		msg, err := sub2.Next(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedReceivedData, msg.Data)
	},
		// libp2p hearbeats every second, so at most the message should take 1 second
		2*time.Second)

	// new root id after spork
	rootIDAfterSpork := unittest.IdentifierFixture()

	// topic after the spork
	topicAfterSpork := channels.TopicFromChannel(channels.TestNetworkChannel, rootIDAfterSpork)

	// mimic that node1 is now part of the new spork while node2 remains on the old spork
	// by unsubscribing node1 from 'topicBeforeSpork' and subscribing it to 'topicAfterSpork'
	// and keeping node2 subscribed to topic 'topicBeforeSpork'
	err = node1.Unsubscribe(topicBeforeSpork)
	require.NoError(t, err)
	_, err = node1.Subscribe(topicAfterSpork, topicValidator)
	require.NoError(t, err)

	// assert that node 1 can no longer send a message to node 2 via PubSub
	outgoingMessageScope, err = message.NewOutgoingScope(
		flow.IdentifierList{id2.NodeID},
		topicAfterSpork,
		&libp2pmessage.TestMessage{
			Text: string("hello"),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypePubSub)
	require.NoError(t, err)

	// send a 1-k message from source node to destination node
	err = node1.Publish(ctx, outgoingMessageScope)
	require.NoError(t, err)

	// assert that the message is never received by the destination node
	_ = unittest.RequireNeverReturnBefore(t, func() {
		_, _ = sub2.Next(ctx)
	},
		// libp2p hearbeats every second, so at most the message should take 1 second
		2*time.Second,
		"nodes on different sporks were able to communicate")
}

func testOneToOneMessagingFails(t *testing.T, sourceNode p2p.LibP2PNode, peerInfo peer.AddrInfo) {
	// create stream from source node to destination address
	sourceNode.Host().Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.AddressTTL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := sourceNode.OpenAndWriteOnStream(ctx, peerInfo.ID, t.Name(), func(stream network.Stream) error {
		// this callback should never be called
		assert.Fail(t, "stream creation should have failed")
		return nil
	})
	// assert that stream creation failed
	require.Error(t, err)
	// assert that it failed with the expected error
	assert.Regexp(t, ".*failed to negotiate security protocol.*|.*protocols not supported.*", err)
}
