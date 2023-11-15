package internal_test

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAppSpecificScoreCache tests the functionality of AppSpecificScoreCache;
// specifically, it tests the Add and Get methods.
// It does not test the eviction policy of the cache.
func TestAppSpecificScoreCache(t *testing.T) {
	logger := zerolog.Nop()

	cache := internal.NewAppSpecificScoreCache(10, logger, metrics.NewNoopCollector())
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
