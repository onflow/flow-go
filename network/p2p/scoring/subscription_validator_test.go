package scoring_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
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
	dhtPrefix := "subscription-validator-test"

	idProvider := mock.NewIdentityProvider(t)

	node1, _ := p2pfixtures.NodeFixture(t, ctx, sporkId, dhtPrefix, p2pfixtures.WithPeerScoringEnabled(idProvider))

	time.Sleep(1 * time.Second)

	p2pfixtures.StopNodes(t, []*p2pnode.Node{node1})
}
