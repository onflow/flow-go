package ratelimit

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/p2p/utils/ratelimiter"
)

// ControlMessageRateLimiter rate limiter that rate limits the amount of
type ControlMessageRateLimiter struct {
	*ratelimiter.RateLimiter
}

var _ p2p.BasicRateLimiter = (*ControlMessageRateLimiter)(nil)

// NewControlMessageRateLimiter returns a new ControlMessageRateLimiter. The cleanup loop will be started in a
// separate goroutine and should be stopped by calling Close.
func NewControlMessageRateLimiter(logger zerolog.Logger, limit rate.Limit, burst int) p2p.BasicRateLimiter {
	if limit == 0 {
		logger.Warn().Msg("control message rate limit set to 0 using noop rate limiter")
		// setup noop rate limiter if rate limiting is disabled
		return ratelimit.NewNoopRateLimiter()
	}

	// NOTE: we use a lockout duration of 0 because we only need to expose the basic functionality of the
	// rate limiter and not the lockout feature.
	lockoutDuration := time.Duration(0)
	return &ControlMessageRateLimiter{
		RateLimiter: ratelimiter.NewRateLimiter(limit, burst, lockoutDuration),
	}
}

// Allow checks the cached limiter for the peer and returns limiter.Allow().
// If a limiter is not cached for a peer one is created.
func (c *ControlMessageRateLimiter) Allow(peerID peer.ID, n int) bool {
	limiter := c.GetLimiter(peerID)
	if !limiter.AllowN(time.Now(), n) {
		c.UpdateLastRateLimit(peerID, time.Now())
		return false
	}

	return true
}
