package dns

import (
	"net"
	"sync"
	"time"
)

// DefaultTimeToLive is the default duration a dns result is cached.
const (
	// DefaultTimeToLive is the default duration a dns result is cached.
	DefaultTimeToLive = 5 * time.Minute
	cacheEntryExists  = true
	cacheEntryFresh   = true // TTL yet has not reached
)

// cache is a ttl-based cache for dns entries
type cache struct {
	ttl      time.Duration // time-to-live for cache entry
	ipCache  sync.Map
	txtCache sync.Map
}

func newCache() *cache {
	return &cache{
		ttl: DefaultTimeToLive,
	}
}

// resolveIPCache resolves the domain through the cache if it is available.
// First boolean variable determines whether the domain exists in the cache.
// Second boolean variable determines whether the domain cache is fresh, i.e., TTL has not yet reached.
func (c *cache) resolveIPCache(domain string) ([]net.IPAddr, bool, bool) {
	entity, ok := c.ipCache.Load(domain)
	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryFresh
	}

	entry := entity.(*ipCacheEntry)

	if time.Duration(runtimeNano()-entry.timestamp) > c.ttl {
		// exists but expired
		return entry.addresses, cacheEntryExists, !cacheEntryFresh
	}

	// exists and fresh
	return entry.addresses, cacheEntryExists, cacheEntryFresh
}

// resolveIPCache resolves the txt through the cache if it is available.
// First boolean variable determines whether the txt exists in the cache.
// Second boolean variable determines whether the txt cache is fresh, i.e., TTL has not yet reached.
func (c *cache) resolveTXTCache(txt string) ([]string, bool, bool) {
	entity, ok := c.txtCache.Load(txt)

	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryFresh
	}

	entry := entity.(*txtCacheEntry)

	if time.Duration(runtimeNano()-entry.timestamp) > c.ttl {
		// exists but expired
		return entry.addresses, cacheEntryExists, !cacheEntryFresh
	}

	// exists and fresh
	return entry.addresses, cacheEntryExists, cacheEntryFresh
}

// updateIPCache updates the cache entry for the domain.
func (c *cache) updateIPCache(domain string, addr []net.IPAddr) {
	c.ipCache.Store(domain, &ipCacheEntry{
		addresses: addr,
		timestamp: runtimeNano(),
	})
}

// updateTXTCache updates the cache entry for the txt.
func (c *cache) updateTXTCache(txt string, addr []string) {
	c.txtCache.Store(txt, &txtCacheEntry{
		addresses: addr,
		timestamp: runtimeNano(),
	})
}

// invalidateIPCacheEntry atomically invalidates ip cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateIPCacheEntry(domain string) bool {
	_, exists := c.ipCache.Load(domain)

	if !exists {
		return false
	}

	// delete is idempotent and once in a while it is ok to report as exist even
	// when it may have been deleted to avoid the cost of write lock
	c.ipCache.Delete(domain)

	return true
}

// invalidateTXTCacheEntry atomically invalidates txt cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateTXTCacheEntry(txt string) bool {
	_, exists := c.txtCache.Load(txt)

	if !exists {
		return false
	}

	// delete is idempotent and once in a while it is ok to report as exist even
	// when it may have been deleted to avoid the cost of write lock
	c.txtCache.Delete(txt)

	return exists
}
