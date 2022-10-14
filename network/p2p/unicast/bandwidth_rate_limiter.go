package unicast

import (
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/network/p2p"

	"github.com/onflow/flow-go/network/message"
)

// BandWidthRateLimiterImpl unicast rate limiter that limits the bandwidth that can be sent
// by a peer per some configured interval.
type BandWidthRateLimiterImpl struct {
	limiters                 *rateLimiterMap
	limit                    rate.Limit
	burst                    int
	now                      p2p.GetTimeNow
	rateLimitLockoutDuration time.Duration // the amount of time that has to pass before a peer is allowed to connect
}

// NewBandWidthRateLimiter returns a new BandWidthRateLimiterImpl. The cleanup loop will be started in a
// separate goroutine and should be stopped by calling Close.
func NewBandWidthRateLimiter(limit rate.Limit, burst, lockoutDuration int, opts ...p2p.RateLimiterOpt) *BandWidthRateLimiterImpl {
	l := &BandWidthRateLimiterImpl{
		limiters:                 newLimiterMap(rateLimiterTTL, cleanUpTickInterval),
		limit:                    limit,
		burst:                    burst,
		now:                      time.Now,
		rateLimitLockoutDuration: time.Duration(lockoutDuration) * time.Second,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Allow checks the cached limiter for the peer and returns limiter.AllowN(msg.Size())
// which will check if a peer is able to send a message of msg.Size().
// If a limiter is not cached one is created.
func (b *BandWidthRateLimiterImpl) Allow(peerID peer.ID, msg *message.Message) bool {
	limiter := b.getLimiter(peerID)
	if !limiter.AllowN(b.now(), msg.Size()) {
		b.limiters.updateLastRateLimit(peerID, b.now())
		return false
	}

	return true
}

// IsRateLimited returns true is a peer is currently rate limited.
func (b *BandWidthRateLimiterImpl) IsRateLimited(peerID peer.ID) bool {
	metadata, ok := b.limiters.get(peerID)
	if !ok {
		return false
	}
	return time.Since(metadata.lastRateLimit) < b.rateLimitLockoutDuration
}

// SetTimeNowFunc overrides the default time.Now func with the GetTimeNow func provided.
func (b *BandWidthRateLimiterImpl) SetTimeNowFunc(now p2p.GetTimeNow) {
	b.now = now
}

// Start starts cleanup loop for underlying caches.
func (b *BandWidthRateLimiterImpl) Start() {
	go b.limiters.cleanupLoop()
}

// Stop sends cleanup signal to underlying rate limiters and rate limited peers maps. After the rate limiter
// is stopped it can not be reused.
func (b *BandWidthRateLimiterImpl) Stop() {
	b.limiters.close()
}

// getLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (b *BandWidthRateLimiterImpl) getLimiter(peerID peer.ID) *rate.Limiter {
	if metadata, ok := b.limiters.get(peerID); ok {
		return metadata.limiter
	}

	limiter := rate.NewLimiter(b.limit, b.burst)
	b.limiters.store(peerID, limiter)

	return limiter
}
