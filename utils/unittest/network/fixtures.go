package testnetwork

import (
	"fmt"
	"math/rand"
	"net"
)

type IpLookupTestCase struct {
	Domain string
	Result []net.IPAddr
}

type TxtLookupTestCase struct {
	Domain string
	Result []string
}

func NetIPAddrFixture() net.IPAddr {
	token := make([]byte, 4)
	rand.Read(token)

	ip := net.IPAddr{
		IP:   net.IPv4(token[0], token[1], token[2], token[3]),
		Zone: "flow0",
	}

	return ip
}

func TxtIPFixture() string {
	token := make([]byte, 4)
	rand.Read(token)
	return "dnsaddr=" + net.IPv4(token[0], token[1], token[2], token[3]).String()
}

func IpLookupFixture(count int) map[string]*IpLookupTestCase {
	tt := make(map[string]*IpLookupTestCase)
	for i := 0; i < count; i++ {
		ipTestCase := &IpLookupTestCase{
			Domain: fmt.Sprintf("example%d.com", i),
			Result: []net.IPAddr{ // resolves each Domain to 4 addresses.
				NetIPAddrFixture(),
				NetIPAddrFixture(),
				NetIPAddrFixture(),
				NetIPAddrFixture(),
			},
		}

		tt[ipTestCase.Domain] = ipTestCase
	}

	return tt
}

func TxtLookupFixture(count int) map[string]*TxtLookupTestCase {
	tt := make(map[string]*TxtLookupTestCase)

	for i := 0; i < count; i++ {
		ttTestCase := &TxtLookupTestCase{
			Domain: fmt.Sprintf("_dnsaddr.example%d.com", i),
			Result: []string{ // resolves each Domain to 4 addresses.
				TxtIPFixture(),
				TxtIPFixture(),
				TxtIPFixture(),
				TxtIPFixture(),
			},
		}

		tt[ttTestCase.Domain] = ttTestCase
	}

	return tt
}
