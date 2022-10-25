package ratelimit

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestMessageRateLimiter_Allow ensures rate limiter allows messages as expected
func TestMessageRateLimiter_Allow(t *testing.T) {
	// limiter limit will be set to 5 events/sec the 6th event per interval will be rate limited
	limit := rate.Limit(1)

	// burst per interval
	burst := 1

	id, _ := unittest.IdentityWithNetworkingKeyFixture()
	peerID, err := unittest.PeerIDFromFlowID(id)
	require.NoError(t, err)

	// setup message rate limiter
	messageRateLimiter := NewMessageRateLimiter(limit, burst, 1)

	require.True(t, messageRateLimiter.Allow(peerID, nil))

	// second message should be rate limited
	require.False(t, messageRateLimiter.Allow(peerID, nil))
	
	
		// wait for the next interval, the rate limiter should allow the next message.
	time.Sleep(1 * time.Second)

	require.True(t, messageRateLimiter.Allow(peerID, nil))
}

// TestMessageRateLimiter_IsRateLimited ensures IsRateLimited returns true for peers that are rate limited.
func TestMessageRateLimiter_IsRateLimited(t *testing.T) {
	// limiter limit will be set to 5 events/sec the 6th event per interval will be rate limited
	limit := rate.Limit(1)

	// burst per interval
	burst := 1

	id, _ := unittest.IdentityWithNetworkingKeyFixture()
	peerID, err := unittest.PeerIDFromFlowID(id)
	require.NoError(t, err)

	// setup message rate limiter
	messageRateLimiter := NewMessageRateLimiter(limit, burst, 1)

	require.False(t, messageRateLimiter.IsRateLimited(peerID))
	require.True(t, messageRateLimiter.Allow(peerID, nil))

	// second message should be rate limited
	require.False(t, messageRateLimiter.Allow(peerID, nil))
	require.True(t, messageRateLimiter.IsRateLimited(peerID))
	
	// wait for the next interval, the rate limiter should allow the next message.
	time.Sleep(1 * time.Second)
	require.True(t, messageRateLimiter.Allow(peerID, nil))
	require.False(t, messageRateLimiter.IsRateLimited(peerID))
}
