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

// TestMoveUpdateCycle tests that (1) within one update cycle, "AddTopicForPeer" calls append the topics to the list of
// subscribed topics for peer, (2) as long as there is no "AddTopicForPeer" call, moving to the next update cycle
// does not change the subscribed topics for a peer, and (3) calling "AddTopicForPeer" after moving to the next update
// cycle clears the subscribed topics for a peer and adds the new topic.
func TestMoveUpdateCycle(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory()))

	peerID := unittest.PeerIdFixture(t)
	topic1 := "topic1"
	topic2 := "topic2"
	topic3 := "topic3"
	topic4 := "topic4"

	// adds topic1, topic2, and topic3 to the peer
	topics, err := cache.AddTopicForPeer(peerID, topic1)
	require.NoError(t, err, "adding first topic to existing peer should not produce an error")
	require.Equal(t, []string{topic1}, topics, "updated topics should match the added topic")
	topics, err = cache.AddTopicForPeer(peerID, topic2)
	require.NoError(t, err, "adding second topic to existing peer should not produce an error")
	require.Equal(t, []string{topic1, topic2}, topics, "updated topics should match the added topics")
	topics, err = cache.AddTopicForPeer(peerID, topic3)
	require.NoError(t, err, "adding third topic to existing peer should not produce an error")
	require.Equal(t, []string{topic1, topic2, topic3}, topics, "updated topics should match the added topics")

	// move to next update cycle
	cache.MoveToNextUpdateCycle()
	topics, found := cache.GetSubscribedTopics(peerID)
	require.True(t, found, "existing peer should be found")
	require.ElementsMatch(t, []string{topic1, topic2, topic3}, topics, "retrieved topics should match the added topics")

	// add topic4 to the peer; since we moved to the next update cycle, the topics for the peer should be cleared
	// and topic4 should be the only topic for the peer
	topics, err = cache.AddTopicForPeer(peerID, topic4)
	require.NoError(t, err, "adding fourth topic to existing peer should not produce an error")
	require.Equal(t, []string{topic4}, topics, "updated topics should match the added topic")

	// move to next update cycle
	cache.MoveToNextUpdateCycle()

	// since we did not add any topic to the peer, the topics for the peer should be the same as before
	topics, found = cache.GetSubscribedTopics(peerID)
	require.True(t, found, "existing peer should be found")
	require.ElementsMatch(t, []string{topic4}, topics, "retrieved topics should match the added topics")
}

// TestMoveUpdateCycleWithDifferentPeers tests that moving to the next update cycle does not affect the subscribed
// topics for other peers.
func TestMoveUpdateCycleWithDifferentPeers(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory()))

	peer1 := unittest.PeerIdFixture(t)
	peer2 := unittest.PeerIdFixture(t)
	topic1 := "topic1"
	topic2 := "topic2"

	// add topic1 to peer1
	topics, err := cache.AddTopicForPeer(peer1, topic1)
	require.NoError(t, err, "adding first topic to peer1 should not produce an error")
	require.Equal(t, []string{topic1}, topics, "updated topics should match the added topic")

	// add topic2 to peer2
	topics, err = cache.AddTopicForPeer(peer2, topic2)
	require.NoError(t, err, "adding first topic to peer2 should not produce an error")
	require.Equal(t, []string{topic2}, topics, "updated topics should match the added topic")

	// move to next update cycle
	cache.MoveToNextUpdateCycle()

	// since we did not add any topic to the peers, the topics for the peers should be the same as before
	topics, found := cache.GetSubscribedTopics(peer1)
	require.True(t, found, "peer1 should be found")
	require.ElementsMatch(t, []string{topic1}, topics, "retrieved topics should match the added topics")

	topics, found = cache.GetSubscribedTopics(peer2)
	require.True(t, found, "peer2 should be found")
	require.ElementsMatch(t, []string{topic2}, topics, "retrieved topics should match the added topics")

	// now add topic2 to peer1; it should overwrite the previous topics for peer1, but not affect the topics for peer2
	topics, err = cache.AddTopicForPeer(peer1, topic2)
	require.NoError(t, err, "adding second topic to peer1 should not produce an error")
	require.Equal(t, []string{topic2}, topics, "updated topics should match the added topic")

	topics, found = cache.GetSubscribedTopics(peer2)
	require.True(t, found, "peer2 should be found")
	require.ElementsMatch(t, []string{topic2}, topics, "retrieved topics should match the added topics")
}
