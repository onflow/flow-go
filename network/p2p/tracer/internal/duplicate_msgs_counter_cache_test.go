package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

const defaultDecay = .99

// TestGossipSubDuplicateMessageTrackerCache tests the functionality of the GossipSubDuplicateMessageTrackerCache.
// It ensures that the Inc and Get methods work as expected, does not test counter decay specifically.
func TestGossipSubDuplicateMessageTrackerCache(t *testing.T) {
	cache := duplicateMessageTrackerCacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())
	peerID := unittest.PeerIdFixture(t)
	// expect count to initially be 0
	count, err, found := cache.Get(peerID)
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, 0.0, count)

	// increment the counter for a peer
	count, err = cache.Inc(peerID)
	require.NoError(t, err)
	require.Equal(t, 1.0, count)
	count, err, found = cache.Get(peerID)
	require.NoError(t, err)
	require.True(t, found)
	unittest.RequireNumericallyClose(t, 1.0, count, .1)
}

// TestGossipSubDuplicateMessageTrackerCache tests the concurrent functionality of the GossipSubDuplicateMessageTrackerCache.
// It ensures that the Inc and Get methods work as expected, does not test counter decay specifically.
func TestGossipSubDuplicateMessageTrackerCache_Concurrent_Inc_Get(t *testing.T) {
	cache := duplicateMessageTrackerCacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())
	peerId1 := unittest.PeerIdFixture(t)
	peerId2 := unittest.PeerIdFixture(t)

	wg := sync.WaitGroup{}
	wg.Add(2)
	// get initial counts concurrently
	go func() {
		defer wg.Done()
		count, err, found := cache.Get(peerId1)
		require.NoError(t, err)
		require.False(t, found)
		require.Equal(t, 0.0, count)
	}()

	go func() {
		defer wg.Done()
		count, err, found := cache.Get(peerId2)
		require.NoError(t, err)
		require.False(t, found)
		require.Equal(t, 0.0, count)
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "failed to get initial counts")

	//  increment counts concurrently
	wg.Add(2)
	go func() {
		defer wg.Done()
		count, err := cache.Inc(peerId1)
		require.NoError(t, err)
		require.Equal(t, 1.0, count)
	}()

	go func() {
		defer wg.Done()
		count, err := cache.Inc(peerId2)
		require.NoError(t, err)
		require.Equal(t, 1.0, count)
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "failed to get increment counts")

	// get updated counts concurrently
	wg.Add(2)
	go func() {
		defer wg.Done()
		count, err, found := cache.Get(peerId1)
		require.NoError(t, err)
		require.True(t, found)
		unittest.RequireNumericallyClose(t, 1.0, count, .1)
	}()

	go func() {
		defer wg.Done()
		count, err, found := cache.Get(peerId2)
		require.NoError(t, err)
		require.True(t, found)
		unittest.RequireNumericallyClose(t, 1.0, count, .1)
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "failed to get updated counts")
}

// TestGossipSubDuplicateMessageTrackerCache tests the decay functionality of the GossipSubDuplicateMessageTrackerCache.
// It ensures that duplicate message is decayed as expected, during sustained good behavior the counter should be
// eventually be decayed to 0.
func TestGossipSubDuplicateMessageTrackerCache_DecayAdjustment(t *testing.T) {
	// lower decay value will increase the speed of the test
	decay := .5
	cache := duplicateMessageTrackerCacheFixture(t, 100, decay, zerolog.Nop(), metrics.NewNoopCollector())
	peerID := unittest.PeerIdFixture(t)
	// expect count to initially be 0
	count, err, found := cache.Get(peerID)
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, 0.0, count)

	// increment the counter for a peer
	count, err = cache.Inc(peerID)
	require.NoError(t, err)
	require.Equal(t, 1.0, count)

	require.Eventually(t, func() bool {
		count, err, found = cache.Get(peerID)
		require.NoError(t, err)
		require.True(t, found)
		return count == 0.0
	}, 10*time.Second, 100*time.Millisecond)
}

// rpcSentCacheFixture returns a new *GossipSubDuplicateMessageTrackerCache.
func duplicateMessageTrackerCacheFixture(t *testing.T, sizeLimit uint32, decay float64, logger zerolog.Logger, collector module.HeroCacheMetrics) *GossipSubDuplicateMessageTrackerCache {
	r := NewGossipSubDuplicateMessageTrackerCache(sizeLimit, decay, logger, collector)
	// expect cache to be empty
	require.Equalf(t, uint(0), r.c.Size(), "cache size must be 0")
	require.NotNil(t, r)
	return r
}
