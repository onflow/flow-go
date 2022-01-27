package dns

import (
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/herocache"
)

// DefaultTimeToLive is the default duration a dns result is cached.
const (
	// DefaultTimeToLive is the default duration a dns result is cached.
	DefaultTimeToLive = 5 * time.Minute
	DefaultCacheSize  = 10e3
	cacheEntryExists  = true
	cacheEntryFresh   = true // TTL yet has not reached
)

// cache is a ttl-based cache for dns entries
type cache struct {
	sync.RWMutex
	ttl    time.Duration // time-to-live for cache entry
	dCache mempool.DNSCache
}

func newCache(sizeLimit uint32, logger zerolog.Logger) *cache {
	return &cache{
		ttl:    DefaultTimeToLive,
		dCache: herocache.NewDNSCache(sizeLimit, logger),
	}
}

// resolveIPCache resolves the domain through the cache if it is available.
// First boolean variable determines whether the domain exists in the cache.
// Second boolean variable determines whether the domain cache is fresh, i.e., TTL has not yet reached.
func (c *cache) resolveIPCache(domain string) ([]net.IPAddr, bool, bool) {
	addresses, timeStamp, ok := c.dCache.GetIpDomain(domain)
	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryFresh
	}

	if time.Duration(runtimeNano()-timeStamp) > c.ttl {
		// exists but expired
		return addresses, cacheEntryExists, !cacheEntryFresh
	}

	// exists and fresh
	return addresses, cacheEntryExists, cacheEntryFresh
}

// resolveIPCache resolves the txt through the cache if it is available.
// First boolean variable determines whether the txt exists in the cache.
// Second boolean variable determines whether the txt cache is fresh, i.e., TTL has not yet reached.
func (c *cache) resolveTXTCache(txt string) ([]string, bool, bool) {
	addresses, timeStamp, ok := c.dCache.GetTxtDomain(txt)
	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryFresh
	}

	if time.Duration(runtimeNano()-timeStamp) > c.ttl {
		// exists but expired
		return addresses, cacheEntryExists, !cacheEntryFresh
	}

	// exists and fresh
	return addresses, cacheEntryExists, cacheEntryFresh
}

// updateIPCache updates the cache entry for the domain.
func (c *cache) updateIPCache(domain string, addr []net.IPAddr) {
	c.dCache.PutIpDomain(domain, runtimeNano(), addr)
}

// updateTXTCache updates the cache entry for the txt.
func (c *cache) updateTXTCache(txt string, addr []string) {
	c.dCache.PutTxtDomain(txt, runtimeNano(), addr)
}

// invalidateIPCacheEntry atomically invalidates ip cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateIPCacheEntry(domain string) bool {
	return c.dCache.RemoveIp(domain)
}

// invalidateTXTCacheEntry atomically invalidates txt cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateTXTCacheEntry(txt string) bool {
	return c.dCache.RemoveTxt(txt)
}
