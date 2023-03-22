package ratelimit

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/utils"
)

// ControlMessageRateLimiter rate limiter that rate limits the amount of
type ControlMessageRateLimiter struct {
	*utils.RateLimiter
}

// NewControlMessageRateLimiter returns a new ControlMessageRateLimiter. The cleanup loop will be started in a
// separate goroutine and should be stopped by calling Close.
func NewControlMessageRateLimiter(limit rate.Limit, burst int) p2p.BasicRateLimiter {
	// NOTE: we use a lockout duration of 0 because we only need to expose the basic functionality of the
	// rate limiter and not the lockout feature.
	lockoutDuration := time.Duration(0)
	return &ControlMessageRateLimiter{
		RateLimiter: utils.NewRateLimiter(limit, burst, lockoutDuration),
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
