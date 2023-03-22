package ratelimit

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/irrecoverable"
)

type NoopRateLimiter struct{}

func (n *NoopRateLimiter) Allow(peer.ID, int) bool {
	return true
}
func (n *NoopRateLimiter) IsRateLimited(peer.ID) bool {
	return false
}
func (n *NoopRateLimiter) CleanupLoop(irrecoverable.SignalerContext) {}
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
