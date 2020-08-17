package dns

import (
	"context"
	"fmt"
	"sync"

	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
)

type hostnameMa multiaddr.Multiaddr
type ipMa multiaddr.Multiaddr

type lookUpResult struct {
	ipAddr ipMa
	err    error
}

type DnsResolver struct {
	sync.Mutex
	hostnameIPMap map[hostnameMa]ipMa
	ipHostnameMap map[ipMa]hostnameMa
	resolver madns.Resolver
}

func NewDnsResolver(resolver madns.Resolver) DnsResolver {
	return DnsResolver{
		hostnameIPMap: make(map[hostnameMa]ipMa),
		ipHostnameMap: make(map[ipMa]hostnameMa),
		resolver: resolver,
	}
}

func (d *DnsResolver) worker(ctx context.Context, hostnames  <- chan hostnameMa, results chan <- lookUpResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for h := range hostnames {
		ma := multiaddr.StringCast(fmt.Sprintf("/dns4/%s", h))
		resolvedMa, err := d.resolver.Resolve(ctx, ma)
		result := lookUpResult{}
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


