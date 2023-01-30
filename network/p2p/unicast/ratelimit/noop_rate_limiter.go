package ratelimit

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
)

type NoopRateLimiter struct{}

func (n *NoopRateLimiter) Allow(_ peer.ID, _ int) bool {
	return true
}

func (n *NoopRateLimiter) IsRateLimited(_ peer.ID) bool {
	return false
}

func (n *NoopRateLimiter) SetTimeNowFunc(_ p2p.GetTimeNow) {}

func (n *NoopRateLimiter) Stop() {}

func (n *NoopRateLimiter) Start() {}

func NewNoopRateLimiter() *NoopRateLimiter {
	return &NoopRateLimiter{}
}

// NoopRateLimiters returns noop rate limiters.
func NoopRateLimiters() *RateLimiters {
	return &RateLimiters{
		MessageRateLimiter:   &NoopRateLimiter{},
		BandWidthRateLimiter: &NoopRateLimiter{},
		disabled:             true,
		notifier:             NewUnicastRateLimiterDistributor(),
	}
}
