package dns

import (
	"fmt"
	"net"
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

// ipResolutionResult encapsulates result of resolving an ip address within dns cache.
type ipResolutionResult struct {
	addresses []net.IPAddr
	exists    bool // determines whether the domain exists in the cache.
	fresh     bool // determines whether the domain cache is fresh, i.e., TTL has not yet reached.
	locked    bool // determines whether the domain record is currently being resolved by another thread.
}

// txtResolutionResult encapsulates result of resolving a txt record within dns cache.
type txtResolutionResult struct {
	records []string
	exists  bool // determines whether the domain exists in the cache.
	fresh   bool // determines whether the domain cache is fresh, i.e., TTL has not yet reached.
	locked  bool // determines whether the domain record is currently being resolved by another thread.
}

// cache is a ttl-based cache for dns entries
type cache struct {
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
func (c *cache) resolveIPCache(domain string) *ipResolutionResult {
	record, ok := c.dCache.GetDomainIp(domain)
	currentTimeStamp := runtimeNano()
	if !ok {
		// does not exist
		return &ipResolutionResult{
			addresses: nil,
			exists:    !cacheEntryExists,
			fresh:     !cacheEntryFresh,
			locked:    false,
		}
	}
	c.logger.Trace().
		Str("domain", domain).
		Str("address", fmt.Sprintf("%v", record.Addresses)).
		Int64("record_timestamp", record.Timestamp).
		Int64("current_timestamp", currentTimeStamp).
		Bool("record_locked", record.Locked).
		Msg("dns record retrieved")

	if time.Duration(currentTimeStamp-record.Timestamp) > c.ttl {
		// exists but fresh
		return &ipResolutionResult{
			addresses: record.Addresses,
			exists:    cacheEntryExists,
			fresh:     !cacheEntryFresh,
			locked:    record.Locked,
		}
	}

	// exists and fresh
	return &ipResolutionResult{
		addresses: record.Addresses,
		exists:    cacheEntryExists,
		fresh:     cacheEntryFresh,
		locked:    record.Locked,
	}
}

// resolveIPCache resolves the txt through the cache if it is available.
func (c *cache) resolveTXTCache(txt string) *txtResolutionResult {
	record, ok := c.dCache.GetTxtRecord(txt)
	currentTimeStamp := runtimeNano()
	if !ok {
		// does not exist
		return &txtResolutionResult{
			records: nil,
			exists:  !cacheEntryExists,
			fresh:   !cacheEntryFresh,
			locked:  false,
		}
	}
	c.logger.Trace().
		Str("txt", txt).
		Str("address", fmt.Sprintf("%v", record.Records)).
		Int64("record_timestamp", record.Timestamp).
		Int64("current_timestamp", currentTimeStamp).
		Bool("record_locked", record.Locked).
		Msg("dns record retrieved")

	if time.Duration(currentTimeStamp-record.Timestamp) > c.ttl {
		// exists but fresh
		return &txtResolutionResult{
			records: record.Records,
			exists:  cacheEntryExists,
			fresh:   !cacheEntryFresh,
			locked:  record.Locked,
		}
	}

	// exists and fresh
	return &txtResolutionResult{
		records: record.Records,
		exists:  cacheEntryExists,
		fresh:   cacheEntryFresh,
		locked:  record.Locked,
	}
}

// updateIPCache updates the cache entry for the domain.
func (c *cache) updateIPCache(domain string, addr []net.IPAddr) {
	lg := c.logger.With().
		Str("domain", domain).
		Str("address", fmt.Sprintf("%v", addr)).Logger()

	timestamp := runtimeNano()

	err := c.dCache.UpdateIPDomain(domain, addr, runtimeNano())
	if err != nil {
		lg.Error().Err(err).Msg("could not update ip record")
		return
	}

	ipSize, txtSize := c.dCache.Size()
	lg.Trace().
		Int64("timestamp", timestamp).
		Uint("ip_size", ipSize).
		Uint("txt_size", txtSize).
		Msg("dns cache updated")
}

// updateTXTCache updates the cache entry for the txt record.
func (c *cache) updateTXTCache(txt string, record []string) {
	lg := c.logger.With().
		Str("txt", txt).
		Strs("record", record).Logger()

	timestamp := runtimeNano()
	err := c.dCache.UpdateTxtRecord(txt, record, runtimeNano())
	if err != nil {
		lg.Error().Err(err).Msg("could not update txt record")
		return
	}

	ipSize, txtSize := c.dCache.Size()
	lg.Trace().
		Int64("timestamp", timestamp).
		Uint("ip_size", ipSize).
		Uint("txt_size", txtSize).
		Msg("dns cache updated")
}

// shouldResolveIP returns true if there is no other concurrent attempt ongoing for resolving the domain.
func (c *cache) shouldResolveIP(domain string) bool {
	lg := c.logger.With().
		Str("domain", domain).Logger()

	locked, err := c.dCache.LockIPDomain(domain)
	if err != nil {
		lg.Error().Err(err).Msg("cannot lock ip domain")
		return false
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
		return false
	}

	lg.Trace().Bool("locked", locked).Msg("attempt on locking txt domain")
	return locked
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
