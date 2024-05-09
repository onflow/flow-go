package ratelimit

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/utils/ratelimiter"
)

// BandWidthRateLimiter unicast rate limiter that limits the bandwidth that can be sent
// by a peer per some configured interval.
type BandWidthRateLimiter struct {
	*ratelimiter.RateLimiter
}

// NewBandWidthRateLimiter returns a new BandWidthRateLimiter. The cleanup loop will be started in a
// separate goroutine and should be stopped by calling Close.
func NewBandWidthRateLimiter(limit rate.Limit, burst int, lockout time.Duration, opts ...p2p.RateLimiterOpt) *BandWidthRateLimiter {
	l := &BandWidthRateLimiter{
		RateLimiter: ratelimiter.NewRateLimiter(limit, burst, lockout, opts...),
	}

	return l
}

// Allow checks the cached limiter for the peer and returns limiter.AllowN(msg.Size())
// which will check if a peer is able to send a message of msg.Size().
// If a limiter is not cached one is created.
func (b *BandWidthRateLimiter) Allow(peerID peer.ID, msgSize int) bool {
	limiter := b.GetLimiter(peerID)
	if !limiter.AllowN(time.Now(), msgSize) {
		b.UpdateLastRateLimit(peerID, time.Now())
		return false
	}

	return true
}
