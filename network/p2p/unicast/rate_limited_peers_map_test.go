package unicast

import (
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
)

// TestRateLimitedPeersMap_exists checks true is returned for stored items and false for missing items.
func TestRateLimitedPeersMap_exists(t *testing.T) {
	t.Parallel()
	m := newRateLimitedPeersMap(time.Second, time.Second)
	peerID := peer.ID("id")
	m.store(peerID)
	require.True(t, m.exists(peerID))
	require.False(t, m.exists("fake"))
}

// TestRateLimitedPeersMap_remove checks the map removes keys as expected.
func TestRateLimitedPeersMap_remove(t *testing.T) {
	t.Parallel()
	m := newRateLimitedPeersMap(time.Second, time.Second)
	peerID := peer.ID("id")
	m.store(peerID)
	require.True(t, m.exists(peerID))

	m.remove(peerID)
	require.False(t, m.exists(peerID))
}

// TestRateLimitedPeersMap_cleanup checks the map removes expired keys as expected.
func TestRateLimitedPeersMap_cleanup(t *testing.T) {
	t.Parallel()

	// set fake ttl to 10 minutes
	ttl := 10 * time.Minute
	m := newRateLimitedPeersMap(ttl, time.Second)

	start := time.Now()

	// store some peerID's
	peerID1 := peer.ID("id1")
	m.store(peerID1)

	peerID2 := peer.ID("id2")
	m.store(peerID2)

	peerID3 := peer.ID("id3")
	m.store(peerID3)

	// manually set lastActive on 2 items so that they are removed during cleanup
	m.peers[peerID2].lastActive = start.Add(-10 * time.Minute)
	m.peers[peerID3].lastActive = start.Add(-20 * time.Minute)

	// light clean up will only remove expired keys
	m.cleanup()
	require.True(t, m.exists(peerID1))
	require.False(t, m.exists(peerID2))
	require.False(t, m.exists(peerID3))

}

// TestRateLimitedPeersMap_cleanupLoopDone checks that the cleanup loop runs when signal is sent on done channel.
func TestRateLimitedPeersMap_cleanupLoopDone(t *testing.T) {
	t.Parallel()

	// set fake ttl to 10 minutes
	ttl := 10 * time.Minute

	// set short tick to kick of cleanup
	tick := 10 * time.Millisecond

	m := newRateLimitedPeersMap(ttl, tick)

	start := time.Now()

	// store some peerID's
	peerID1 := peer.ID("id1")
	m.store(peerID1)

	peerID2 := peer.ID("id2")
	m.store(peerID2)

	peerID3 := peer.ID("id3")
	m.store(peerID3)

	// manually set lastActive on 2 items so that they are removed during cleanup
	m.peers[peerID1].lastActive = start.Add(-10 * time.Minute)
	m.peers[peerID2].lastActive = start.Add(-10 * time.Minute)
	m.peers[peerID3].lastActive = start.Add(-20 * time.Minute)

	// kick off clean up process, tick should happen immediately
	go m.cleanupLoop()
	time.Sleep(100 * time.Millisecond)
	m.close()

	fmt.Println(m.peers)
	require.False(t, m.exists(peerID1))
	require.False(t, m.exists(peerID2))
	require.False(t, m.exists(peerID3))
}
