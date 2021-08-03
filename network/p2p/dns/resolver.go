package dns

import (
	"context"
	"net"

	madns "github.com/multiformats/go-multiaddr-dns"
)

type Resolver struct {
	res madns.BasicResolver
}

func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	return r.res.LookupIPAddr(ctx, domain)
}

func (r *Resolver) LookupTXT(ctx context.Context, txt string) ([]string, error) {
	return r.res.LookupTXT(ctx, txt)
}
