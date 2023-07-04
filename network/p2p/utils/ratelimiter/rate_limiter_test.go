package ratelimiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestRateLimiter_Allow ensures rate limiter allows messages as expected
func TestRateLimiter_Allow(t *testing.T) {
	// limiter limit will be set to 5 events/sec the 6th event per interval will be rate limited
	limit := rate.Limit(1)

	// burst per interval
	burst := 1

	id, _ := unittest.IdentityWithNetworkingKeyFixture()
	peerID, err := unittest.PeerIDFromFlowID(id)
	require.NoError(t, err)

	// setup rate limiter
	rateLimiter := NewRateLimiter(limit, burst, time.Second)

	require.True(t, rateLimiter.Allow(peerID, 0))

	// second message should be rate limited
	require.False(t, rateLimiter.Allow(peerID, 0))

	// wait for the next interval, the rate limiter should allow the next message.
	time.Sleep(1 * time.Second)

	require.True(t, rateLimiter.Allow(peerID, 0))
}

// TestRateLimiter_IsRateLimited ensures IsRateLimited returns true for peers that are rate limited.
func TestRateLimiter_IsRateLimited(t *testing.T) {
	// limiter limit will be set to 5 events/sec the 6th event per interval will be rate limited
	limit := rate.Limit(1)

	// burst per interval
	burst := 1

	id, _ := unittest.IdentityWithNetworkingKeyFixture()
	peerID, err := unittest.PeerIDFromFlowID(id)
	require.NoError(t, err)

	// setup rate limiter
	rateLimiter := NewRateLimiter(limit, burst, time.Second)

	require.False(t, rateLimiter.IsRateLimited(peerID))
	require.True(t, rateLimiter.Allow(peerID, 0))

	// second message should be rate limited
	require.False(t, rateLimiter.Allow(peerID, 0))
	require.True(t, rateLimiter.IsRateLimited(peerID))

	// wait for the next interval, the rate limiter should allow the next message.
	time.Sleep(1 * time.Second)
	require.True(t, rateLimiter.Allow(peerID, 0))
	require.False(t, rateLimiter.IsRateLimited(peerID))
}
