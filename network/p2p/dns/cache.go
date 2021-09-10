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
	cacheEntryExpired = true
)

// cache is a ttl-based cache for dns entries
type cache struct {
	sync.RWMutex
	ttl      time.Duration // time-to-live for cache entry
	ipCache  map[string]*ipCacheEntry
	txtCache map[string]*txtCacheEntry
}

func newCache() *cache {
	return &cache{
		ttl:      DefaultTimeToLive,
		ipCache:  make(map[string]*ipCacheEntry),
		txtCache: make(map[string]*txtCacheEntry),
	}
}

// resolveIPCache resolves the domain through the cache if it is available.
func (c *cache) resolveIPCache(domain string) ([]net.IPAddr, bool, bool) {
	c.RLock()
	defer c.RUnlock()

	entry, ok := c.ipCache[domain]

	if !ok {
		// does not exist
		return nil, !cacheEntryExists, cacheEntryExpired
	}

	if time.Duration(runtimeNano()-entry.timestamp) > c.ttl {
		// exists but expired
		return entry.addresses, cacheEntryExists, cacheEntryExpired
	}

	// exists and fresh
	return entry.addresses, cacheEntryExists, !cacheEntryExpired
}

// resolveIPCache resolves the txt through the cache if it is available.
func (c *cache) resolveTXTCache(txt string) ([]string, bool, bool) {
	c.RLock()
	defer c.RUnlock()

	entry, ok := c.txtCache[txt]

	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryExpired
	}

	if time.Duration(runtimeNano()-entry.timestamp) > c.ttl {
		// exists but expired
		return entry.addresses, cacheEntryExists, cacheEntryExpired
	}

	// exists and fresh
	return entry.addresses, cacheEntryExists, !cacheEntryExpired
}

// updateIPCache updates the cache entry for the domain.
func (c *cache) updateIPCache(domain string, addr []net.IPAddr) {
	c.Lock()
	defer c.Unlock()

	c.ipCache[domain] = &ipCacheEntry{
		addresses: addr,
		timestamp: runtimeNano(),
	}
}

// updateTXTCache updates the cache entry for the txt.
func (c *cache) updateTXTCache(txt string, addr []string) {
	c.Lock()
	defer c.Unlock()

	c.txtCache[txt] = &txtCacheEntry{
		addresses: addr,
		timestamp: runtimeNano(),
	}
}

// invalidateIPCacheEntry atomically invalidates ip cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateIPCacheEntry(domain string) bool {
	c.Lock()
	defer c.Unlock()

	_, exists := c.ipCache[domain]

	delete(c.ipCache, domain)

	return exists
}

// invalidateTXTCacheEntry atomically invalidates txt cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateTXTCacheEntry(txt string) bool {
	c.Lock()
	defer c.Unlock()

	_, exists := c.txtCache[txt]

	delete(c.txtCache, txt)

	return exists
}
