package ingest

import (
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/model/flow"
)

type AddressRateLimiter struct {
	mu       sync.RWMutex
	limiters map[flow.Address]*rate.Limiter

	ttl             time.Duration
	cleanupInterval time.Duration
	limit           rate.Limit
}

func (r *AddressRateLimiter) IsRateLimited(address flow.Address) bool {
	limiter := r.getLimiter(address)
	return limiter.Allow()
}

func (r *AddressRateLimiter) getLimiter(address flow.Address) *rate.Limiter {
	r.mu.RLock()
	limiter, ok := r.limiters[address]
	r.mu.RUnlock()

	if !ok {
		r.mu.Lock()
		limiter, ok = r.limiters[address]
		if !ok {
			limiter = rate.NewLimiter(r.limit, 1)
			r.limiters[address] = limiter
		}
		r.mu.Unlock()
	}

	return limiter
}

// func (r *AddressRateLimiter) cleanup(
