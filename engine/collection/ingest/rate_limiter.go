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
	burst    int // X messages allowed at one time
}

func NewAddressRateLimiter(limit rate.Limit, burst int) *AddressRateLimiter {
	return &AddressRateLimiter{
		limiters: make(map[flow.Address]*rate.Limiter),
		limit:    limit,
		burst:    burst,
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

	r.limiters[address] = rate.NewLimiter(r.limit, r.burst)
}

// RemoveAddress remove an address for being rate limitted
func (r *AddressRateLimiter) RemoveAddress(address flow.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.limiters, address)
}
