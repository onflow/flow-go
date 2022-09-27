package scoring_test

import (
	"context"
	"math/rand"
	"regexp"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/internal/p2pfixtures"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSubscriptionProvider_GetSubscribedTopics tests that when a peer has not subscribed to
// any topic, the subscription validator returns no error.
func TestSubscriptionValidator_NoSubscribedTopic(t *testing.T) {
	idProvider := mock.NewIdentityProvider(t)
	sp := mockp2p.NewSubscriptionProvider(t)

	sv := scoring.NewSubscriptionValidator(idProvider)
	sv.RegisterSubscriptionProvider(sp)

	// mocks peer 1 not subscribed to any topic.
	peer1 := p2pfixtures.PeerIdFixture(t)
	idProvider.On("ByPeerID", peer1).Return(unittest.IdentityFixture(), true)
	sp.On("GetSubscribedTopics", peer1).Return([]string{})

	// as peer 1 has not subscribed to any topic, the subscription validator should return no error.
	require.NoError(t, sv.MustSubscribedToAllowedTopics(peer1))
}

// TestSubscriptionValidator_UnknownChannel tests that when a peer has subscribed to an unknown
// topic, the subscription validator returns an error.
func TestSubscriptionValidator_UnknownChannel(t *testing.T) {
	idProvider := mock.NewIdentityProvider(t)
	sp := mockp2p.NewSubscriptionProvider(t)

	sv := scoring.NewSubscriptionValidator(idProvider)
	sv.RegisterSubscriptionProvider(sp)

	// mocks peer 1 not subscribed to an unknown topic.
	peer1 := p2pfixtures.PeerIdFixture(t)
	idProvider.On("ByPeerID", peer1).Return(unittest.IdentityFixture(), true)
	sp.On("GetSubscribedTopics", peer1).Return([]string{"unknown-topic-1", "unknown-topic-2"})

	// as peer 1 has subscribed to unknown topics, the subscription validator should return an error.
	require.Error(t, sv.MustSubscribedToAllowedTopics(peer1))
}

// TestSubscriptionValidator_ValidSubscription tests that when a peer has subscribed to valid
// topics based on its Flow protocol role, the subscription validator returns no error.
func TestSubscriptionValidator_ValidSubscriptions(t *testing.T) {
	idProvider := mock.NewIdentityProvider(t)
	sp := mockp2p.NewSubscriptionProvider(t)

	sv := scoring.NewSubscriptionValidator(idProvider)
	sv.RegisterSubscriptionProvider(sp)

	for _, role := range flow.Roles() {
		peer := p2pfixtures.PeerIdFixture(t)
		// allowed channels for the role excluding the test channels.
		allowedChannels := channels.ChannelsByRole(role).ExcludePattern(regexp.MustCompile("^(test).*"))
		sporkID := unittest.IdentifierFixture()

		allowedTopics := make([]string, 0, len(allowedChannels))
		for _, channel := range allowedChannels {
			allowedTopics = append(allowedTopics, channels.TopicFromChannel(channel, sporkID).String())
		}

		// peer should pass the subscription validator as it has subscribed to any subset of its allowed topics.
		for i := range allowedTopics {
			idProvider.On("ByPeerID", peer).Return(unittest.IdentityFixture(unittest.WithRole(role)), true)
			sp.On("GetSubscribedTopics", peer).Return(allowedTopics[:i+1])
			require.NoError(t, sv.MustSubscribedToAllowedTopics(peer))
		}
	}
}

// TestSubscriptionValidator_SubscribeToAllTopics tests that regardless of its role when a peer has subscribed to all
// topics, the subscription validator returns an error.
func TestSubscriptionValidator_SubscribeToAllTopics(t *testing.T) {
	idProvider := mock.NewIdentityProvider(t)
	sp := mockp2p.NewSubscriptionProvider(t)

	sv := scoring.NewSubscriptionValidator(idProvider)
	sv.RegisterSubscriptionProvider(sp)

	allChannels := channels.Channels().ExcludePattern(regexp.MustCompile("^(test).*"))
	sporkID := unittest.IdentifierFixture()
	allTopics := make([]string, 0, len(allChannels))
	for _, channel := range allChannels {
		allTopics = append(allTopics, channels.TopicFromChannel(channel, sporkID).String())
	}

	for _, role := range flow.Roles() {
		peer := p2pfixtures.PeerIdFixture(t)
		idProvider.On("ByPeerID", peer).Return(unittest.IdentityFixture(unittest.WithRole(role)), true)
		sp.On("GetSubscribedTopics", peer).Return(allTopics)
		require.Error(t, sv.MustSubscribedToAllowedTopics(peer), role)
	}
}

func TestSubscriptionValidator_InvalidSubscriptions(t *testing.T) {
	idProvider := mock.NewIdentityProvider(t)
	sp := mockp2p.NewSubscriptionProvider(t)

	sv := scoring.NewSubscriptionValidator(idProvider)
	sv.RegisterSubscriptionProvider(sp)

	for _, role := range flow.Roles() {
		peer := p2pfixtures.PeerIdFixture(t)
		unauthorizedChannels := channels.Channels(). // all channels
								ExcludeChannels(channels.ChannelsByRole(role)). // excluding the channels for the role
								ExcludePattern(regexp.MustCompile("^(test).*")) // excluding the test channels.
		sporkID := unittest.IdentifierFixture()
		unauthorizedTopics := make([]string, 0, len(unauthorizedChannels))
		for _, channel := range unauthorizedChannels {
			unauthorizedTopics = append(unauthorizedTopics, channels.TopicFromChannel(channel, sporkID).String())
		}

		// peer should NOT pass subscription validator as it has subscribed to any subset of its unauthorized topics.
		for i := range unauthorizedTopics {
			idProvider.On("ByPeerID", peer).Return(unittest.IdentityFixture(unittest.WithRole(role)), true)
			sp.On("GetSubscribedTopics", peer).Return(unauthorizedTopics[:i+1])
			require.Error(t, sv.MustSubscribedToAllowedTopics(peer))
		}
	}
}

func TestLibP2PSubscriptionValidator(t *testing.T) {
	ctx := context.Background()
	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)
	// one consensus node.
	conNode, conId := p2pfixtures.NodeFixture(t, ctx, sporkId, t.Name(),
		p2pfixtures.WithLogger(unittest.Logger()),
		p2pfixtures.WithPeerScoringEnabled(idProvider),
		p2pfixtures.WithRole(flow.RoleConsensus))

	// two verification node.
	verNode1, verId1 := p2pfixtures.NodeFixture(t, ctx, sporkId, t.Name(),
		p2pfixtures.WithLogger(unittest.Logger()),
		p2pfixtures.WithPeerScoringEnabled(idProvider),
		p2pfixtures.WithRole(flow.RoleVerification))

	verNode2, verId2 := p2pfixtures.NodeFixture(t, ctx, sporkId, t.Name(),
		p2pfixtures.WithLogger(unittest.Logger()),
		p2pfixtures.WithPeerScoringEnabled(idProvider),
		p2pfixtures.WithRole(flow.RoleVerification))

	ids := flow.IdentityList{&conId, &verId1, &verId2}
	nodes := []*p2pnode.Node{conNode, verNode1, verNode2}

	provider := id.NewFixedIdentityProvider(ids)
	idProvider.On("ByPeerID", mocktestify.Anything).Return(
		func(peerId peer.ID) *flow.Identity {
			identity, _ := provider.ByPeerID(peerId)
			return identity
		}, func(peerId peer.ID) bool {
			_, ok := provider.ByPeerID(peerId)
			return ok
		})

	defer p2pfixtures.StopNodes(t, nodes)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector())

	// consensus node subscribes to the block topic.
	conSub, err := conNode.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// both verification nodes subscribe to the blocks and chunks topic (because they are allowed to).
	ver1SubBlocks, err := verNode1.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	ver1SubChunks, err := verNode1.Subscribe(channels.TopicFromChannel(channels.RequestChunks, sporkId), unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	ver2SubBlocks, err := verNode2.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	ver2SubChunks, err := verNode2.Subscribe(channels.TopicFromChannel(channels.RequestChunks, sporkId), unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// wait for the subscriptions to be established
	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	proposalMsg := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture())
	// consensus node publishes a proposal
	require.NoError(t, conNode.Publish(ctx, blockTopic, proposalMsg))

	// checks that the message is received by all nodes.
	ctx1s, _ := context.WithTimeout(ctx, 1*time.Second)
	p2pfixtures.SubsMustReceiveMessage(t, ctx1s, proposalMsg, []*pubsub.Subscription{conSub, ver1SubBlocks, ver2SubBlocks})

	// now consensus node is doing something very bad!
	// it is subscribing to a channel that it is not supposed to subscribe to.
	conSubChunks, err := conNode.Subscribe(channels.TopicFromChannel(channels.RequestChunks, sporkId), unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// let's wait for a bit to subscription propagate.
	time.Sleep(5 * time.Second)

	// consensus node publishes another proposal, but this time, it should not reach verification node.
	// since upon an unauthorized subscription, verification node should have slashed consensus node on
	// the GossipSub scoring protocol.
	proposalMsg = p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture())
	// publishes a message to the topic.
	require.NoError(t, conNode.Publish(ctx, blockTopic, proposalMsg))

	ctx5s, _ := context.WithTimeout(ctx, 5*time.Second)
	p2pfixtures.SubsMustNeverReceiveMessage(t, ctx5s, []*pubsub.Subscription{ver1SubBlocks, ver2SubBlocks})

	// moreover, a verification node publishing a message to the request chunk topic should not reach consensus node.
	// however, both verification nodes should receive the message.
	chunkDataPackRequestMsg := p2pfixtures.MustEncodeEvent(t, &messages.ChunkDataRequest{
		ChunkID: unittest.IdentifierFixture(),
		Nonce:   rand.Uint64(),
	})
	require.NoError(t, verNode1.Publish(ctx, channels.TopicFromChannel(channels.RequestChunks, sporkId), chunkDataPackRequestMsg))

	ctx1s, _ = context.WithTimeout(ctx, 1*time.Second)
	p2pfixtures.SubsMustReceiveMessage(t, ctx1s, chunkDataPackRequestMsg, []*pubsub.Subscription{ver1SubChunks, ver2SubChunks})

	ctx5s, _ = context.WithTimeout(ctx, 5*time.Second)
	p2pfixtures.SubsMustNeverReceiveMessage(t, ctx5s, []*pubsub.Subscription{conSubChunks})
}
