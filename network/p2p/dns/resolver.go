package dns

import (
	"context"
	"net"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/metrics"
)

type Resolver struct {
	res       madns.BasicResolver
	collector metrics.NetworkCollector
	logger    zerolog.Logger
}

func NewResolver(res madns.BasicResolver, collector metrics.NetworkCollector, logger zerolog.Logger) *Resolver {
	return &Resolver{
		res:       res,
		collector: collector,
		logger:    logger.With().Str("module", "dns-resolver").Logger(),
	}
}

func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	return r.res.LookupIPAddr(ctx, domain)
}

func (r *Resolver) LookupTXT(ctx context.Context, txt string) ([]string, error) {
	return r.res.LookupTXT(ctx, txt)
}
