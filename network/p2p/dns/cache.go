package dns

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/mempool"
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
	logger zerolog.Logger
}

func newCache(logger zerolog.Logger, dnsCache mempool.DNSCache) *cache {
	return &cache{
		ttl:    DefaultTimeToLive,
		dCache: dnsCache,
		logger: logger.With().Str("component", "dns-cache").Logger(),
	}
}

// resolveIPCache resolves the domain through the cache if it is available.
// First boolean variable determines whether the domain exists in the cache.
// Second boolean variable determines whether the domain cache is fresh, i.e., TTL has not yet reached.
// Third boolean variable determines whether the domain record is currently being resolved by another thread.
func (c *cache) resolveIPCache(domain string) ([]net.IPAddr, bool, bool, bool) {
	c.RLock()
	defer c.RUnlock()

	record, ok := c.dCache.GetDomainIp(domain)
	currentTimeStamp := runtimeNano()
	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryFresh, false
	}
	c.logger.Trace().
		Str("domain", domain).
		Str("address", fmt.Sprintf("%v", record.Addresses)).
		Int64("record_timestamp", record.Timestamp).
		Int64("current_timestamp", currentTimeStamp).
		Bool("record_locked", record.Locked).
		Msg("dns record retrieved")

	if time.Duration(currentTimeStamp-record.Timestamp) > c.ttl {
		// exists but expired
		return record.Addresses, cacheEntryExists, !cacheEntryFresh, record.Locked
	}

	// exists and fresh
	return record.Addresses, cacheEntryExists, cacheEntryFresh, record.Locked
}

// resolveIPCache resolves the txt through the cache if it is available.
// First boolean variable determines whether the txt record exists in the cache.
// Second boolean variable determines whether the txt record cache is fresh, i.e., TTL has not yet reached.
// Third boolean variable determines whether the domain record is currently being resolved by another thread.
func (c *cache) resolveTXTCache(txt string) ([]string, bool, bool, bool) {
	c.RLock()
	defer c.RUnlock()

	record, ok := c.dCache.GetTxtRecord(txt)
	currentTimeStamp := runtimeNano()
	if !ok {
		// does not exist
		return nil, !cacheEntryExists, !cacheEntryFresh, false
	}
	c.logger.Trace().
		Str("txt", txt).
		Str("address", fmt.Sprintf("%v", record.Record)).
		Int64("record_timestamp", record.Timestamp).
		Int64("current_timestamp", currentTimeStamp).
		Bool("record_locked", record.Locked).
		Msg("dns record retrieved")

	if time.Duration(currentTimeStamp-record.Timestamp) > c.ttl {
		// exists but expired
		return record.Record, cacheEntryExists, !cacheEntryFresh, record.Locked
	}

	// exists and fresh
	return record.Record, cacheEntryExists, cacheEntryFresh, record.Locked
}

// updateIPCache updates the cache entry for the domain.
func (c *cache) updateIPCache(domain string, addr []net.IPAddr) {
	c.Lock()
	defer c.Unlock()

	timestamp := runtimeNano()
	removed := c.dCache.RemoveIp(domain)
	added := c.dCache.PutIpDomain(domain, addr, runtimeNano())

	ipSize, txtSize := c.dCache.Size()
	c.logger.Trace().
		Str("domain", domain).
		Str("address", fmt.Sprintf("%v", addr)).
		Bool("old_entry_removed", removed).
		Bool("new_entry_added", added).
		Int64("timestamp", timestamp).
		Uint("ip_size", ipSize).
		Uint("txt_size", txtSize).
		Msg("dns cache updated")
}

// updateTXTCache updates the cache entry for the txt record.
func (c *cache) updateTXTCache(txt string, record []string) {
	c.Lock()
	defer c.Unlock()

	timestamp := runtimeNano()
	removed := c.dCache.RemoveTxt(txt)
	added := c.dCache.PutTxtRecord(txt, record, runtimeNano())

	ipSize, txtSize := c.dCache.Size()
	c.logger.Trace().
		Str("txt", txt).
		Strs("record", record).
		Bool("old_entry_removed", removed).
		Bool("new_entry_added", added).
		Int64("timestamp", timestamp).
		Uint("ip_size", ipSize).
		Uint("txt_size", txtSize).
		Msg("dns cache updated")
}

// invalidateIPCacheEntry atomically invalidates ip cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateIPCacheEntry(domain string) bool {
	c.Lock()
	defer c.Unlock()

	return c.dCache.RemoveIp(domain)
}

// shouldResolveIP returns true if there is no other concurrent attempt ongoing for resolving the domain.
func (c *cache) shouldResolveIP(domain string) bool {
	lg := c.logger.With().
		Str("domain", domain).Logger()

	locked, err := c.dCache.LockIPDomain(domain)
	if err != nil {
		lg.Error().Err(err).Msg("cannot lock ip domain")
	}

	lg.Trace().Bool("locked", locked).Msg("attempt on locking ip domain")
	return locked
}

// shouldResolveTxt returns true if there is no other concurrent attempt ongoing for resolving the txt.
func (c *cache) shouldResolveTXT(txt string) bool {
	lg := c.logger.With().
		Str("txt", txt).Logger()

	locked, err := c.dCache.LockTxtRecord(txt)
	if err != nil {
		lg.Error().Err(err).Msg("cannot lock txt domain")
	}

	lg.Trace().Bool("locked", locked).Msg("attempt on locking txt domain")
	return locked
}

// invalidateTXTCacheEntry atomically invalidates txt cache entry. Boolean variable determines whether invalidation
// is successful.
func (c *cache) invalidateTXTCacheEntry(txt string) bool {
	c.Lock()
	defer c.Unlock()

	return c.dCache.RemoveTxt(txt)
}
