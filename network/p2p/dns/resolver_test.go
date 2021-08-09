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

type txtLookupTestCase struct {
	domain string
	result []string
}

// mockBasicResolverForDomains mocks the resolver for the ip and txt lookup test cases.
func mockBasicResolverForDomains(resolver *mocknetwork.BasicResolver, ipLookupTestCases []*ipLookupTestCase, txtLookupTestCases []*txtLookupTestCase) {
	for _, tc := range ipLookupTestCases {
		resolver.On("LookupIPAddr", tc.domain).Return(tc.result, nil).Once()
	}

	for _, tc := range txtLookupTestCases {
		resolver.On("LookupTXT", tc.domain).Return(tc.result, nil).Once()
	}
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
