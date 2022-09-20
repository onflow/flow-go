package scoring_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/p2p/internal/p2pfixtures"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
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
