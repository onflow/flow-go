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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)



var host1 = "host1.com"
var host2 = "host2.com"
var badhost = "bad.com"
var host1ma = multiaddr.StringCast(fmt.Sprintf("/dns4/%s", host1))
var host2ma = multiaddr.StringCast(fmt.Sprintf("/dns4/%s", host2))
var badhostma = multiaddr.StringCast(fmt.Sprintf("/dns4/%s", badhost))

var ip4a = "192.0.2.1"
var ip4aAddr = net.IPAddr{IP: net.ParseIP(ip4a)}
var ip4aMa = multiaddr.StringCast(fmt.Sprintf("/ip4/%s", ip4a))

var ip4b = "192.0.2.2"
var ip4bAddr = net.IPAddr{IP: net.ParseIP(ip4b)}
var ip4bMa = multiaddr.StringCast(fmt.Sprintf("/ip4/%s", ip4b))

var ip4c = "192.0.2.3"
var ip4cAddr = net.IPAddr{IP: net.ParseIP(ip4c)}
var ip4cMa = multiaddr.StringCast(fmt.Sprintf("/ip4/%s", ip4c))

var badip = "192.0.2.19"
var badipMa = multiaddr.StringCast(fmt.Sprintf("/ip4/%s", badip))

var ip6aAddr = net.IPAddr{IP: net.ParseIP("2001:db8::a3")}
var ip6bAddr = net.IPAddr{IP: net.ParseIP("2001:db8::a4")}

func makeResolver() *madns.Resolver {
	mock := &madns.MockBackend{
		IP: map[string][]net.IPAddr{
			host1: {ip6aAddr, ip4aAddr, ip4bAddr, ip6bAddr}, // sandwich ipv4 addresses between ipv6 addresses
			host2: {ip4cAddr},
		},
	}
	resolver := madns.Resolver{Backend: mock}
	return &resolver
}

type dNSResolverTestSuite struct {
	suite.Suite
	backend madns.MockBackend
	logger  zerolog.Logger
	ctx     context.Context
}

func TestDNSResolverTestSuite(t *testing.T) {
	suite.Run(t, new(dNSResolverTestSuite))
}

func (d *dNSResolverTestSuite) SetupTest() {
	d.logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	d.ctx = context.Background()
}

func (d *dNSResolverTestSuite) TestDNSRefresh() {
	resolver := makeResolver()
	dnsResolver := NewDnsResolver(resolver, DefaultDnsMap, d.logger)
	dnsResolver.refreshDNS(d.ctx, []multiaddr.Multiaddr{host1ma, host2ma})
	d.assertFwdLookup([]multiaddr.Multiaddr{host1ma, host2ma, badhostma}, []multiaddr.Multiaddr{ip4aMa, ip4cMa, nil})
	d.assertReverseLookup([]multiaddr.Multiaddr{ip4aMa, ip4cMa, badipMa}, []multiaddr.Multiaddr{host1ma, host2ma, nil})
}

func (d *dNSResolverTestSuite) assertFwdLookup(hostnames []multiaddr.Multiaddr, ips []multiaddr.Multiaddr) {
	for i, h := range hostnames {
		actualIP := DefaultDnsMap.LookUp(h)
		expectedIP := ips[i]
		require.Equal(d.T(), expectedIP, actualIP)
	}
}

func (d *dNSResolverTestSuite) assertReverseLookup(ips []multiaddr.Multiaddr, hostnames []multiaddr.Multiaddr) {
	for i, ip := range ips {
		actualHostname := DefaultDnsMap.ReverseLookUp(ip)
		expectedHostname := hostnames[i]
		fmt.Println(actualHostname)
		require.Equal(d.T(), expectedHostname, actualHostname)
	}
}
