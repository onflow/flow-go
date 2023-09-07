package ratelimiter

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/utils/ratelimiter/internal"
)

const (
	cleanUpTickInterval = 10 * time.Minute
	rateLimiterTTL      = 10 * time.Minute
)

// RateLimiter generic rate limiter
type RateLimiter struct {
	component.Component
	// limiterMap map that stores a rate limiter with metadata per peer.
	limiterMap *internal.RateLimiterMap
	// limit amount of messages allowed per second.
	limit rate.Limit
	// burst amount of messages allowed at one time.
	burst int
	// rateLimitLockoutDuration the amount of time that has to pass before a peer is allowed to connect.
	rateLimitLockoutDuration time.Duration
}

var _ component.Component = (*RateLimiter)(nil)
var _ p2p.RateLimiter = (*RateLimiter)(nil)

// NewRateLimiter returns a new RateLimiter.
func NewRateLimiter(limit rate.Limit, burst int, lockoutDuration time.Duration, opts ...p2p.RateLimiterOpt) *RateLimiter {
	l := &RateLimiter{
		limiterMap:               internal.NewLimiterMap(rateLimiterTTL, cleanUpTickInterval),
		limit:                    limit,
		burst:                    burst,
		rateLimitLockoutDuration: lockoutDuration,
	}

	for _, opt := range opts {
		opt(l)
	}

	l.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			l.limiterMap.CleanupLoop(ctx)
		}).Build()

	return l
}

// Allow checks the cached limiter for the peer and returns limiterMap.Allow().
// If a limiter is not cached for a peer one is created. This func can be overridden
// and the message size parameter can be used with AllowN.
func (r *RateLimiter) Allow(peerID peer.ID, _ int) bool {
	limiter := r.GetLimiter(peerID)
	if !limiter.AllowN(time.Now(), 1) {
		r.limiterMap.UpdateLastRateLimit(peerID, time.Now())
		return false
	}

	return true
}

// IsRateLimited returns true is a peer is currently rate limited.
func (r *RateLimiter) IsRateLimited(peerID peer.ID) bool {
	metadata, ok := r.limiterMap.Get(peerID)
	if !ok {
		return false
	}
	return time.Since(metadata.LastRateLimit()) < r.rateLimitLockoutDuration
}

// GetLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (r *RateLimiter) GetLimiter(peerID peer.ID) *rate.Limiter {
	if metadata, ok := r.limiterMap.Get(peerID); ok {
		return metadata.Limiter()
	}

	limiter := rate.NewLimiter(r.limit, r.burst)
	r.limiterMap.Store(peerID, limiter)

	return limiter
}

// UpdateLastRateLimit updates the last time a peer was rate limited in the limiter map.
func (r *RateLimiter) UpdateLastRateLimit(peerID peer.ID, lastRateLimit time.Time) {
	r.limiterMap.UpdateLastRateLimit(peerID, lastRateLimit)
}
