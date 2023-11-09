package scoring_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/slices"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSubscriptionProvider_GetSubscribedTopics tests that the SubscriptionProvider returns the correct
// list of topics a peer is subscribed to.
func TestSubscriptionProvider_GetSubscribedTopics(t *testing.T) {
	tp := mockp2p.NewTopicProvider(t)
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// set a low update interval to speed up the test
	cfg.NetworkConfig.SubscriptionProviderConfig.SubscriptionUpdateInterval = 100 * time.Millisecond

	sp, err := scoring.NewSubscriptionProvider(&scoring.SubscriptionProviderConfig{
		Logger: unittest.Logger(),
		TopicProviderOracle: func() p2p.TopicProvider {
			return tp
		},
		Params: &cfg.NetworkConfig.SubscriptionProviderConfig,
	})
	require.NoError(t, err)

	tp.On("GetTopics").Return([]string{"topic1", "topic2", "topic3"}).Maybe()

	peer1 := p2pfixtures.PeerIdFixture(t)
	peer2 := p2pfixtures.PeerIdFixture(t)
	peer3 := p2pfixtures.PeerIdFixture(t)

	// mock peers 1 and 2 subscribed to topic 1 (along with other random peers)
	tp.On("ListPeers", "topic1").Return(append([]peer.ID{peer1, peer2}, p2pfixtures.PeerIdsFixture(t, 10)...))
	// mock peers 2 and 3 subscribed to topic 2 (along with other random peers)
	tp.On("ListPeers", "topic2").Return(append([]peer.ID{peer2, peer3}, p2pfixtures.PeerIdsFixture(t, 10)...))
	// mock peers 1 and 3 subscribed to topic 3 (along with other random peers)
	tp.On("ListPeers", "topic3").Return(append([]peer.ID{peer1, peer3}, p2pfixtures.PeerIdsFixture(t, 10)...))

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unittest.RequireCloseBefore(t, sp.Done(), 1*time.Second, "subscription provider did not stop in time")
	}()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sp.Start(signalerCtx)
	unittest.RequireCloseBefore(t, sp.Ready(), 1*time.Second, "subscription provider did not start in time")

	// As the calls to the TopicProvider are asynchronous, we need to wait for the goroutines to finish.
	assert.Eventually(t, func() bool {
		return slices.AreStringSlicesEqual([]string{"topic1", "topic3"}, sp.GetSubscribedTopics(peer1))
	}, 1*time.Second, 100*time.Millisecond)

	assert.Eventually(t, func() bool {
		return slices.AreStringSlicesEqual([]string{"topic1", "topic2"}, sp.GetSubscribedTopics(peer2))
	}, 1*time.Second, 100*time.Millisecond)

	assert.Eventually(t, func() bool {
		return slices.AreStringSlicesEqual([]string{"topic2", "topic3"}, sp.GetSubscribedTopics(peer3))
	}, 1*time.Second, 100*time.Millisecond)
}
