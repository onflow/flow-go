package ingest

import (
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/model/flow"
)

// AddressRateLimiter limits the rate of ingested transactions with a given payer address.
type AddressRateLimiter struct {
	mu       sync.RWMutex
	limiters map[flow.Address]*rate.Limiter
	limit    rate.Limit // X messages allowed per second
	burst    int        // X messages allowed at one time
}

// AddressRateLimiter limits the rate of ingested transactions with a given payer address.
// It allows the given "limit" amount messages per second with a "burst" amount of messages to be sent at once
//
// for example,
// To config 1 message per 100 milliseconds, convert to per second first, which is 10 message per second,
// so limit is 10 ( rate.Limit(10) ), and burst is 1.
// Note: rate.Limit(0.1), burst = 1 means 1 message per 10 seconds, instead of 1 message per 100 milliseconds.
//
// To config 3 message per minute, the per-second-basis is 0.05 (3/60), so the limit should be rate.Limit(0.05),
// and burst is 3.
//
// Note: The rate limit configured for each node may differ from the effective network-wide rate limit
// for a given payer. In particular, the number of clusters and the message propagation factor will
// influence how the individual rate limit translates to a network-wide rate limit.
// For example, suppose we have 5 collection clusters and configure each Collection Node with a rate
// limit of 1 message per second. Then, the effective network-wide rate limit for a payer address would
// be *at least* 5 messages per second.
func NewAddressRateLimiter(limit rate.Limit, burst int) *AddressRateLimiter {
	return &AddressRateLimiter{
		limiters: make(map[flow.Address]*rate.Limiter),
		limit:    limit,
		burst:    burst,
	}
}

// Allow returns whether the given address should be allowed (not rate limited)
func (r *AddressRateLimiter) Allow(address flow.Address) bool {
	return !r.IsRateLimited(address)
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

// AddAddress add an address to be rate limited
func (r *AddressRateLimiter) AddAddress(address flow.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.limiters[address]
	if ok {
		return
	}

	r.limiters[address] = rate.NewLimiter(r.limit, r.burst)
}

// RemoveAddress remove an address for being rate limited
func (r *AddressRateLimiter) RemoveAddress(address flow.Address) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.limiters, address)
}

// GetAddresses get the list of rate limited address
func (r *AddressRateLimiter) GetAddresses() []flow.Address {
	r.mu.RLock()
	defer r.mu.RUnlock()

	addresses := make([]flow.Address, 0, len(r.limiters))
	for address := range r.limiters {
		addresses = append(addresses, address)
	}

	return addresses
}

// GetLimitConfig get the limit config
func (r *AddressRateLimiter) GetLimitConfig() (rate.Limit, int) {
	return r.limit, r.burst
}

// SetLimitConfig update the limit config
// Note all the existing limiters will be updated, and reset
func (r *AddressRateLimiter) SetLimitConfig(limit rate.Limit, burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for address := range r.limiters {
		r.limiters[address] = rate.NewLimiter(limit, burst)
	}

	r.limit = limit
	r.burst = burst
}

// Util functions
func AddAddresses(r *AddressRateLimiter, addresses []flow.Address) {
	for _, address := range addresses {
		r.AddAddress(address)
	}
}

func RemoveAddresses(r *AddressRateLimiter, addresses []flow.Address) {
	for _, address := range addresses {
		r.RemoveAddress(address)
	}
}

// parse addresses string into a list of flow addresses
func ParseAddresses(addresses string) ([]flow.Address, error) {
	addressList := make([]flow.Address, 0)
	for _, addr := range strings.Split(addresses, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		flowAddr, err := flow.StringToAddress(addr)
		if err != nil {
			return nil, err
		}
		addressList = append(addressList, flowAddr)
	}
	return addressList, nil
}
