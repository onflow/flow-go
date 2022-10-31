package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/messageutils"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestBandWidthRateLimiter_Allow ensures rate limiter allows messages as expected.
func TestBandWidthRateLimiter_Allow(t *testing.T) {
	//limiter limit will be set up to 1000 bytes/sec
	limit := rate.Limit(1000)

	//burst per interval
	burst := 1000

	// setup bandwidth rate limiter
	bandwidthRateLimiter := NewBandWidthRateLimiter(limit, burst, 1)

	id, _ := unittest.IdentityWithNetworkingKeyFixture()
	peerID, err := unittest.PeerIDFromFlowID(id)
	require.NoError(t, err)

	// create message with about 400bytes (300 random bytes + 100bytes message info)
	b := make([]byte, 300)
	for i := range b {
		b[i] = byte('X')
	}

	msg, _, _ := messageutils.CreateMessage(t, unittest.IdentifierFixture(), unittest.IdentifierFixture(), channels.TestNetworkChannel, string(b))

	allowed := bandwidthRateLimiter.Allow(peerID, msg)
	require.True(t, allowed)
	allowed = bandwidthRateLimiter.Allow(peerID, msg)
	require.True(t, allowed)
	allowed = bandwidthRateLimiter.Allow(peerID, msg)
	require.False(t, allowed)

	// wait for 1 second, the rate limiter should allow 3 messages again
	time.Sleep(1 * time.Second)
	allowed = bandwidthRateLimiter.Allow(peerID, msg)
	require.True(t, allowed)
	allowed = bandwidthRateLimiter.Allow(peerID, msg)
	require.True(t, allowed)
	allowed = bandwidthRateLimiter.Allow(peerID, msg)
	require.False(t, allowed)
}

// TestBandWidthRateLimiter_IsRateLimited ensures IsRateLimited returns true for peers that are rate limited.
func TestBandWidthRateLimiter_IsRateLimited(t *testing.T) {
	//limiter limit will be set up to 1000 bytes/sec
	limit := rate.Limit(1000)

	//burst per interval
	burst := 1000

	// setup bandwidth rate limiter
	bandwidthRateLimiter := NewBandWidthRateLimiter(limit, burst, 1)

	// for the duration of a simulated second we will send 3 messages. Each message is about
	// 400 bytes, the 3rd message will put our limiter over the 1000 byte limit at 1200 bytes. Thus
	// the 3rd message should be rate limited.
	id, _ := unittest.IdentityWithNetworkingKeyFixture()
	peerID, err := unittest.PeerIDFromFlowID(id)
	require.NoError(t, err)

	// create message with about 400bytes (300 random bytes + 100bytes message info)
	b := make([]byte, 500000)
	for i := range b {
		b[i] = byte('X')
	}

	require.False(t, bandwidthRateLimiter.IsRateLimited(peerID))

	msg, _, _ := messageutils.CreateMessage(t, unittest.IdentifierFixture(), unittest.IdentifierFixture(), channels.TestNetworkChannel, string(b))
	allowed := bandwidthRateLimiter.Allow(peerID, msg)
	require.False(t, allowed)
	require.True(t, bandwidthRateLimiter.IsRateLimited(peerID))

	// wait for 1 second, the rate limiter should reset
	time.Sleep(1 * time.Second)
	require.False(t, bandwidthRateLimiter.IsRateLimited(peerID))
}
