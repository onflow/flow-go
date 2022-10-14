package limiter_map_test

import (
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit/internal/limiter_map"
	"github.com/stretchr/testify/require"
)

// TestLimiterMap_get checks true is returned for stored items and false for missing items.
func TestLimiterMap_get(t *testing.T) {
	t.Parallel()
	m := limiter_map.NewLimiterMap(time.Second, time.Second)
	peerID := peer.ID("id")
	m.Store(peerID, rate.NewLimiter(0, 0))

	_, ok := m.Get(peerID)
	require.True(t, ok)
	_, ok = m.Get("fake")
	require.False(t, ok)
}

// TestLimiterMap_remove checks the map removes keys as expected.
func TestLimiterMap_remove(t *testing.T) {
	t.Parallel()
	m := limiter_map.NewLimiterMap(time.Second, time.Second)
	peerID := peer.ID("id")
	m.Store(peerID, rate.NewLimiter(0, 0))

	_, ok := m.Get(peerID)
	require.True(t, ok)

	m.Remove(peerID)
	_, ok = m.Get(peerID)
	require.False(t, ok)
}

// TestLimiterMap_cleanup checks the map removes expired keys as expected.
func TestLimiterMap_cleanup(t *testing.T) {
	t.Parallel()

	// set fake ttl to 10 minutes
	ttl := 10 * time.Minute
	m := limiter_map.NewLimiterMap(ttl, time.Second)

	start := time.Now()

	// Store some peerID's
	peerID1 := peer.ID("id1")
	m.Store(peerID1, rate.NewLimiter(0, 0))

	peerID2 := peer.ID("id2")
	m.Store(peerID2, rate.NewLimiter(0, 0))

	peerID3 := peer.ID("id3")
	m.Store(peerID3, rate.NewLimiter(0, 0))

	// manually set LastAccessed on 2 items so that they are removed during Cleanup
	limiter, _ := m.Get(peerID2)
	limiter.LastAccessed = start.Add(-10 * time.Minute)

	limiter, _ = m.Get(peerID3)
	limiter.LastAccessed = start.Add(-20 * time.Minute)

	// light clean up will only Remove expired keys
	m.Cleanup()

	_, ok := m.Get(peerID1)
	require.True(t, ok)
	_, ok = m.Get(peerID2)
	require.False(t, ok)
	_, ok = m.Get(peerID3)
	require.False(t, ok)
}

// TestLimiterMap_cleanupLoopDone checks that the Cleanup loop runs when signal is sent on done channel.
func TestLimiterMap_cleanupLoopDone(t *testing.T) {
	t.Parallel()

	// set fake ttl to 10 minutes
	ttl := 10 * time.Minute

	// set short tick to kick of Cleanup
	tick := 10 * time.Millisecond

	m := limiter_map.NewLimiterMap(ttl, tick)

	start := time.Now()

	// Store some peerID's
	peerID1 := peer.ID("id1")
	m.Store(peerID1, rate.NewLimiter(0, 0))

	peerID2 := peer.ID("id2")
	m.Store(peerID2, rate.NewLimiter(0, 0))

	peerID3 := peer.ID("id3")
	m.Store(peerID3, rate.NewLimiter(0, 0))

	// manually set LastAccessed on 2 items so that they are removed during Cleanup
	limiter, _ := m.Get(peerID1)
	limiter.LastAccessed = start.Add(-10 * time.Minute)

	limiter, _ = m.Get(peerID2)
	limiter.LastAccessed = start.Add(-10 * time.Minute)

	limiter, _ = m.Get(peerID3)
	limiter.LastAccessed = start.Add(-20 * time.Minute)

	// kick off clean up process, tick should happen immediately
	go m.CleanupLoop()
	time.Sleep(100 * time.Millisecond)
	m.Close()

	_, ok := m.Get(peerID1)
	require.False(t, ok)
	_, ok = m.Get(peerID2)
	require.False(t, ok)
	_, ok = m.Get(peerID3)
	require.False(t, ok)
}
