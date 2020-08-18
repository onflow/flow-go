package dns

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/suite"
)

var host1 = "host1.com"
var host2 = "host2.com"
var host1ma = multiaddr.StringCast(fmt.Sprintf("/dns4/%s", host1))
var host2ma = multiaddr.StringCast(fmt.Sprintf("/dns4/%s", host2))

var ip4a = net.IPAddr{IP: net.ParseIP("192.0.2.1")}
var ip4b = net.IPAddr{IP: net.ParseIP("192.0.2.2")}
var ip4c = net.IPAddr{IP: net.ParseIP("189.10.10.1")}
var ip6a = net.IPAddr{IP: net.ParseIP("2001:db8::a3")}
var ip6b = net.IPAddr{IP: net.ParseIP("2001:db8::a4")}

func makeResolver() madns.Resolver {
	mock := &madns.MockBackend{
		IP: map[string][]net.IPAddr{
			host1: []net.IPAddr{ip4a, ip4b, ip6a, ip6b},
			host2: []net.IPAddr{ip4c},
		},
	}
	resolver := madns.Resolver{Backend: mock}
	return resolver
}

type dNSResolverTestSuite struct {
	suite.Suite
	backend madns.MockBackend
	logger zerolog.Logger
	ctx context.Context
}

func TestDNSResolverTestSuite(t *testing.T) {
	suite.Run(t, new(dNSResolverTestSuite))
}

func (d *dNSResolverTestSuite) SetupTest() {
	d.logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	d.ctx = context.Background()
}


func(d *dNSResolverTestSuite) TestDNSRefresh() {
	resolver := makeResolver()
	dnsResolver := NewDnsResolver(resolver, d.logger)
	dnsResolver.refreshDNS(d.ctx, []hostnameMa{host1ma, host2ma})
}







