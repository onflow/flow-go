package ratelimit

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type NoopRateLimiter struct {
	component.Component
}

func (n *NoopRateLimiter) Allow(peer.ID, int) bool {
	return true
}
func (n *NoopRateLimiter) IsRateLimited(peer.ID) bool {
	return false
}

func (n *NoopRateLimiter) Start(irrecoverable.SignalerContext) {}

func NewNoopRateLimiter() *NoopRateLimiter {
	return &NoopRateLimiter{
		Component: &module.NoopComponent{},
	}
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
