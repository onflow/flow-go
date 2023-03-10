package p2p

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// RateLimiter rate limiter with lockout feature that can be used via the IsRateLimited method.
// This limiter allows users to flag a peer as rate limited for a lockout duration.
type RateLimiter interface {
	BasicRateLimiter
	// IsRateLimited returns true if a peer is rate limited.
	IsRateLimited(peerID peer.ID) bool
}

// BasicRateLimiter rate limiter interface
type BasicRateLimiter interface {
	// Allow returns true if a message with the give size should be allowed to be processed.
	Allow(peerID peer.ID, msgSize int) bool

	// SetTimeNowFunc allows users to override the underlying time module used.
	SetTimeNowFunc(now GetTimeNow)

	// Now returns the time using the configured GetTimeNow func.
	Now() time.Time

	// CleanupLoop starts cleanup loop for underlying rate limiters and rate limited peers maps.
	// This func blocks until the signaler context is canceled.
	CleanupLoop(ctx irrecoverable.SignalerContext)
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

// UnicastRateLimiterDistributor consumes then distributes notifications from the ratelimit.RateLimiters whenever a peer is rate limited.
type UnicastRateLimiterDistributor interface {
	RateLimiterConsumer
	AddConsumer(consumer RateLimiterConsumer)
}

// RateLimiterConsumer consumes notifications from the ratelimit.RateLimiters whenever a peer is rate limited.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type RateLimiterConsumer interface {
	OnRateLimitedPeer(pid peer.ID, role, msgType, topic, reason string)
}
