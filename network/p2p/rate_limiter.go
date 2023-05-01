package p2p

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/component"
)

// RateLimiter rate limiter with lockout feature that can be used via the IsRateLimited method.
// This limiter allows users to flag a peer as rate limited for a lockout duration.
type RateLimiter interface {
	BasicRateLimiter
	// IsRateLimited returns true if a peer is rate limited.
	IsRateLimited(peerID peer.ID) bool
}

// BasicRateLimiter rate limiter interface
type BasicRateLimiter interface {
	component.Component
	// Allow returns true if a message with the give size should be allowed to be processed.
	Allow(peerID peer.ID, msgSize int) bool
}

type RateLimiterOpt func(limiter RateLimiter)

// UnicastRateLimiterDistributor consumes then distributes notifications from the ratelimit.RateLimiters whenever a peer is rate limited.
type UnicastRateLimiterDistributor interface {
	RateLimiterConsumer
	AddConsumer(consumer RateLimiterConsumer)
}

// RateLimiterConsumer consumes notifications from the ratelimit.RateLimiters whenever a peer is rate limited.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type RateLimiterConsumer interface {
	OnRateLimitedPeer(pid peer.ID, role, msgType, topic, reason string)
}
