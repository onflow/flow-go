package internal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewSubscriptionRecordCache tests that NewSubscriptionRecordCache returns a valid cache.
func TestNewSubscriptionRecordCache(t *testing.T) {
	sizeLimit := uint32(100)

	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory()))

	require.NotNil(t, cache, "cache should not be nil")
	require.IsType(t, &internal.SubscriptionRecordCache{}, cache, "cache should be of type *SubscriptionRecordCache")
}

// TestGetSubscribedTopics tests the retrieval of subscribed topics for a peer.
func TestGetSubscribedTopics(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory()))

	// create a dummy peer ID
	peerID := unittest.PeerIdFixture(t)

	// case when the peer has a subscription
	topics := []string{"topic1", "topic2"}
	updatedTopics, err := cache.AddTopicForPeer(peerID, topics[0])
	require.NoError(t, err, "adding topic 1 should not produce an error")
	require.Equal(t, topics[:1], updatedTopics, "updated topics should match the added topic")
	updatedTopics, err = cache.AddTopicForPeer(peerID, topics[1])
	require.NoError(t, err, "adding topic 2 should not produce an error")
	require.Equal(t, topics, updatedTopics, "updated topics should match the added topic")

	retrievedTopics, found := cache.GetSubscribedTopics(peerID)
	require.True(t, found, "peer should be found")
	require.ElementsMatch(t, topics, retrievedTopics, "retrieved topics should match the added topics")

	// case when the peer does not have a subscription
	nonExistentPeerID := unittest.PeerIdFixture(t)
	retrievedTopics, found = cache.GetSubscribedTopics(nonExistentPeerID)
	require.False(t, found, "non-existent peer should not be found")
	require.Nil(t, retrievedTopics, "retrieved topics for non-existent peer should be nil")
}
