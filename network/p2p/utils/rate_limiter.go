package utils

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
)

var (
	defaultGetTimeNowFunc = time.Now
)

const (
	cleanUpTickInterval = 10 * time.Minute
	rateLimiterTTL      = 10 * time.Minute
)

// RateLimiter generic rate limiter
type RateLimiter struct {
	// limiters map that stores a rate limiter with metadata per peer.
	limiters *RateLimiterMap
	// limit amount of messages allowed per second.
	limit rate.Limit
	// burst amount of messages allowed at one time.
	burst int
	// now func that returns timestamp used to rate limit.
	// The default time.Now func is used.
	now p2p.GetTimeNow
	// rateLimitLockoutDuration the amount of time that has to pass before a peer is allowed to connect.
	rateLimitLockoutDuration time.Duration
}

// NewRateLimiter returns a new RateLimiter.
func NewRateLimiter(limit rate.Limit, burst int, lockoutDuration time.Duration, opts ...p2p.RateLimiterOpt) *RateLimiter {
	l := &RateLimiter{
		limiters:                 NewLimiterMap(rateLimiterTTL, cleanUpTickInterval),
		limit:                    limit,
		burst:                    burst,
		now:                      defaultGetTimeNowFunc,
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

// CleanupLoop starts cleanup loop for underlying cache.
// This func blocks until the signaler context is canceled.
func (r *RateLimiter) CleanupLoop(ctx irrecoverable.SignalerContext) {
	r.limiters.CleanupLoop(ctx)
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
