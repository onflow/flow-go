package unicast

import (
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
)

// TestLimiterMap_get checks true is returned for stored items and false for missing items.
func TestLimiterMap_get(t *testing.T) {
	t.Parallel()
	m := newLimiterMap(time.Second, time.Second)
	peerID := peer.ID("id")
	m.store(peerID, rate.NewLimiter(0, 0))

	_, ok := m.get(peerID)
	require.True(t, ok)
	_, ok = m.get("fake")
	require.False(t, ok)
}

// TestLimiterMap_remove checks the map removes keys as expected.
func TestLimiterMap_remove(t *testing.T) {
	t.Parallel()
	m := newLimiterMap(time.Second, time.Second)
	peerID := peer.ID("id")
	m.store(peerID, rate.NewLimiter(0, 0))

	_, ok := m.get(peerID)
	require.True(t, ok)

	m.remove(peerID)
	_, ok = m.get(peerID)
	require.False(t, ok)
}

// TestLimiterMap_cleanup checks the map removes expired keys as expected.
func TestLimiterMap_cleanup(t *testing.T) {
	t.Parallel()

	// set fake ttl to 10 minutes
	ttl := 10 * time.Minute
	m := newLimiterMap(ttl, time.Second)

	start := time.Now()

	// store some peerID's
	peerID1 := peer.ID("id1")
	m.store(peerID1, rate.NewLimiter(0, 0))

	peerID2 := peer.ID("id2")
	m.store(peerID2, rate.NewLimiter(0, 0))

	peerID3 := peer.ID("id3")
	m.store(peerID3, rate.NewLimiter(0, 0))

	// manually set lastActive on 2 items so that they are removed during cleanup
	m.limiters[peerID2].lastActive = start.Add(-10 * time.Minute)
	m.limiters[peerID3].lastActive = start.Add(-20 * time.Minute)

	// light clean up will only remove expired keys
	m.cleanup()

	_, ok := m.get(peerID1)
	require.True(t, ok)
	_, ok = m.get(peerID2)
	require.False(t, ok)
	_, ok = m.get(peerID3)
	require.False(t, ok)

	// full cleanup removes all keys
	m.cleanup()
	_, ok = m.get(peerID1)
	require.False(t, ok)
}

// TestLimiterMap_cleanupLoopDone checks that the cleanup loop runs when signal is sent on done channel.
func TestLimiterMap_cleanupLoopDone(t *testing.T) {
	t.Parallel()

	// set fake ttl to 10 minutes
	ttl := 10 * time.Minute

	// set short tick to kick of cleanup
	tick := 10 * time.Millisecond

	m := newLimiterMap(ttl, tick)

	start := time.Now()

	// store some peerID's
	peerID1 := peer.ID("id1")
	m.store(peerID1, rate.NewLimiter(0, 0))

	peerID2 := peer.ID("id2")
	m.store(peerID2, rate.NewLimiter(0, 0))

	peerID3 := peer.ID("id3")
	m.store(peerID3, rate.NewLimiter(0, 0))

	// manually set lastActive on 2 items so that they are removed during cleanup
	m.limiters[peerID2].lastActive = start.Add(-10 * time.Minute)
	m.limiters[peerID3].lastActive = start.Add(-20 * time.Minute)

	// kick off clean up process, tick should happen immediately
	go m.cleanupLoop()
	m.close()

	time.Sleep(100 * time.Millisecond)

	_, ok := m.get(peerID1)
	require.False(t, ok)
	_, ok = m.get(peerID2)
	require.False(t, ok)
	_, ok = m.get(peerID3)
	require.False(t, ok)
}
