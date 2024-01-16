package ingest

import (
	"strings"
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

// AddressRateLimiter limits the rate of messages sent to a given address
// It allows the given "limit" amount messages per second with a "burst" amount of messages to be sent at once
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
