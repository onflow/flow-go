package dns

import (
	"context"
	"net"
	"sync"
	"time"
	_ "unsafe" // for linking runtimeNano

	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/onflow/flow-go/module"
)

//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

// defaultTimeToLive is the default duration a dns result is cached.
const defaultTimeToLive = 5 * time.Minute

// Resolver is a cache-based dns resolver for libp2p.
type Resolver struct {
	sync.RWMutex
	ttl       time.Duration // time-to-live for cache entry
	res       madns.BasicResolver
	collector module.ResolverMetrics
	ipCache   map[string]*ipCacheEntry
	txtCache  map[string]*txtCacheEntry
}

type ipCacheEntry struct {
	addresses []net.IPAddr
	timestamp int64
}

type txtCacheEntry struct {
	addresses []string
	timestamp int64
}

type optFunc func(resolver *Resolver)

func WithBasicResolver(basic madns.BasicResolver) func(resolver *Resolver) {
	return func(resolver *Resolver) {
		resolver.res = basic
	}
}

func WithTTL(ttl time.Duration) func(resolver *Resolver) {
	return func(resolver *Resolver) {
		resolver.ttl = ttl
	}
}

func NewResolver(collector module.ResolverMetrics, opts ...optFunc) (*madns.Resolver, error) {
	resolver := &Resolver{
		res:       madns.DefaultResolver,
		ttl:       defaultTimeToLive,
		collector: collector,
		ipCache:   make(map[string]*ipCacheEntry),
		txtCache:  make(map[string]*txtCacheEntry),
	}

	for _, opt := range opts {
		opt(resolver)
	}

	return madns.NewResolver(madns.WithDefaultResolver(resolver))
}

// LookupIPAddr implements BasicResolver interface for libp2p.
func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	r.Lock()
	defer r.Unlock()

	started := runtimeNano()

	addr, err := r.lookupIPAddr(ctx, domain)

	r.collector.DNSLookupDuration(
		time.Duration(runtimeNano() - started))
	return addr, err
}

func (r *Resolver) lookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	if addr, ok := r.resolveIPCache(domain); ok {
		// resolving address from cache
		r.collector.OnDNSCacheHit()
		return addr, nil
	}

	// resolves domain through underlying resolver
	r.collector.OnDNSCacheMiss()
	addr, err := r.res.LookupIPAddr(ctx, domain)
	if err != nil {
		return nil, err
	}

	r.updateIPCache(domain, addr) // updates cache

	return addr, nil
}

// LookupTXT implements BasicResolver interface for libp2p.
func (r *Resolver) LookupTXT(ctx context.Context, txt string) ([]string, error) {
	r.Lock()
	defer r.Unlock()

	started := runtimeNano()

	addr, err := r.lookupTXT(ctx, txt)

	r.collector.DNSLookupDuration(
		time.Duration(runtimeNano() - started))
	return addr, err
}

func (r *Resolver) lookupTXT(ctx context.Context, txt string) ([]string, error) {
	if addr, ok := r.resolveTXTCache(txt); ok {
		// resolving address from cache
		r.collector.OnDNSCacheHit()
		return addr, nil
	}

	// resolves txt through underlying resolver
	r.collector.OnDNSCacheMiss()
	addr, err := r.res.LookupTXT(ctx, txt)
	if err != nil {
		return nil, err
	}

	r.updateTXTCache(txt, addr) // updates cache

	return addr, err
}

// resolveIPCache resolves the domain through the cache if it is available.
func (r *Resolver) resolveIPCache(domain string) ([]net.IPAddr, bool) {
	entry, ok := r.ipCache[domain]

	if !ok {
		return nil, false
	}

	if time.Duration(runtimeNano()-entry.timestamp) > r.ttl {
		// invalidates cache entry
		delete(r.ipCache, domain)
		return nil, false
	}

	return entry.addresses, true
}

// resolveIPCache resolves the txt through the cache if it is available.
func (r *Resolver) resolveTXTCache(txt string) ([]string, bool) {
	entry, ok := r.txtCache[txt]

	if !ok {
		return nil, false
	}

	if time.Duration(runtimeNano()-entry.timestamp) > r.ttl {
		// invalidates cache entry
		delete(r.txtCache, txt)
		return nil, false
	}

	return entry.addresses, true
}

// updateIPCache updates the cache entry for the domain.
func (r *Resolver) updateIPCache(domain string, addr []net.IPAddr) {
	r.ipCache[domain] = &ipCacheEntry{
		addresses: addr,
		timestamp: runtimeNano(),
	}
}

// updateTXTCache updates the cache entry for the txt.
func (r *Resolver) updateTXTCache(txt string, addr []string) {
	r.txtCache[txt] = &txtCacheEntry{
		addresses: addr,
		timestamp: runtimeNano(),
	}
}
