package dns

import (
	"context"
	"net"
	"time"
	_ "unsafe" // for linking runtimeNano

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/util"
)

//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

// Resolver is a cache-based dns resolver for libp2p.
// DNS cache implementation notes:
//  1. Generic / possibly expected functionality NOT implemented:
//     - Caches domains for TTL seconds as given by upstream DNS resolver, e.g. [1].
//     - Possibly pre-expire cached domains so no connection time resolve delay.
//  2. Actual / pragmatic functionality implemented below:
//     - Caches domains for global (not individual domain record TTL) TTL seconds.
//     - Cached IP is returned even if cached entry expired; so no connection time resolve delay.
//     - Detecting expired cached domain triggers async DNS lookup to refresh cached entry.
//
// [1] https://en.wikipedia.org/wiki/Name_server#Caching_name_server
type Resolver struct {
	c           *cache
	res         madns.BasicResolver // underlying resolver
	collector   module.ResolverMetrics
	ipRequests  chan *lookupIPRequest
	txtRequests chan *lookupTXTRequest
	logger      zerolog.Logger
	component.Component
	cm *component.ComponentManager
}

type lookupIPRequest struct {
	domain string
}

type lookupTXTRequest struct {
	txt string
}

// optFunc is the option function for Resolver.
type optFunc func(resolver *Resolver)

// WithBasicResolver is an option function for setting the basic resolver of this Resolver.
func WithBasicResolver(basic madns.BasicResolver) optFunc {
	return func(resolver *Resolver) {
		resolver.res = basic
	}
}

// WithTTL is an option function for setting the time to live for cache entries.
func WithTTL(ttl time.Duration) optFunc {
	return func(resolver *Resolver) {
		resolver.c.ttl = ttl
	}
}

const (
	numIPAddrLookupWorkers = 16
	numTxtLookupWorkers    = 16
	ipAddrLookupQueueSize  = 64
	txtLookupQueueSize     = 64
)

// NewResolver is the factory function for creating an instance of this resolver.
func NewResolver(logger zerolog.Logger, collector module.ResolverMetrics, dnsCache mempool.DNSCache, opts ...optFunc) *Resolver {
	resolver := &Resolver{
		logger:      logger.With().Str("component", "dns-resolver").Logger(),
		res:         madns.DefaultResolver,
		c:           newCache(logger, dnsCache),
		collector:   collector,
		ipRequests:  make(chan *lookupIPRequest, ipAddrLookupQueueSize),
		txtRequests: make(chan *lookupTXTRequest, txtLookupQueueSize),
	}

	cm := component.NewComponentManagerBuilder()

	for i := 0; i < numIPAddrLookupWorkers; i++ {
		cm.AddWorker(resolver.processIPAddrLookups)
	}

	for i := 0; i < numTxtLookupWorkers; i++ {
		cm.AddWorker(resolver.processTxtLookups)
	}

	resolver.cm = cm.Build()
	resolver.Component = resolver.cm

	for _, opt := range opts {
		opt(resolver)
	}

	return resolver
}

func (r *Resolver) processIPAddrLookups(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	r.logger.Trace().Msg("processing ip worker started")

	for {
		select {
		case req := <-r.ipRequests:
			lg := r.logger.With().Str("domain", req.domain).Logger()
			lg.Trace().Msg("ip domain request picked for resolving")
			_, err := r.lookupResolverForIPAddr(ctx, req.domain)
			if err != nil {
				// invalidates cached entry when hits error on resolving.
				invalidated := r.c.invalidateIPCacheEntry(req.domain)
				if invalidated {
					r.collector.OnDNSCacheInvalidated()
				}
				lg.Error().Err(err).Msg("resolving ip address faced an error")
			}
			lg.Trace().Msg("ip domain resolved successfully")
		case <-ctx.Done():
			r.logger.Trace().Msg("processing ip worker terminated")
			return
		}
	}
}

func (r *Resolver) processTxtLookups(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	r.logger.Trace().Msg("processing txt worker started")

	for {
		select {
		case req := <-r.txtRequests:
			lg := r.logger.With().Str("domain", req.txt).Logger()
			lg.Trace().Msg("txt domain picked for resolving")
			_, err := r.lookupResolverForTXTRecord(ctx, req.txt)
			if err != nil {
				// invalidates cached entry when hits error on resolving.
				invalidated := r.c.invalidateTXTCacheEntry(req.txt)
				if invalidated {
					r.collector.OnDNSCacheInvalidated()
				}
				lg.Error().Err(err).Msg("resolving txt domain faced an error")
			}
			lg.Trace().Msg("txt domain resolved successfully")
		case <-ctx.Done():
			r.logger.Trace().Msg("processing txt worker terminated")
			return
		}
	}
}

// LookupIPAddr implements BasicResolver interface for libp2p for looking up ip addresses through resolver.
func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	started := runtimeNano()

	addr, err := r.lookupIPAddr(ctx, domain)

	r.collector.DNSLookupDuration(
		time.Duration(runtimeNano() - started))
	return addr, err
}

// lookupIPAddr encapsulates the logic of resolving an ip address through cache.
// If domain exists on cache it is resolved through the cache.
// An expired domain on cache is still addressed through the cache, however, a request is fired up asynchronously
// through the underlying basic resolver to resolve it from the network.
func (r *Resolver) lookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	result := r.c.resolveIPCache(domain)

	lg := r.logger.With().
		Str("domain", domain).
		Bool("cache_hit", result.exists).
		Bool("cache_fresh", result.fresh).
		Bool("locked_for_resolving", result.locked).Logger()

	lg.Trace().Msg("ip lookup request arrived")

	if !result.exists {
		r.collector.OnDNSCacheMiss()
		return r.lookupResolverForIPAddr(ctx, domain)
	}

	if !result.fresh && result.locked {
		lg.Trace().Msg("ip expired, but a resolving is in progress, returning expired one for now")
		return result.addresses, nil
	}

	if !result.fresh && r.c.shouldResolveIP(domain) && !util.CheckClosed(r.cm.ShutdownSignal()) {
		select {
		case r.ipRequests <- &lookupIPRequest{domain}:
			lg.Trace().Msg("ip lookup request queued for resolving")
		default:
			lg.Warn().Msg("ip lookup request queue is full, dropping request")
			r.collector.OnDNSLookupRequestDropped()
		}
	}

	r.collector.OnDNSCacheHit()
	return result.addresses, nil
}

// lookupResolverForIPAddr queries the underlying resolver for the domain and updates the cache if query is successful.
func (r *Resolver) lookupResolverForIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	addr, err := r.res.LookupIPAddr(ctx, domain)
	if err != nil {
		return nil, err
	}

	r.c.updateIPCache(domain, addr) // updates cache
	r.logger.Info().Str("ip_domain", domain).Msg("domain updated in cache")
	return addr, nil
}

// LookupTXT implements BasicResolver interface for libp2p.
// If txt exists on cache it is resolved through the cache.
// An expired txt on cache is still addressed through the cache, however, a request is fired up asynchronously
// through the underlying basic resolver to resolve it from the network.
func (r *Resolver) LookupTXT(ctx context.Context, txt string) ([]string, error) {

	started := runtimeNano()

	addr, err := r.lookupTXT(ctx, txt)

	r.collector.DNSLookupDuration(
		time.Duration(runtimeNano() - started))
	return addr, err
}

// lookupIPAddr encapsulates the logic of resolving a txt through cache.
func (r *Resolver) lookupTXT(ctx context.Context, txt string) ([]string, error) {
	result := r.c.resolveTXTCache(txt)

	lg := r.logger.With().
		Str("txt", txt).
		Bool("cache_hit", result.exists).
		Bool("cache_fresh", result.fresh).
		Bool("locked_for_resolving", result.locked).Logger()

	lg.Trace().Msg("txt lookup request arrived")

	if !result.exists {
		r.collector.OnDNSCacheMiss()
		return r.lookupResolverForTXTRecord(ctx, txt)
	}

	if !result.fresh && result.locked {
		lg.Trace().Msg("txt expired, but a resolving is in progress, returning expired one for now")
		return result.records, nil
	}

	if !result.fresh && r.c.shouldResolveTXT(txt) && !util.CheckClosed(r.cm.ShutdownSignal()) {
		select {
		case r.txtRequests <- &lookupTXTRequest{txt}:
			lg.Trace().Msg("ip lookup request queued for resolving")
		default:
			lg.Warn().Msg("txt lookup request queue is full, dropping request")
			r.collector.OnDNSLookupRequestDropped()
		}
	}

	r.collector.OnDNSCacheHit()
	return result.records, nil
}

// lookupResolverForTXTRecord queries the underlying resolver for the domain and updates the cache if query is successful.
func (r *Resolver) lookupResolverForTXTRecord(ctx context.Context, txt string) ([]string, error) {
	addr, err := r.res.LookupTXT(ctx, txt)
	if err != nil {
		return nil, err
	}

	r.c.updateTXTCache(txt, addr) // updates cache
	r.logger.Info().Str("txt_domain", txt).Msg("domain updated in cache")
	return addr, nil
}
