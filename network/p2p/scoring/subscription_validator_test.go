package scoring_test

import (
	"context"
	"math/rand"
	"regexp"
	"testing"
	"time"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/test"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"

	"github.com/libp2p/go-libp2p/core/peer"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSubscriptionProvider_GetSubscribedTopics tests that when a peer has not subscribed to
// any topic, the subscription validator returns no error.
func TestSubscriptionValidator_NoSubscribedTopic(t *testing.T) {
	sp := mockp2p.NewSubscriptionProvider(t)
	sv := scoring.NewSubscriptionValidator(unittest.Logger(), sp)

	// mocks peer 1 not subscribed to any topic.
	peer1 := unittest.PeerIdFixture(t)
	sp.On("GetSubscribedTopics", peer1).Return([]string{})

	// as peer 1 has not subscribed to any topic, the subscription validator should return no error regardless of the
	// role.
	for _, role := range flow.Roles() {
		require.NoError(t, sv.CheckSubscribedToAllowedTopics(peer1, role))
	}
}

// TestSubscriptionValidator_UnknownChannel tests that when a peer has subscribed to an unknown
// topic, the subscription validator returns an error.
func TestSubscriptionValidator_UnknownChannel(t *testing.T) {
	sp := mockp2p.NewSubscriptionProvider(t)
	sv := scoring.NewSubscriptionValidator(unittest.Logger(), sp)

	// mocks peer 1 not subscribed to an unknown topic.
	peer1 := unittest.PeerIdFixture(t)
	sp.On("GetSubscribedTopics", peer1).Return([]string{"unknown-topic-1", "unknown-topic-2"})

	// as peer 1 has subscribed to unknown topics, the subscription validator should return an error
	// regardless of the role.
	for _, role := range flow.Roles() {
		err := sv.CheckSubscribedToAllowedTopics(peer1, role)
		require.Error(t, err)
		require.True(t, p2p.IsInvalidSubscriptionError(err))
	}
}

// TestSubscriptionValidator_ValidSubscription tests that when a peer has subscribed to valid
// topics based on its Flow protocol role, the subscription validator returns no error.
func TestSubscriptionValidator_ValidSubscriptions(t *testing.T) {
	sp := mockp2p.NewSubscriptionProvider(t)
	sv := scoring.NewSubscriptionValidator(unittest.Logger(), sp)

	for _, role := range flow.Roles() {
		peerId := unittest.PeerIdFixture(t)
		// allowed channels for the role excluding the test channels.
		allowedChannels := channels.ChannelsByRole(role).ExcludePattern(regexp.MustCompile("^(test).*"))
		sporkID := unittest.IdentifierFixture()

		allowedTopics := make([]string, 0, len(allowedChannels))
		for _, channel := range allowedChannels {
			allowedTopics = append(allowedTopics, channels.TopicFromChannel(channel, sporkID).String())
		}

		// peer should pass the subscription validator as it has subscribed to any subset of its allowed topics.
		for i := range allowedTopics {
			sp.On("GetSubscribedTopics", peerId).Return(allowedTopics[:i+1])
			require.NoError(t, sv.CheckSubscribedToAllowedTopics(peerId, role))
		}
	}
}

// TestSubscriptionValidator_SubscribeToAllTopics tests that regardless of its role when a peer has subscribed to all
// topics, the subscription validator returns an error.
//
// Note: this test is to ensure that the subscription validator is not bypassed by subscribing to all topics.
// It also based on the assumption that within the list of all topics, there are guaranteed to be some that are not allowed.
// If this assumption is not true, this test will fail. Hence, the test should be updated accordingly if the assumption
// is no longer true.
func TestSubscriptionValidator_SubscribeToAllTopics(t *testing.T) {
	sp := mockp2p.NewSubscriptionProvider(t)
	sv := scoring.NewSubscriptionValidator(unittest.Logger(), sp)

	allChannels := channels.Channels().ExcludePattern(regexp.MustCompile("^(test).*"))
	sporkID := unittest.IdentifierFixture()
	allTopics := make([]string, 0, len(allChannels))
	for _, channel := range allChannels {
		allTopics = append(allTopics, channels.TopicFromChannel(channel, sporkID).String())
	}

	for _, role := range flow.Roles() {
		peerId := unittest.PeerIdFixture(t)
		sp.On("GetSubscribedTopics", peerId).Return(allTopics)
		err := sv.CheckSubscribedToAllowedTopics(peerId, role)
		require.Error(t, err, role)
		require.True(t, p2p.IsInvalidSubscriptionError(err), role)
	}
}

// TestSubscriptionValidator_InvalidSubscription tests that when a peer has subscribed to invalid
// topics based on its Flow protocol role, the subscription validator returns an error.
func TestSubscriptionValidator_InvalidSubscriptions(t *testing.T) {
	sp := mockp2p.NewSubscriptionProvider(t)
	sv := scoring.NewSubscriptionValidator(unittest.Logger(), sp)

	for _, role := range flow.Roles() {
		peerId := unittest.PeerIdFixture(t)
		unauthorizedChannels := channels.Channels(). // all channels
								ExcludeChannels(channels.ChannelsByRole(role)). // excluding the channels for the role
								ExcludePattern(regexp.MustCompile("^(test).*")) // excluding the test channels.
		sporkID := unittest.IdentifierFixture()
		unauthorizedTopics := make([]string, 0, len(unauthorizedChannels))
		for _, channel := range unauthorizedChannels {
			unauthorizedTopics = append(unauthorizedTopics, channels.TopicFromChannel(channel, sporkID).String())
		}

		// peer should NOT pass subscription validator as it has subscribed to any subset of its unauthorized topics,
		// regardless of the role.
		for i := range unauthorizedTopics {
			sp.On("GetSubscribedTopics", peerId).Return(unauthorizedTopics[:i+1])
			err := sv.CheckSubscribedToAllowedTopics(peerId, role)
			require.Error(t, err, role)
			require.True(t, p2p.IsInvalidSubscriptionError(err), role)
		}
	}
}

// TestSubscriptionValidator_Integration tests that when a peer is subscribed to an invalid topic, it is penalized
// by the subscription validator of other peers on that same (invalid) topic,
// and they prevent the peer from sending messages on that topic.
// This test is an integration test that tests the subscription validator in the context of the pubsub.
//
// Scenario:
// Part-1:
// 1. Two verification nodes and one consensus node are created.
// 2. All nodes subscribe to a legit shared topic (PushBlocks).
// 3. Consensus node publishes a block on this topic.
// 4. Test checks that all nodes receive the block.
//
// Part-2:
// 1. Consensus node subscribes to an invalid topic, which it is not supposed to (RequestChunks).
// 2. Consensus node publishes a block on the PushBlocks topic.
// 3. Test checks that none of the nodes receive the block.
// 4. Verification node also publishes a chunk request on the RequestChunks channel.
// 5. Test checks that consensus node does not receive the chunk request while the other verification node does.
func TestSubscriptionValidator_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// set a low update interval to speed up the test
	cfg.NetworkConfig.GossipSub.SubscriptionProvider.UpdateInterval = 10 * time.Millisecond
	cfg.NetworkConfig.GossipSub.ScoringParameters.AppSpecificScore.ScoreTTL = 10 * time.Millisecond

	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)
	// one consensus node.
	conNode, conId := p2ptest.NodeFixture(t, sporkId, t.Name(),
		idProvider,
		p2ptest.WithLogger(unittest.Logger()),
		p2ptest.OverrideFlowConfig(cfg),
		p2ptest.WithRole(flow.RoleConsensus))

	// two verification node.
	verNode1, verId1 := p2ptest.NodeFixture(t, sporkId, t.Name(),
		idProvider,
		p2ptest.WithLogger(unittest.Logger()),
		p2ptest.OverrideFlowConfig(cfg),
		p2ptest.WithRole(flow.RoleVerification))

	verNode2, verId2 := p2ptest.NodeFixture(t, sporkId, t.Name(),
		idProvider,
		p2ptest.WithLogger(unittest.Logger()),
		p2ptest.OverrideFlowConfig(cfg),
		p2ptest.WithRole(flow.RoleVerification))

	// suppress peer provider error
	peerProvider := func() peer.IDSlice {
		return []peer.ID{conNode.ID(), verNode1.ID(), verNode2.ID()}
	}
	verNode1.WithPeersProvider(peerProvider)
	verNode2.WithPeersProvider(peerProvider)
	conNode.WithPeersProvider(peerProvider)

	ids := flow.IdentityList{&conId, &verId1, &verId2}
	nodes := []p2p.LibP2PNode{conNode, verNode1, verNode2}

	provider := id.NewFixedIdentityProvider(ids)
	idProvider.On("ByPeerID", mocktestify.Anything).Return(
		func(peerId peer.ID) *flow.Identity {
			identity, _ := provider.ByPeerID(peerId)
			return identity
		}, func(peerId peer.ID) bool {
			_, ok := provider.ByPeerID(peerId)
			return ok
		})

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	topicValidator := flowpubsub.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter())

	// wait for the subscriptions to be established
	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// consensus node subscribes to the block topic.
	conSub, err := conNode.Subscribe(blockTopic, topicValidator)
	require.NoError(t, err)

	// both verification nodes subscribe to the blocks and chunks topic (because they are allowed to).
	ver1SubBlocks, err := verNode1.Subscribe(blockTopic, topicValidator)
	require.NoError(t, err)

	ver1SubChunks, err := verNode1.Subscribe(channels.TopicFromChannel(channels.RequestChunks, sporkId), topicValidator)
	require.NoError(t, err)

	ver2SubBlocks, err := verNode2.Subscribe(blockTopic, topicValidator)
	require.NoError(t, err)

	ver2SubChunks, err := verNode2.Subscribe(channels.TopicFromChannel(channels.RequestChunks, sporkId), topicValidator)
	require.NoError(t, err)

	// let the subscriptions be established
	time.Sleep(2 * time.Second)

	outgoingMessageScope, err := message.NewOutgoingScope(
		ids.NodeIDs(),
		channels.TopicFromChannel(channels.PushBlocks, sporkId),
		unittest.ProposalFixture(),
		unittest.NetworkCodec().Encode,
		message.ProtocolTypePubSub)
	require.NoError(t, err)
	require.NoError(t, conNode.Publish(ctx, outgoingMessageScope))

	// checks that the message is received by all nodes.
	ctx1s, cancel1s := context.WithTimeout(ctx, 1*time.Second)
	defer cancel1s()

	expectedReceivedData, err := outgoingMessageScope.Proto().Marshal()
	require.NoError(t, err)

	p2pfixtures.SubsMustReceiveMessage(t, ctx1s, expectedReceivedData, []p2p.Subscription{conSub, ver1SubBlocks, ver2SubBlocks})

	// now consensus node is doing something very bad!
	// it is subscribing to a channel that it is not supposed to subscribe to.
	conSubChunks, err := conNode.Subscribe(channels.TopicFromChannel(channels.RequestChunks, sporkId), topicValidator)
	require.NoError(t, err)

	// let's wait for a bit to subscription propagate.
	time.Sleep(5 * time.Second)

	// consensus node publishes another proposal, but this time, it should not reach verification node.
	// since upon an unauthorized subscription, verification node should have slashed consensus node on
	// the GossipSub scoring protocol.
	outgoingMessageScope, err = message.NewOutgoingScope(
		ids.NodeIDs(),
		channels.TopicFromChannel(channels.PushBlocks, sporkId),
		unittest.ProposalFixture(),
		unittest.NetworkCodec().Encode,
		message.ProtocolTypePubSub)
	require.NoError(t, err)
	require.NoError(t, conNode.Publish(ctx, outgoingMessageScope))

	ctx5s, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	p2pfixtures.SubsMustNeverReceiveAnyMessage(t, ctx5s, []p2p.Subscription{ver1SubBlocks, ver2SubBlocks})

	// moreover, a verification node publishing a message to the request chunk topic should not reach consensus node.
	// however, both verification nodes should receive the message.
	outgoingMessageScope, err = message.NewOutgoingScope(
		ids.NodeIDs(),
		channels.TopicFromChannel(channels.RequestChunks, sporkId),
		&messages.ChunkDataRequest{
			ChunkID: unittest.IdentifierFixture(),
			Nonce:   rand.Uint64(),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypePubSub)
	require.NoError(t, err)
	require.NoError(t, verNode1.Publish(ctx, outgoingMessageScope))

	ctx1s, cancel1s = context.WithTimeout(ctx, 1*time.Second)
	defer cancel1s()

	expectedReceivedData, err = outgoingMessageScope.Proto().Marshal()
	require.NoError(t, err)

	p2pfixtures.SubsMustReceiveMessage(t, ctx1s, expectedReceivedData, []p2p.Subscription{ver1SubChunks, ver2SubChunks})

	ctx5s, cancel5s = context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	p2pfixtures.SubsMustNeverReceiveAnyMessage(t, ctx5s, []p2p.Subscription{conSubChunks})
}
