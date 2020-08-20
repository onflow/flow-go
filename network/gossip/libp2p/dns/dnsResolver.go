package dns

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"
)

const worker_count = 3

type lookUpResult struct {
	hostname multiaddr.Multiaddr
	ipAddr   multiaddr.Multiaddr
	err      error
}

type DnsResolver struct {
	dnsMap   DnsMap
	resolver *madns.Resolver
	log      zerolog.Logger
}

func NewDnsResolver(resolver *madns.Resolver, dnsMap DnsMap, log zerolog.Logger) DnsResolver {
	return DnsResolver{
		dnsMap:   dnsMap,
		resolver: resolver,
		log:      log,
	}
}

func (d *DnsResolver) worker(ctx context.Context,
	hostnames <-chan multiaddr.Multiaddr,
	results chan<- lookUpResult,
	wg *sync.WaitGroup) {
	defer wg.Done()
	for h := range hostnames {
		resolvedMa, err := d.resolver.Resolve(ctx, h)
		result := lookUpResult{
			hostname: h,
		}
		if err != nil {
			result.err = err
		} else {
			if len(resolvedMa) > 0 {
				result.ipAddr = resolvedMa[0]
			} else {
				result.err = fmt.Errorf("address for hostname %s not found", h)
			}
		}
		results <- result
	}
}

func (d *DnsResolver) refreshDNS(ctx context.Context, hostnamesMA []multiaddr.Multiaddr) {

	hostnamesCount := len(hostnamesMA)
	hostnamesChan := make(chan multiaddr.Multiaddr, hostnamesCount)     // job queue
	resultsChan := make(chan lookUpResult, hostnamesCount) // results queue

	wg := sync.WaitGroup{}

	// spin up workers to do the dns lookup (producers)
	for w := 0; w < worker_count; w++ {
		wg.Add(1)
		go d.worker(ctx, hostnamesChan, resultsChan, &wg)
	}

	updateMap := func(results <-chan lookUpResult, wg *sync.WaitGroup) {
		defer wg.Done()
		for i := 0; i < hostnamesCount; i++ {
			result := <-results
			if result.err != nil {
				d.log.Err(result.err).Str("hostname", result.hostname.String()).Msg("dns hostname lookup failed")
				continue
			}
			d.dnsMap.update(result.hostname, result.ipAddr)
		}
	}

	// spin up the function to update the dns cache (consumer)
	wg.Add(1)
	go updateMap(resultsChan, &wg)

	// push hostnames to hostname channel
	for _, h := range hostnamesMA {
		hostnamesChan <- h
	}

	close(hostnamesChan)
	wg.Wait()
}

func (d *DnsResolver) RefreshDNSPeriodically(ctx context.Context, hostnamesMA []multiaddr.Multiaddr, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.refreshDNS(ctx, hostnamesMA)
			}
		}
	}()
}
