package dns

import (
	"context"
	"fmt"
	"sync"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"
)

const worker_count = 3

type lookUpResult struct {
	hostname hostnameMa
	ipAddr ipMa
	err    error
}

type DnsResolver struct {
	resolver madns.Resolver
	log zerolog.Logger
}

func NewDnsResolver(resolver madns.Resolver, log zerolog.Logger) DnsResolver {
	return DnsResolver{
		resolver: resolver,
		log: log,
	}
}

func (d *DnsResolver) worker(ctx context.Context, hostnames  <- chan hostnameMa, results chan <- lookUpResult, wg *sync.WaitGroup) {
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


func (d *DnsResolver) refreshDNS(ctx context.Context, hostnames []hostnameMa) {

	hostnamesCnt := len(hostnames)
	hostnamesChan := make(chan hostnameMa, hostnamesCnt) // job queue
	resultsChan := make(chan lookUpResult, hostnamesCnt) // results queue

	wg := sync.WaitGroup{}

	// spin up workers to do the dns lookup (producers)
	for w := 0; w < worker_count; w++ {
		wg.Add(1)
		d.worker(ctx, hostnamesChan, resultsChan, &wg)
	}

	updateMap := func(results <- chan lookUpResult, wg *sync.WaitGroup) {
		defer wg.Done()
		for result := range resultsChan {
			if result.err != nil {
				d.log.Err(result.err).Str("hostname", result.hostname.String()).Msg("dns hostname lookup failed")
			}
			dnsMap.update(result.hostname, result.ipAddr)
		}
	}

	// spin up the function to update the dns cache (consumer)
	wg.Add(1)
	go updateMap(resultsChan, &wg)

	// push hostnames to job queue
	for _, h := range hostnames {
		hostnamesChan <- h
	}

	close(hostnamesChan)
	wg.Done()
}



