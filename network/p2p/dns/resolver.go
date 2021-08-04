package dns

import (
	"context"
	"net"
	"time"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/metrics"
)

type Resolver struct {
	res       madns.BasicResolver
	collector metrics.NetworkCollector
	logger    zerolog.Logger
}

func NewResolver(res madns.BasicResolver, collector metrics.NetworkCollector) *Resolver {
	return &Resolver{
		res:       res,
		collector: collector,
	}
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
