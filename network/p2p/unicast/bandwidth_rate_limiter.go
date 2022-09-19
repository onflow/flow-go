package unicast

import (
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/network/message"
)

// BandWidthRateLimiterImpl unicast rate limiter that limits the bandwidth that can be sent
// by a peer per some configured interval.
type BandWidthRateLimiterImpl struct {
	rateLimitedPeers *rateLimitedPeersMap
	limiters         *rateLimiterMap
	limit            rate.Limit
	burst            int
	now              GetTimeNow
}

// NewBandWidthRateLimiter returns a new BandWidthRateLimiterImpl. The cleanup loop will be started in a
// separate goroutine and should be stopped by calling Close.
func NewBandWidthRateLimiter(limit rate.Limit, burst int, opts ...RateLimiterOpt) *BandWidthRateLimiterImpl {
	l := &BandWidthRateLimiterImpl{
		rateLimitedPeers: newRateLimitedPeersMap(rateLimiterTTL, cleanUpTickDuration),
		limiters:         newLimiterMap(rateLimiterTTL, cleanUpTickDuration),
		limit:            limit,
		burst:            burst,
		now:              time.Now,
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
		b.rateLimitedPeers.store(peerID)
		return false
	} else {
		b.rateLimitedPeers.remove(peerID)
		return true
	}
}

// IsRateLimited returns true is a peer is currently rate limited.
func (b *BandWidthRateLimiterImpl) IsRateLimited(peerID peer.ID) bool {
	return b.rateLimitedPeers.exists(peerID)
}

// SetTimeNowFunc overrides the default time.Now func with the GetTimeNow func provided.
func (b *BandWidthRateLimiterImpl) SetTimeNowFunc(now GetTimeNow) {
	b.now = now
}

// Start starts cleanup loop for underlying caches.
func (b *BandWidthRateLimiterImpl) Start() {
	go b.limiters.cleanupLoop()
	go b.rateLimitedPeers.cleanupLoop()
}

// Stop sends cleanup signal to underlying rate limiters and rate limited peers maps. After the rate limiter
// is stopped it can not be reused.
func (b *BandWidthRateLimiterImpl) Stop() {
	b.limiters.close()
	b.rateLimitedPeers.close()
}

// getLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (b *BandWidthRateLimiterImpl) getLimiter(peerID peer.ID) *rate.Limiter {
	if limiter, ok := b.limiters.get(peerID); ok {
		return limiter
	}

	limiter := rate.NewLimiter(b.limit, b.burst)
	b.limiters.store(peerID, limiter)

	return limiter
}
