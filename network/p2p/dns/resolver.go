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

	addr, err := r.res.LookupIPAddr(ctx, domain)

	r.collector.DNSLookupDuration(time.Since(started))

	return addr, err
}

func (r *Resolver) LookupTXT(ctx context.Context, txt string) ([]string, error) {
	started := time.Now()

	addr, err := r.res.LookupTXT(ctx, txt)

	r.collector.DNSLookupDuration(time.Since(started))

	return addr, err
}
