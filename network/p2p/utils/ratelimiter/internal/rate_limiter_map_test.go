package internal_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p/utils/ratelimiter/internal"
)

// TestLimiterMap_get checks true is returned for stored items and false for missing items.
func TestLimiterMap_get(t *testing.T) {
	m := internal.NewLimiterMap(time.Second, time.Second)
	peerID := peer.ID("id")
	m.Store(peerID, rate.NewLimiter(0, 0))

	_, ok := m.Get(peerID)
	require.True(t, ok)
	_, ok = m.Get("fake")
	require.False(t, ok)
}

// TestLimiterMap_remove checks the map removes keys as expected.
func TestLimiterMap_remove(t *testing.T) {
	m := internal.NewLimiterMap(time.Second, time.Second)
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
	// set fake ttl to 10 minutes
	ttl := 10 * time.Minute

	// set short tick to kick off cleanup
	tick := 10 * time.Millisecond

	m := internal.NewLimiterMap(ttl, tick)

	start := time.Now()

	// Store some peerID's
	peerID1 := peer.ID("id1")
	m.Store(peerID1, rate.NewLimiter(0, 0))

	peerID2 := peer.ID("id2")
	m.Store(peerID2, rate.NewLimiter(0, 0))

	peerID3 := peer.ID("id3")
	m.Store(peerID3, rate.NewLimiter(0, 0))

	// manually set lastAccessed on 2 items so that they are removed during Cleanup
	limiter, _ := m.Get(peerID1)
	limiter.SetLastAccessed(start.Add(-10 * time.Minute))

	limiter, _ = m.Get(peerID2)
	limiter.SetLastAccessed(start.Add(-10 * time.Minute))

	limiter, _ = m.Get(peerID3)
	limiter.SetLastAccessed(start.Add(-20 * time.Minute))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// kick off clean up process, tick should happen immediately
	go m.CleanupLoop(signalerCtx)
	time.Sleep(100 * time.Millisecond)
	_, ok := m.Get(peerID1)
	require.False(t, ok)
	_, ok = m.Get(peerID2)
	require.False(t, ok)
	_, ok = m.Get(peerID3)
	require.False(t, ok)
}

// TestLimiterMap_cleanupLoopCtxCanceled checks that the Cleanup loop runs when ctx is canceled before cleanup loop exits.
func TestLimiterMap_cleanupLoopCtxCanceled(t *testing.T) {
	// set fake ttl to 10 minutes
	ttl := 10 * time.Minute

	// set long tick so that clean up is only done when ctx is canceled
	tick := time.Hour

	m := internal.NewLimiterMap(ttl, tick)

	start := time.Now()

	// Store some peerID's
	peerID1 := peer.ID("id1")
	m.Store(peerID1, rate.NewLimiter(0, 0))

	peerID2 := peer.ID("id2")
	m.Store(peerID2, rate.NewLimiter(0, 0))

	peerID3 := peer.ID("id3")
	m.Store(peerID3, rate.NewLimiter(0, 0))

	// manually set lastAccessed on 2 items so that they are removed during Cleanup
	limiter, _ := m.Get(peerID1)
	limiter.SetLastAccessed(start.Add(-10 * time.Minute))

	limiter, _ = m.Get(peerID2)
	limiter.SetLastAccessed(start.Add(-10 * time.Minute))

	limiter, _ = m.Get(peerID3)
	limiter.SetLastAccessed(start.Add(-20 * time.Minute))

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// kick off clean up loop
	go m.CleanupLoop(signalerCtx)

	// clean up should be kicked off when SignalerContext is canceled
	cancel()
	// sleep for 100ms
	time.Sleep(100 * time.Millisecond)
	_, ok := m.Get(peerID1)
	require.False(t, ok)
	_, ok = m.Get(peerID2)
	require.False(t, ok)
	_, ok = m.Get(peerID3)
	require.False(t, ok)
}
