package p2p

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// RateLimiter unicast rate limiter interface
type RateLimiter interface {
	// Allow returns true if a message with the give size should be allowed to be processed.
	Allow(peerID peer.ID, msgSize int) bool

	// IsRateLimited returns true if a peer is rate limited.
	IsRateLimited(peerID peer.ID) bool

	// SetTimeNowFunc allows users to override the underlying time module used.
	SetTimeNowFunc(now GetTimeNow)

	// Stop sends cleanup signal to underlying rate limiters and rate limited peers maps. After the rate limiter
	// is stopped it can not be reused.
	Stop()

	// Start starts cleanup loop for underlying rate limiters and rate limited peers maps.
	Start()
}

// GetTimeNow callback used to get the current time. This allows us to improve testing by manipulating the current time
// as opposed to using time.Now directly.
type GetTimeNow func() time.Time

type RateLimiterOpt func(limiter RateLimiter)

func WithGetTimeNowFunc(now GetTimeNow) RateLimiterOpt {
	return func(limiter RateLimiter) {
		limiter.SetTimeNowFunc(now)
	}
}

// RateLimiterConsumer consumes notifications from the ratelimit.RateLimiters whenever a peer is rate limited.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type RateLimiterConsumer interface {
	OnRateLimitedPeer(pid peer.ID, role, msgType, topic, reason string)
}
