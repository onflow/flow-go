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

// IsRateLimited returns whether the given address should be rate limited
func (r *AddressRateLimiter) IsRateLimited(address flow.Address) bool {
	r.mu.RLock()
	limiter, ok := r.limiters[address]
	r.mu.RUnlock()

	if !ok {
		return false
	}

	rateLimited := !limiter.Allow()
	return rateLimited
}

// AddAddress add an address to be rate limitted
func (r *AddressRateLimiter) AddAddress(address flow.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.limiters[address]
	if ok {
		return
	}

	r.limiters[address] = rate.NewLimiter(r.limit, 1)
}

// RemoveAddress remove an address for being rate limitted
func (r *AddressRateLimiter) RemoveAddress(address flow.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.limiters, address)
}
