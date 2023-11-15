package internal_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAppSpecificScoreCache tests the functionality of AppSpecificScoreCache;
// specifically, it tests the Add and Get methods.
// It does not test the eviction policy of the cache.
func TestAppSpecificScoreCache(t *testing.T) {
	cache := internal.NewAppSpecificScoreCache(10, unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, cache, "failed to create AppSpecificScoreCache")

	peerID := unittest.PeerIdFixture(t)
	score := 5.0
	updateTime := time.Now()

	err := cache.Add(peerID, score, updateTime)
	require.Nil(t, err, "failed to add score to cache")

	// retrieve score from cache
	retrievedScore, lastUpdated, found := cache.Get(peerID)
	require.True(t, found, "failed to find score in cache")
	require.Equal(t, score, retrievedScore, "retrieved score does not match expected")
	require.Equal(t, updateTime, lastUpdated, "retrieved update time does not match expected")

	// test cache update
	newScore := 10.0
	err = cache.Add(peerID, newScore, updateTime.Add(time.Minute))
	require.Nil(t, err, "Failed to update score in cache")

	// retrieve updated score
	updatedScore, updatedTime, found := cache.Get(peerID)
	require.True(t, found, "failed to find updated score in cache")
	require.Equal(t, newScore, updatedScore, "updated score does not match expected")
	require.Equal(t, updateTime.Add(time.Minute), updatedTime, "updated time does not match expected")
}

// TestAppSpecificScoreCache_Concurrent_Add_Get_Update tests the concurrent functionality of AppSpecificScoreCache;
// specifically, it tests the Add and Get methods under concurrent access.
func TestAppSpecificScoreCache_Concurrent_Add_Get_Update(t *testing.T) {
	cache := internal.NewAppSpecificScoreCache(10, unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, cache, "failed to create AppSpecificScoreCache")

	peerId1 := unittest.PeerIdFixture(t)
	score1 := 5.0
	lastUpdated1 := time.Now()

	peerId2 := unittest.PeerIdFixture(t)
	score2 := 10.0
	lastUpdated2 := time.Now().Add(time.Minute)

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := cache.Add(peerId1, score1, lastUpdated1)
		require.Nil(t, err, "failed to add score1 to cache")
	}()

	go func() {
		defer wg.Done()
		err := cache.Add(peerId2, score2, lastUpdated2)
		require.Nil(t, err, "failed to add score2 to cache")
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "failed to add scores to cache")

	// retrieve scores concurrently
	wg.Add(2)
	go func() {
		defer wg.Done()
		retrievedScore, lastUpdated, found := cache.Get(peerId1)
		require.True(t, found, "failed to find score1 in cache")
		require.Equal(t, score1, retrievedScore, "retrieved score1 does not match expected")
		require.Equal(t, lastUpdated1, lastUpdated, "retrieved update time1 does not match expected")
	}()

	go func() {
		defer wg.Done()
		retrievedScore, lastUpdated, found := cache.Get(peerId2)
		require.True(t, found, "failed to find score2 in cache")
		require.Equal(t, score2, retrievedScore, "retrieved score2 does not match expected")
		require.Equal(t, lastUpdated2, lastUpdated, "retrieved update time2 does not match expected")
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "failed to retrieve scores from cache")

	// test cache update
	newScore1 := 15.0
	newScore2 := 20.0
	lastUpdated1 = time.Now().Add(time.Minute)
	lastUpdated2 = time.Now().Add(time.Minute)

	wg.Add(2)
	go func() {
		defer wg.Done()
		err := cache.Add(peerId1, newScore1, lastUpdated1)
		require.Nil(t, err, "failed to update score1 in cache")
	}()

	go func() {
		defer wg.Done()
		err := cache.Add(peerId2, newScore2, lastUpdated2)
		require.Nil(t, err, "failed to update score2 in cache")
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "failed to update scores in cache")

	// retrieve updated scores concurrently
	wg.Add(2)

	go func() {
		defer wg.Done()
		updatedScore, updatedTime, found := cache.Get(peerId1)
		require.True(t, found, "failed to find updated score1 in cache")
		require.Equal(t, newScore1, updatedScore, "updated score1 does not match expected")
		require.Equal(t, lastUpdated1, updatedTime, "updated time1 does not match expected")
	}()

	go func() {
		defer wg.Done()
		updatedScore, updatedTime, found := cache.Get(peerId2)
		require.True(t, found, "failed to find updated score2 in cache")
		require.Equal(t, newScore2, updatedScore, "updated score2 does not match expected")
		require.Equal(t, lastUpdated2, updatedTime, "updated time2 does not match expected")
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "failed to retrieve updated scores from cache")
}

// TestAppSpecificScoreCache_Eviction tests the eviction policy of AppSpecificScoreCache;
// specifically, it tests that the cache evicts the least recently used record when the cache is full.
func TestAppSpecificScoreCache_Eviction(t *testing.T) {
	cache := internal.NewAppSpecificScoreCache(10, unittest.Logger(), metrics.NewNoopCollector())
	require.NotNil(t, cache, "failed to create AppSpecificScoreCache")

	peerIds := unittest.PeerIdFixtures(t, 11)
	scores := []float64{5.0, 10.0, 15.0, 20.0, 25.0, 30.0, 35.0, -1, -2, -3, -4}
	require.Equal(t, len(peerIds), len(scores), "peer ids and scores must have the same length")

	// add scores to cache
	for i := 0; i < len(peerIds); i++ {
		err := cache.Add(peerIds[i], scores[i], time.Now())
		require.Nil(t, err, "failed to add score to cache")
	}

	// retrieve scores from cache; the first score should have been evicted
	for i := 1; i < len(peerIds); i++ {
		retrievedScore, _, found := cache.Get(peerIds[i])
		require.True(t, found, "failed to find score in cache")
		require.Equal(t, scores[i], retrievedScore, "retrieved score does not match expected")
	}

	// the first score should not be in the cache
	_, _, found := cache.Get(peerIds[0])
	require.False(t, found, "score should not be in cache")
}
