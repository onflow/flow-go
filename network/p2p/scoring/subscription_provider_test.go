package scoring_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
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
	idProvider := mock.NewIdentityProvider(t)

	// set a low update interval to speed up the test
	cfg.NetworkConfig.SubscriptionProvider.SubscriptionUpdateInterval = 100 * time.Millisecond

	sp, err := scoring.NewSubscriptionProvider(&scoring.SubscriptionProviderConfig{
		Logger: unittest.Logger(),
		TopicProviderOracle: func() p2p.TopicProvider {
			return tp
		},
		Params:                  &cfg.NetworkConfig.SubscriptionProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		IdProvider:              idProvider,
	})
	require.NoError(t, err)

	tp.On("GetTopics").Return([]string{"topic1", "topic2", "topic3"}).Maybe()

	peer1 := unittest.PeerIdFixture(t)
	peer2 := unittest.PeerIdFixture(t)
	peer3 := unittest.PeerIdFixture(t)

	idProvider.On("ByPeerID", mockery.Anything).Return(unittest.IdentityFixture(), true).Maybe()

	// mock peers 1 and 2 subscribed to topic 1 (along with other random peers)
	tp.On("ListPeers", "topic1").Return(append([]peer.ID{peer1, peer2}, unittest.PeerIdFixtures(t, 10)...))
	// mock peers 2 and 3 subscribed to topic 2 (along with other random peers)
	tp.On("ListPeers", "topic2").Return(append([]peer.ID{peer2, peer3}, unittest.PeerIdFixtures(t, 10)...))
	// mock peers 1 and 3 subscribed to topic 3 (along with other random peers)
	tp.On("ListPeers", "topic3").Return(append([]peer.ID{peer1, peer3}, unittest.PeerIdFixtures(t, 10)...))

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

// TestSubscriptionProvider_GetSubscribedTopics_SkippingUnknownPeers tests that the SubscriptionProvider skips
// unknown peers when returning the list of topics a peer is subscribed to. In other words, if a peer is unknown,
// the SubscriptionProvider should not keep track of its subscriptions.
func TestSubscriptionProvider_GetSubscribedTopics_SkippingUnknownPeers(t *testing.T) {
	tp := mockp2p.NewTopicProvider(t)
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	idProvider := mock.NewIdentityProvider(t)

	// set a low update interval to speed up the test
	cfg.NetworkConfig.SubscriptionProvider.SubscriptionUpdateInterval = 100 * time.Millisecond

	sp, err := scoring.NewSubscriptionProvider(&scoring.SubscriptionProviderConfig{
		Logger: unittest.Logger(),
		TopicProviderOracle: func() p2p.TopicProvider {
			return tp
		},
		Params:                  &cfg.NetworkConfig.SubscriptionProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		IdProvider:              idProvider,
	})
	require.NoError(t, err)

	tp.On("GetTopics").Return([]string{"topic1", "topic2", "topic3"}).Maybe()

	peer1 := unittest.PeerIdFixture(t)
	peer2 := unittest.PeerIdFixture(t)
	peer3 := unittest.PeerIdFixture(t)

	// mock peers 1 and 2 as a known peer; peer 3 as an unknown peer
	idProvider.On("ByPeerID", mockery.Anything).
		Return(func(pid peer.ID) *flow.Identity {
			if pid == peer1 || pid == peer2 {
				return unittest.IdentityFixture()
			}
			return nil
		}, func(pid peer.ID) bool {
			if pid == peer1 || pid == peer2 {
				return true
			}
			return false
		}).Maybe()

	// mock peers 1 and 2 subscribed to topic 1 (along with other random peers)
	tp.On("ListPeers", "topic1").Return(append([]peer.ID{peer1, peer2}, unittest.PeerIdFixtures(t, 10)...))
	// mock peers 2 and 3 subscribed to topic 2 (along with other random peers)
	tp.On("ListPeers", "topic2").Return(append([]peer.ID{peer2, peer3}, unittest.PeerIdFixtures(t, 10)...))
	// mock peers 1 and 3 subscribed to topic 3 (along with other random peers)
	tp.On("ListPeers", "topic3").Return(append([]peer.ID{peer1, peer3}, unittest.PeerIdFixtures(t, 10)...))

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unittest.RequireCloseBefore(t, sp.Done(), 1*time.Second, "subscription provider did not stop in time")
	}()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sp.Start(signalerCtx)
	unittest.RequireCloseBefore(t, sp.Ready(), 1*time.Second, "subscription provider did not start in time")

	// As the calls to the TopicProvider are asynchronous, we need to wait for the goroutines to finish.
	// peer 1 should be eventually subscribed to topic 1 and topic 3; while peer 3 should not have any subscriptions record since it is unknown
	assert.Eventually(t, func() bool {
		return slices.AreStringSlicesEqual([]string{"topic1", "topic3"}, sp.GetSubscribedTopics(peer1)) &&
			slices.AreStringSlicesEqual([]string{}, sp.GetSubscribedTopics(peer3))
	}, 1*time.Second, 100*time.Millisecond)

	// peer 2 should be eventually subscribed to topic 1 and topic 2; while peer 3 should not have any subscriptions record since it is unknown
	assert.Eventually(t, func() bool {
		return slices.AreStringSlicesEqual([]string{"topic1", "topic2"}, sp.GetSubscribedTopics(peer2)) &&
			slices.AreStringSlicesEqual([]string{}, sp.GetSubscribedTopics(peer3))
	}, 1*time.Second, 100*time.Millisecond)
}
