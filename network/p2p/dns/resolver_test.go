package dns

import (
	"fmt"
	"math/rand"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
)

func TestResolver(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	_, err := NewResolver(metrics.NewNoopCollector(), WithBasicResolver(&basicResolver))
	require.NoError(t, err)

}

type ipLookupTestCase struct {
	domain string
	result []net.IPAddr
}

func mockBasicResolverForDomains(resolver *Resolver, testCases []*ipLookupTestCase) {

}

func ipLookupFixture(count int) []*ipLookupTestCase {
	tt := make([]*ipLookupTestCase, count)
	for i := 0; i < count; i++ {
		tt = append(tt, &ipLookupTestCase{
			domain: fmt.Sprintf("example%d.com", i),
			result: netIPAddrFixture(),
		})
	}

	return tt
}

func netIPAddrFixture() []net.IPAddr {
	token := make([]byte, 4)
	rand.Read(token)

	ip := net.IPAddr{
		IP:   net.IPv4(token[0], token[1], token[2], token[3]),
		Zone: "flow",
	}

	return []net.IPAddr{ip}
}
