package unicast

import (
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/message"
)

// BandWidthRateLimiterImpl unicast rate limiter that limits the bandwidth that can be sent
// by a peer per some configured interval.
type BandWidthRateLimiterImpl struct {
	rateLimitedPeers *rateLimitedPeers
	limiters         *limiters
	limit            rate.Limit
	burst            int
}

// NewBandWidthRateLimiter returns a new BandWidthRateLimiterImpl.
func NewBandWidthRateLimiter(limit rate.Limit, burst int) *BandWidthRateLimiterImpl {
	return &BandWidthRateLimiterImpl{
		rateLimitedPeers: newRateLimitedPeers(),
		limiters:         newLimiters(),
		limit:            limit,
		burst:            burst,
	}
}

// Allow checks the cached limiter for the peer and returns limiter.AllowN(msg.Size())
// which will check if a peer is able to send a message of msg.Size().
// If a limiter is not cached for a one is created.
func (b *BandWidthRateLimiterImpl) Allow(peerID peer.ID, msg *message.Message) bool {
	limiter := b.getLimiter(peerID)
	if !limiter.AllowN(time.Now(), msg.Size()) {
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

// getLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (b *BandWidthRateLimiterImpl) getLimiter(peerID peer.ID) *rate.Limiter {
	if limiter, ok := b.limiters.get(peerID); ok {
		return limiter
	}

	limiter := rate.NewLimiter(b.limit, b.burst)
	b.limiters.store(peerID, limiter)

	return limiter
}
