package ratelimit

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
)

type NoopRateLimiter struct{}

func (n *NoopRateLimiter) Allow(peer.ID, int) bool {
	return true
}
func (n *NoopRateLimiter) IsRateLimited(peer.ID) bool {
	return false
}
func (n *NoopRateLimiter) SetTimeNowFunc(p2p.GetTimeNow)             {}
func (n *NoopRateLimiter) CleanupLoop(irrecoverable.SignalerContext) {}
func (n *NoopRateLimiter) Now() time.Time {
	return time.Now()
}
func NewNoopRateLimiter() *NoopRateLimiter {
	return &NoopRateLimiter{}
}

// NoopRateLimiters returns noop rate limiters.
func NoopRateLimiters() *RateLimiters {
	return &RateLimiters{
		MessageRateLimiter:   NewNoopRateLimiter(),
		BandWidthRateLimiter: NewNoopRateLimiter(),
		disabled:             true,
		notifier:             NewUnicastRateLimiterDistributor(),
	}
}
