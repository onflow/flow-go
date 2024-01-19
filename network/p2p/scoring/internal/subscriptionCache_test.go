package internal_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewSubscriptionRecordCache tests that NewSubscriptionRecordCache returns a valid cache.
func TestNewSubscriptionRecordCache(t *testing.T) {
	sizeLimit := uint32(100)

	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	require.NotNil(t, cache, "cache should not be nil")
	require.IsType(t, &internal.SubscriptionRecordCache{}, cache, "cache should be of type *SubscriptionRecordCache")
}

// TestSubscriptionCache_GetSubscribedTopics tests the retrieval of subscribed topics for a peer.
func TestSubscriptionCache_GetSubscribedTopics(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	// create a dummy peer ID
	peerID := unittest.PeerIdFixture(t)

	// case when the peer has a subscription
	topics := []string{"topic1", "topic2"}
	updatedTopics, err := cache.AddWithInitTopicForPeer(peerID, topics[0])
	require.NoError(t, err, "adding topic 1 should not produce an error")
	require.Equal(t, topics[:1], updatedTopics, "updated topics should match the added topic")
	updatedTopics, err = cache.AddWithInitTopicForPeer(peerID, topics[1])
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

// TestSubscriptionCache_MoveToNextUpdateCycle tests the increment of update cycles in SubscriptionRecordCache.
// The first increment should set the cycle to 1, and the second increment should set the cycle to 2.
func TestSubscriptionCache_MoveToNextUpdateCycle(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	// initial cycle should be 0, so first increment sets it to 1
	firstCycle := cache.MoveToNextUpdateCycle()
	require.Equal(t, uint64(1), firstCycle, "first cycle should be 1 after first increment")

	// increment cycle again and verify it's now 2
	secondCycle := cache.MoveToNextUpdateCycle()
	require.Equal(t, uint64(2), secondCycle, "second cycle should be 2 after second increment")
}

// TestSubscriptionCache_TestAddTopicForPeer tests adding a topic for a peer.
func TestSubscriptionCache_TestAddTopicForPeer(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	// case when adding a topic to an existing peer
	existingPeerID := unittest.PeerIdFixture(t)
	firstTopic := "topic1"
	secondTopic := "topic2"

	// add first topic to the existing peer
	_, err := cache.AddWithInitTopicForPeer(existingPeerID, firstTopic)
	require.NoError(t, err, "adding first topic to existing peer should not produce an error")

	// add second topic to the same peer
	updatedTopics, err := cache.AddWithInitTopicForPeer(existingPeerID, secondTopic)
	require.NoError(t, err, "adding second topic to existing peer should not produce an error")
	require.ElementsMatch(t, []string{firstTopic, secondTopic}, updatedTopics, "updated topics should match the added topics")

	// case when adding a topic to a new peer
	newPeerID := unittest.PeerIdFixture(t)
	newTopic := "newTopic"

	// add a topic to the new peer
	updatedTopics, err = cache.AddWithInitTopicForPeer(newPeerID, newTopic)
	require.NoError(t, err, "adding topic to new peer should not produce an error")
	require.Equal(t, []string{newTopic}, updatedTopics, "updated topics for new peer should match the added topic")

	// sanity check that the topics for existing peer are still the same
	retrievedTopics, found := cache.GetSubscribedTopics(existingPeerID)
	require.True(t, found, "existing peer should be found")
	require.ElementsMatch(t, []string{firstTopic, secondTopic}, retrievedTopics, "retrieved topics should match the added topics")
}

// TestSubscriptionCache_DuplicateTopics tests adding a duplicate topic for a peer. The duplicate topic should not be added.
func TestSubscriptionCache_DuplicateTopics(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	peerID := unittest.PeerIdFixture(t)
	topic := "topic1"

	// add first topic to the existing peer
	_, err := cache.AddWithInitTopicForPeer(peerID, topic)
	require.NoError(t, err, "adding first topic to existing peer should not produce an error")

	// add second topic to the same peer
	updatedTopics, err := cache.AddWithInitTopicForPeer(peerID, topic)
	require.NoError(t, err, "adding duplicate topic to existing peer should not produce an error")
	require.Equal(t, []string{topic}, updatedTopics, "duplicate topic should not be added")
}

// TestSubscriptionCache_MoveUpdateCycle tests that (1) within one update cycle, "AddWithInitTopicForPeer" calls append the topics to the list of
// subscribed topics for peer, (2) as long as there is no "AddWithInitTopicForPeer" call, moving to the next update cycle
// does not change the subscribed topics for a peer, and (3) calling "AddWithInitTopicForPeer" after moving to the next update
// cycle clears the subscribed topics for a peer and adds the new topic.
func TestSubscriptionCache_MoveUpdateCycle(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	peerID := unittest.PeerIdFixture(t)
	topic1 := "topic1"
	topic2 := "topic2"
	topic3 := "topic3"
	topic4 := "topic4"

	// adds topic1, topic2, and topic3 to the peer
	topics, err := cache.AddWithInitTopicForPeer(peerID, topic1)
	require.NoError(t, err, "adding first topic to existing peer should not produce an error")
	require.Equal(t, []string{topic1}, topics, "updated topics should match the added topic")
	topics, err = cache.AddWithInitTopicForPeer(peerID, topic2)
	require.NoError(t, err, "adding second topic to existing peer should not produce an error")
	require.Equal(t, []string{topic1, topic2}, topics, "updated topics should match the added topics")
	topics, err = cache.AddWithInitTopicForPeer(peerID, topic3)
	require.NoError(t, err, "adding third topic to existing peer should not produce an error")
	require.Equal(t, []string{topic1, topic2, topic3}, topics, "updated topics should match the added topics")

	// move to next update cycle
	cache.MoveToNextUpdateCycle()
	topics, found := cache.GetSubscribedTopics(peerID)
	require.True(t, found, "existing peer should be found")
	require.ElementsMatch(t, []string{topic1, topic2, topic3}, topics, "retrieved topics should match the added topics")

	// add topic4 to the peer; since we moved to the next update cycle, the topics for the peer should be cleared
	// and topic4 should be the only topic for the peer
	topics, err = cache.AddWithInitTopicForPeer(peerID, topic4)
	require.NoError(t, err, "adding fourth topic to existing peer should not produce an error")
	require.Equal(t, []string{topic4}, topics, "updated topics should match the added topic")

	// move to next update cycle
	cache.MoveToNextUpdateCycle()

	// since we did not add any topic to the peer, the topics for the peer should be the same as before
	topics, found = cache.GetSubscribedTopics(peerID)
	require.True(t, found, "existing peer should be found")
	require.ElementsMatch(t, []string{topic4}, topics, "retrieved topics should match the added topics")
}

// TestSubscriptionCache_MoveUpdateCycleWithDifferentPeers tests that moving to the next update cycle does not affect the subscribed
// topics for other peers.
func TestSubscriptionCache_MoveUpdateCycleWithDifferentPeers(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	peer1 := unittest.PeerIdFixture(t)
	peer2 := unittest.PeerIdFixture(t)
	topic1 := "topic1"
	topic2 := "topic2"

	// add topic1 to peer1
	topics, err := cache.AddWithInitTopicForPeer(peer1, topic1)
	require.NoError(t, err, "adding first topic to peer1 should not produce an error")
	require.Equal(t, []string{topic1}, topics, "updated topics should match the added topic")

	// add topic2 to peer2
	topics, err = cache.AddWithInitTopicForPeer(peer2, topic2)
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
	topics, err = cache.AddWithInitTopicForPeer(peer1, topic2)
	require.NoError(t, err, "adding second topic to peer1 should not produce an error")
	require.Equal(t, []string{topic2}, topics, "updated topics should match the added topic")

	topics, found = cache.GetSubscribedTopics(peer2)
	require.True(t, found, "peer2 should be found")
	require.ElementsMatch(t, []string{topic2}, topics, "retrieved topics should match the added topics")
}

// TestSubscriptionCache_ConcurrentUpdate tests subscription cache update in a concurrent environment.
func TestSubscriptionCache_ConcurrentUpdate(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	peerIds := unittest.PeerIdFixtures(t, 100)
	topics := []string{"topic1", "topic2", "topic3"}

	allUpdatesDone := sync.WaitGroup{}
	for _, pid := range peerIds {
		for _, topic := range topics {
			pid := pid
			topic := topic
			allUpdatesDone.Add(1)
			go func() {
				defer allUpdatesDone.Done()
				_, err := cache.AddWithInitTopicForPeer(pid, topic)
				require.NoError(t, err, "adding topic to peer should not produce an error")
			}()
		}
	}

	unittest.RequireReturnsBefore(t, allUpdatesDone.Wait, 1*time.Second, "all updates did not finish in time")

	// verify that all peers have all topics; concurrently
	allTopicsVerified := sync.WaitGroup{}
	for _, pid := range peerIds {
		pid := pid
		allTopicsVerified.Add(1)
		go func() {
			defer allTopicsVerified.Done()
			topics, found := cache.GetSubscribedTopics(pid)
			require.True(t, found, "peer should be found")
			require.ElementsMatch(t, topics, topics, "retrieved topics should match the added topics")
		}()
	}

	unittest.RequireReturnsBefore(t, allTopicsVerified.Wait, 1*time.Second, "all topics were not verified in time")
}

// TestSubscriptionCache_TestSizeLimit tests that the cache evicts the least recently used peer when the cache size limit is reached.
func TestSubscriptionCache_TestSizeLimit(t *testing.T) {
	sizeLimit := uint32(100)
	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory(), network.PrivateNetwork))

	peerIds := unittest.PeerIdFixtures(t, 100)
	topics := []string{"topic1", "topic2", "topic3"}

	// add topics to peers
	for _, pid := range peerIds {
		for _, topic := range topics {
			_, err := cache.AddWithInitTopicForPeer(pid, topic)
			require.NoError(t, err, "adding topic to peer should not produce an error")
		}
	}

	// verify that all peers have all topics
	for _, pid := range peerIds {
		topics, found := cache.GetSubscribedTopics(pid)
		require.True(t, found, "peer should be found")
		require.ElementsMatch(t, topics, topics, "retrieved topics should match the added topics")
	}

	// add one more peer and verify that the first peer is evicted
	newPeerID := unittest.PeerIdFixture(t)
	_, err := cache.AddWithInitTopicForPeer(newPeerID, topics[0])
	require.NoError(t, err, "adding topic to peer should not produce an error")

	_, found := cache.GetSubscribedTopics(peerIds[0])
	require.False(t, found, "peer should not be found")

	// verify that all other peers still have all topics
	for _, pid := range peerIds[1:] {
		topics, found := cache.GetSubscribedTopics(pid)
		require.True(t, found, "peer should be found")
		require.ElementsMatch(t, topics, topics, "retrieved topics should match the added topics")
	}

	// verify that the new peer has the topic
	topics, found = cache.GetSubscribedTopics(newPeerID)
	require.True(t, found, "peer should be found")
	require.ElementsMatch(t, topics, topics, "retrieved topics should match the added topics")
}
