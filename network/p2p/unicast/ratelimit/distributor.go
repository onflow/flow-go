package ratelimit

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
)

// UnicastRateLimiterDistributor subscribes to rate limited peer events from RateLimiters.
type UnicastRateLimiterDistributor struct {
	consumers []p2p.RateLimiterConsumer
	lock      sync.RWMutex
}

var _ p2p.RateLimiterConsumer = (*UnicastRateLimiterDistributor)(nil)

// NewUnicastRateLimiterDistributor returns a new UnicastRateLimiterDistributor.
func NewUnicastRateLimiterDistributor() *UnicastRateLimiterDistributor {
	return &UnicastRateLimiterDistributor{
		consumers: make([]p2p.RateLimiterConsumer, 0),
	}
}

// AddConsumer adds a consumer to the consumers list.
func (r *UnicastRateLimiterDistributor) AddConsumer(consumer p2p.RateLimiterConsumer) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.consumers = append(r.consumers, consumer)
}

// OnRateLimitedPeer invokes each consumer callback with the rate limited peer info.
func (r *UnicastRateLimiterDistributor) OnRateLimitedPeer(pid peer.ID, role, msgType, topic, reason string) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	for _, consumer := range r.consumers {
		consumer.OnRateLimitedPeer(pid, role, msgType, topic, reason)
	}
}
