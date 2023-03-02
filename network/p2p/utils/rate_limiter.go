package utils

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
)

const (
	cleanUpTickInterval = 10 * time.Minute
	rateLimiterTTL      = 10 * time.Minute
)

// RateLimiter generic rate limiter
type RateLimiter struct {
	limiters                 *RateLimiterMap
	limit                    rate.Limit
	burst                    int
	now                      p2p.GetTimeNow
	rateLimitLockoutDuration time.Duration // the amount of time that has to pass before a peer is allowed to connect
}

// NewRateLimiter returns a new RateLimiter.
func NewRateLimiter(limit rate.Limit, burst int, lockoutDuration time.Duration, opts ...p2p.RateLimiterOpt) *RateLimiter {
	l := &RateLimiter{
		limiters:                 NewLimiterMap(rateLimiterTTL, cleanUpTickInterval),
		limit:                    limit,
		burst:                    burst,
		now:                      time.Now,
		rateLimitLockoutDuration: lockoutDuration * time.Second,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Allow checks the cached limiter for the peer and returns limiters.Allow().
// If a limiter is not cached for a peer one is created. This func can be overridden
// and the message size parameter can be used with AllowN.
func (r *RateLimiter) Allow(peerID peer.ID, _ int) bool {
	limiter := r.GetLimiter(peerID)
	if !limiter.AllowN(r.now(), 1) {
		r.limiters.UpdateLastRateLimit(peerID, r.now())
		return false
	}

	return true
}

// IsRateLimited returns true is a peer is currently rate limited.
func (r *RateLimiter) IsRateLimited(peerID peer.ID) bool {
	metadata, ok := r.limiters.Get(peerID)
	if !ok {
		return false
	}
	return time.Since(metadata.LastRateLimit()) < r.rateLimitLockoutDuration
}

// Start starts cleanup loop for underlying cache.
func (r *RateLimiter) Start() {
	go r.limiters.CleanupLoop()
}

// Stop sends cleanup signal to underlying rate limiters and rate limited peers map. After the rate limiter
// is closed it can not be reused.
func (r *RateLimiter) Stop() {
	r.limiters.Close()
}

// SetTimeNowFunc overrides the default time.Now func with the GetTimeNow func provided.
func (r *RateLimiter) SetTimeNowFunc(now p2p.GetTimeNow) {
	r.now = now
}

// Now return the time according to the configured GetTimeNow func
func (r *RateLimiter) Now() time.Time {
	return r.now()
}

// GetLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (r *RateLimiter) GetLimiter(peerID peer.ID) *rate.Limiter {
	if metadata, ok := r.limiters.Get(peerID); ok {
		return metadata.Limiter()
	}

	limiter := rate.NewLimiter(r.limit, r.burst)
	r.limiters.Store(peerID, limiter)

	return limiter
}

// UpdateLastRateLimit updates the last time a peer was rate limited in the limiter map.
func (r *RateLimiter) UpdateLastRateLimit(peerID peer.ID, lastRateLimit time.Time) {
	r.limiters.UpdateLastRateLimit(peerID, lastRateLimit)
}
