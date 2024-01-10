package ingest

import (
	"sync"

	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/model/flow"
)

type AddressRateLimiter struct {
	mu       sync.RWMutex
	limiters map[flow.Address]*rate.Limiter
	limit    rate.Limit
}

func NewAddressRateLimiter(limit rate.Limit) *AddressRateLimiter {
	return &AddressRateLimiter{
		limiters: make(map[flow.Address]*rate.Limiter),
		limit:    limit,
	}
}

func (r *AddressRateLimiter) IsRateLimited(address flow.Address) bool {
	limiter := r.getLimiter(address)
	rateLimited := !limiter.Allow()
	return rateLimited
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

func (r *AddressRateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for address, limiter := range r.limiters {
		// if there is 1 token, the limiter can be removed, because it's
		// equvilent to creating a new one
		token := limiter.Tokens()
		if token == 1 {
			delete(r.limiters, address)
		}
	}
}
