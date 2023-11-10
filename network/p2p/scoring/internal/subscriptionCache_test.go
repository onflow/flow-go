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

// TestMoveToNextUpdateCycle tests the increment of update cycles in SubscriptionRecordCache.
// The first increment should set the cycle to 1, and the second increment should set the cycle to 2.
func TestMoveToNextUpdateCycle(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory()))

	// initial cycle should be 0, so first increment sets it to 1
	firstCycle := cache.MoveToNextUpdateCycle()
	require.Equal(t, uint64(1), firstCycle, "first cycle should be 1 after first increment")

	// increment cycle again and verify it's now 2
	secondCycle := cache.MoveToNextUpdateCycle()
	require.Equal(t, uint64(2), secondCycle, "second cycle should be 2 after second increment")
}

// TestAddTopicForPeer tests adding a topic for a peer.
func TestAddTopicForPeer(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory()))

	// case when adding a topic to an existing peer
	existingPeerID := unittest.PeerIdFixture(t)
	firstTopic := "topic1"
	secondTopic := "topic2"

	// add first topic to the existing peer
	_, err := cache.AddTopicForPeer(existingPeerID, firstTopic)
	require.NoError(t, err, "adding first topic to existing peer should not produce an error")

	// add second topic to the same peer
	updatedTopics, err := cache.AddTopicForPeer(existingPeerID, secondTopic)
	require.NoError(t, err, "adding second topic to existing peer should not produce an error")
	require.ElementsMatch(t, []string{firstTopic, secondTopic}, updatedTopics, "updated topics should match the added topics")

	// case when adding a topic to a new peer
	newPeerID := unittest.PeerIdFixture(t)
	newTopic := "newTopic"

	// add a topic to the new peer
	updatedTopics, err = cache.AddTopicForPeer(newPeerID, newTopic)
	require.NoError(t, err, "adding topic to new peer should not produce an error")
	require.Equal(t, []string{newTopic}, updatedTopics, "updated topics for new peer should match the added topic")

	// sanity check that the topics for existing peer are still the same
	retrievedTopics, found := cache.GetSubscribedTopics(existingPeerID)
	require.True(t, found, "existing peer should be found")
	require.ElementsMatch(t, []string{firstTopic, secondTopic}, retrievedTopics, "retrieved topics should match the added topics")
}

// TestDuplicateTopic tests adding a duplicate topic for a peer. The duplicate topic should not be added.
func TestDuplicateTopics(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory()))

	peerID := unittest.PeerIdFixture(t)
	topic := "topic1"

	// add first topic to the existing peer
	_, err := cache.AddTopicForPeer(peerID, topic)
	require.NoError(t, err, "adding first topic to existing peer should not produce an error")

	// add second topic to the same peer
	updatedTopics, err := cache.AddTopicForPeer(peerID, topic)
	require.NoError(t, err, "adding duplicate topic to existing peer should not produce an error")
	require.Equal(t, []string{topic}, updatedTopics, "duplicate topic should not be added")
}
