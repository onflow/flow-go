package ratelimit

import (
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit/internal/limiter_map"

	"github.com/onflow/flow-go/network/message"
)

// BandWidthRateLimiter unicast rate limiter that limits the bandwidth that can be sent
// by a peer per some configured interval.
type BandWidthRateLimiter struct {
	limiters                 *limiter_map.RateLimiterMap
	limit                    rate.Limit
	burst                    int
	now                      p2p.GetTimeNow
	rateLimitLockoutDuration time.Duration // the amount of time that has to pass before a peer is allowed to connect
}

// NewBandWidthRateLimiter returns a new BandWidthRateLimiter. The cleanup loop will be started in a
// separate goroutine and should be stopped by calling Close.
func NewBandWidthRateLimiter(limit rate.Limit, burst int, lockout time.Duration, opts ...p2p.RateLimiterOpt) *BandWidthRateLimiter {
	l := &BandWidthRateLimiter{
		limiters:                 limiter_map.NewLimiterMap(rateLimiterTTL, cleanUpTickInterval),
		limit:                    limit,
		burst:                    burst,
		now:                      time.Now,
		rateLimitLockoutDuration: lockout * time.Second,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Allow checks the cached limiter for the peer and returns limiter.AllowN(msg.Size())
// which will check if a peer is able to send a message of msg.Size().
// If a limiter is not cached one is created.
func (b *BandWidthRateLimiter) Allow(peerID peer.ID, msg *message.Message) bool {
	limiter := b.getLimiter(peerID)
	if !limiter.AllowN(b.now(), msg.Size()) {
		b.limiters.UpdateLastRateLimit(peerID, b.now())
		return false
	}

	return true
}

// IsRateLimited returns true is a peer is currently rate limited.
func (b *BandWidthRateLimiter) IsRateLimited(peerID peer.ID) bool {
	metadata, ok := b.limiters.Get(peerID)
	if !ok {
		return false
	}
	return time.Since(metadata.LastRateLimit) < b.rateLimitLockoutDuration
}

// SetTimeNowFunc overrides the default time.Now func with the GetTimeNow func provided.
func (b *BandWidthRateLimiter) SetTimeNowFunc(now p2p.GetTimeNow) {
	b.now = now
}

// Start starts cleanup loop for underlying caches.
func (b *BandWidthRateLimiter) Start() {
	go b.limiters.CleanupLoop()
}

// Stop sends cleanup signal to underlying rate limiters and rate limited peers maps. After the rate limiter
// is stopped it can not be reused.
func (b *BandWidthRateLimiter) Stop() {
	b.limiters.Close()
}

// getLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (b *BandWidthRateLimiter) getLimiter(peerID peer.ID) *rate.Limiter {
	if metadata, ok := b.limiters.Get(peerID); ok {
		return metadata.Limiter
	}

	limiter := rate.NewLimiter(b.limit, b.burst)
	b.limiters.Store(peerID, limiter)

	return limiter
}
