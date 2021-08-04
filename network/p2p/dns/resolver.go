package dns

import (
	"context"
	"net"
	"time"

	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/onflow/flow-go/module"
)

type Resolver struct {
	res       madns.BasicResolver
	collector module.NetworkMetrics
}

func NewResolver(collector module.NetworkMetrics) (*madns.Resolver, error) {
	return madns.NewResolver(madns.WithDefaultResolver(&Resolver{
		res:       madns.DefaultResolver,
		collector: collector,
	}))
}

func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	started := time.Now()
	defer r.collector.DNSLookupDuration(time.Since(started))

	return r.res.LookupIPAddr(ctx, domain)
}

func (r *Resolver) LookupTXT(ctx context.Context, txt string) ([]string, error) {
	started := time.Now()
	defer r.collector.DNSLookupDuration(time.Since(started))

	return r.res.LookupTXT(ctx, txt)
}
