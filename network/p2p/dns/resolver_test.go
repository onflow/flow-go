package dns_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestResolver_HappyPath evaluates the happy path behavior of dns resolver against concurrent invocations. Each unique domain
// invocation should go through the underlying basic resolver only once, and the result should get cached for subsequent invocations.
// the test evaluates the correctness of invocations as well as resolution through cache on repetition.
func TestResolver_HappyPath(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	resolver, err := dns.NewResolver(metrics.NewNoopCollector(), dns.WithBasicResolver(&basicResolver))
	require.NoError(t, err)

	size := 2         // we have 10 txt and 10 ip lookup test cases
	times := 5 * size // each domain is queried for resolution 5 times
	txtTestCases := txtLookupFixture(size)
	ipTestCase := ipLookupFixture(size)

	wg := &sync.WaitGroup{}
	wg.Add(2 * times) // ip + txt
	mockBasicResolverForDomains(&basicResolver, ipTestCase, txtTestCases, times)

	ctx := context.Background()
	// each test case is repeated 5 times, since resolver has been mocked only once per test case
	// it ensures that the rest 4 calls are made through the cache and not the resolver.
	for i := 0; i < times; i++ {
		go func(tc *txtLookupTestCase) {
			addrs, err := resolver.LookupTXT(ctx, tc.domain)
			require.NoError(t, err)

			require.ElementsMatch(t, addrs, tc.result)

			wg.Done()
		}(txtTestCases[i%size])

		go func(tc *ipLookupTestCase) {
			addrs, err := resolver.LookupIPAddr(ctx, tc.domain)
			require.NoError(t, err)

			require.ElementsMatch(t, addrs, tc.result)

			wg.Done()
		}(ipTestCase[i%size])
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not resolve all addresses")

	basicResolver.AssertExpectations(t) // asserts that basic resolver is invoked exactly once per domain
}

// TestResolver_HappyPath evaluates the happy path behavior of dns resolver against concurrent invocations. Each unique domain
// invocation should go through the underlying basic resolver only once, and the result should get cached for subsequent invocations.
// the test evaluates the correctness of invocations as well as resolution through cache on repetition.
func TestResolver_CacheExpiry(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	resolver, err := dns.NewResolver(
		metrics.NewNoopCollector(),
		dns.WithBasicResolver(&basicResolver),
		dns.WithTTL(2*time.Second))
	require.NoError(t, err)

	size := 10        // we have 10 txt and 10 ip lookup test cases
	times := 5 * size // each domain is queried for resolution 10 times
	txtTestCases := txtLookupFixture(size)
	ipTestCase := ipLookupFixture(size)
	wg := &sync.WaitGroup{}
	wg.Add(times) // 10 ip + 10 txt
	mockBasicResolverForDomains(&basicResolver, ipTestCase, txtTestCases, 2)

	ctx := context.Background()
	// each test case is repeated 5 times, since resolver has been mocked only once per test case
	// it ensures that the rest 4 calls are made through the cache and not the resolver.
	for i := 0; i < times; i++ {
		go func(tc *txtLookupTestCase) {
			addrs, err := resolver.LookupTXT(ctx, tc.domain)
			require.NoError(t, err)

			require.ElementsMatch(t, addrs, tc.result)

			wg.Done()
		}(txtTestCases[i%size])

		go func(tc *ipLookupTestCase) {
			addrs, err := resolver.LookupIPAddr(ctx, tc.domain)
			require.NoError(t, err)

			require.ElementsMatch(t, addrs, tc.result)

			wg.Done()
		}(ipTestCase[i%size])
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not resolve all addresses")

	time.Sleep(2 * time.Second) // waits enough for cache to get invalidated
	wg.Add(2 * times)           // 10 ip + 10 txt

	for i := 0; i < times; i++ {
		go func(tc *txtLookupTestCase) {
			addrs, err := resolver.LookupTXT(ctx, tc.domain)
			require.NoError(t, err)

			require.ElementsMatch(t, addrs, tc.result)

			wg.Done()
		}(txtTestCases[i%size])

		go func(tc *ipLookupTestCase) {
			addrs, err := resolver.LookupIPAddr(ctx, tc.domain)
			require.NoError(t, err)

			require.ElementsMatch(t, addrs, tc.result)

			wg.Done()
		}(ipTestCase[i%size])
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 1000*time.Second, "could not resolve all addresses")

	basicResolver.AssertExpectations(t) // asserts that basic resolver is invoked exactly once per domain
}

// TestResolver_Error evaluates that when the underlying resolver returns an error, the resolver itself does not cache the result.
func TestResolver_Error(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	resolver, err := dns.NewResolver(metrics.NewNoopCollector(), dns.WithBasicResolver(&basicResolver))
	require.NoError(t, err)

	// one test case for txt and one for ip
	times := 5
	txtTestCases := txtLookupFixture(1)
	ipTestCase := ipLookupFixture(1)
	wg := &sync.WaitGroup{}
	wg.Add(2 * times) // 5 times for ip and 5 times for txt

	// mocks underlying basic resolver invoked 5 times per domain and returns an error each time.
	// this evaluates that upon returning an error, the result is not cached, so the next invocation again goes
	// through the resolver.
	mockBasicResolverForDomainsWithError(&basicResolver, ipTestCase, txtTestCases, times)

	ctx := context.Background()
	// each test case is repeated 5 times, and since underlying basic resolver is mocked to return error, it ensures
	// that all calls go through the resolver without ever getting cached.
	for i := 0; i < times; i++ {
		go func() {
			addrs, err := resolver.LookupTXT(ctx, txtTestCases[0].domain)
			require.Error(t, err)
			require.Nil(t, addrs)
			wg.Done()
		}()

		go func() {
			addrs, err := resolver.LookupIPAddr(ctx, ipTestCase[0].domain)
			require.Error(t, err)
			require.Nil(t, addrs)
			wg.Done()

		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not resolve all addresses")
	basicResolver.AssertExpectations(t) // asserts that basic resolver is invoked exactly once per domain
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
func mockBasicResolverForDomains(resolver *mocknetwork.BasicResolver,
	ipLookupTestCases []*ipLookupTestCase,
	txtLookupTestCases []*txtLookupTestCase,
	times int) {

	for _, tc := range ipLookupTestCases {
		resolver.On("LookupIPAddr", mock.Anything, tc.domain).Return(tc.result, nil).Times(times)
	}

	for _, tc := range txtLookupTestCases {
		resolver.On("LookupTXT", mock.Anything, tc.domain).Return(tc.result, nil).Run(func(args mock.Arguments) {
		}).Times(times)
	}
}

// mockBasicResolverForDomains mocks the resolver returning error for the ip and txt lookup test cases.
func mockBasicResolverForDomainsWithError(resolver *mocknetwork.BasicResolver,
	ipLookupTestCases []*ipLookupTestCase,
	txtLookupTestCases []*txtLookupTestCase,
	times int) {

	for _, tc := range ipLookupTestCases {
		resolver.On("LookupIPAddr", mock.Anything, tc.domain).
			Return(nil, fmt.Errorf("error"))
	}

	for _, tc := range txtLookupTestCases {
		resolver.On("LookupTXT", mock.Anything, tc.domain).
			Return(nil, fmt.Errorf("error"))
	}
}

func ipLookupFixture(count int) map[string]*ipLookupTestCase {
	tt := make(map[string]*ipLookupTestCase)
	for i := 0; i < count; i++ {
		ipTestCase := &ipLookupTestCase{
			domain: fmt.Sprintf("example%d.com", i),
			result: []net.IPAddr{ // resolves each domain to 4 addresses.
				netIPAddrFixture(),
				netIPAddrFixture(),
				netIPAddrFixture(),
				netIPAddrFixture(),
			},
		}

		tt[ipTestCase.domain] = ipTestCase
	}

	return tt
}

func txtLookupFixture(count int) map[string]*txtLookupTestCase {
	tt := make(map[string]*txtLookupTestCase)

	for i := 0; i < count; i++ {
		ttTestCase := &txtLookupTestCase{
			domain: fmt.Sprintf("_dnsaddr.example%d.com", i),
			result: []string{ // resolves each domain to 4 addresses.
				txtIPFixture(),
				txtIPFixture(),
				txtIPFixture(),
				txtIPFixture(),
			},
		}

		tt[ttTestCase.domain] = ttTestCase
	}

	return tt
}

func netIPAddrFixture() net.IPAddr {
	token := make([]byte, 4)
	rand.Read(token)

	ip := net.IPAddr{
		IP:   net.IPv4(token[0], token[1], token[2], token[3]),
		Zone: "flow0",
	}

	return ip
}

func txtIPFixture() string {
	token := make([]byte, 4)
	rand.Read(token)
	return "dnsaddr=" + net.IPv4(token[0], token[1], token[2], token[3]).String()
}
