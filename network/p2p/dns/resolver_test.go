package dns_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	madns "github.com/multiformats/go-multiaddr-dns"
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

	size := 10 // we have 10 txt and 10 ip lookup test cases
	times := 5 // each domain is queried for resolution 5 times
	txtTestCases := txtLookupFixture(size)
	ipTestCases := ipLookupFixture(size)

	// going through the cache, each domain should only being resolved once over the underlying resolver.
	wg := mockBasicResolverForDomains(t, &basicResolver, ipTestCases, txtTestCases, true, 1)

	// each test case is repeated 5 times, since resolver has been mocked only once per test case
	// it ensures that the rest 4 calls are made through the cache and not the resolver.
	queryResolver(t, times, resolver, txtTestCases, ipTestCases)

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not resolve all addresses")
}

// TestResolver_HappyPath evaluates the happy path behavior of dns resolver against concurrent invocations. Each unique domain
// invocation should go through the underlying basic resolver only once, and the result should get cached for subsequent invocations.
// the test evaluates the correctness of invocations as well as resolution through cache on repetition.
func TestResolver_CacheExpiry(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	resolver, err := dns.NewResolver(
		metrics.NewNoopCollector(),
		dns.WithBasicResolver(&basicResolver),
		dns.WithTTL(1*time.Second)) // cache timeout set to 1 seconds for this test

	require.NoError(t, err)

	size := 2  // we have 10 txt and 10 ip lookup test cases
	times := 5 // each domain is queried for resolution 10 times
	txtTestCases := txtLookupFixture(size)
	ipTestCase := ipLookupFixture(size)
	wg := mockBasicResolverForDomains(t, &basicResolver, ipTestCase, txtTestCases, true, 2)

	queryResolver(t, times, resolver, txtTestCases, ipTestCase)

	time.Sleep(2 * time.Second) // waits enough for cache to get invalidated

	queryResolver(t, times, resolver, txtTestCases, ipTestCase)
	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not resolve all addresses")
}

//
//// TestResolver_Error evaluates that when the underlying resolver returns an error, the resolver itself does not cache the result.
//func TestResolver_Error(t *testing.T) {
//	basicResolver := mocknetwork.BasicResolver{}
//	resolver, err := dns.NewResolver(metrics.NewNoopCollector(), dns.WithBasicResolver(&basicResolver))
//	require.NoError(t, err)
//
//	// one test case for txt and one for ip
//	times := 5
//	txtTestCases := txtLookupFixture(1)
//	ipTestCase := ipLookupFixture(1)
//	wg := &sync.WaitGroup{}
//	wg.Add(2 * times) // 5 times for ip and 5 times for txt
//
//	// mocks underlying basic resolver invoked 5 times per domain and returns an error each time.
//	// this evaluates that upon returning an error, the result is not cached, so the next invocation again goes
//	// through the resolver.
//	mockBasicResolverForDomainsWithError(&basicResolver, ipTestCase, txtTestCases, times)
//
//	ctx := context.Background()
//	// each test case is repeated 5 times, and since underlying basic resolver is mocked to return error, it ensures
//	// that all calls go through the resolver without ever getting cached.
//	for i := 0; i < times; i++ {
//		go func() {
//			addrs, err := resolver.LookupTXT(ctx, txtTestCases[0].domain)
//			require.Error(t, err)
//			require.Nil(t, addrs)
//			wg.Done()
//		}()
//
//		go func() {
//			addrs, err := resolver.LookupIPAddr(ctx, ipTestCase[0].domain)
//			require.Error(t, err)
//			require.Nil(t, addrs)
//			wg.Done()
//
//		}()
//	}
//
//	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not resolve all addresses")
//	basicResolver.AssertExpectations(t) // asserts that basic resolver is invoked exactly once per domain
//}

type ipLookupTestCase struct {
	domain string
	result []net.IPAddr
}

type txtLookupTestCase struct {
	domain string
	result []string
}

func queryResolver(t *testing.T, times int, resolver *madns.Resolver, txtTestCases map[string]*txtLookupTestCase,
	ipTestCases map[string]*ipLookupTestCase) {
	ctx := context.Background()
	for i := 0; i < times; i++ {
		for _, txttc := range txtTestCases {
			go func(tc *txtLookupTestCase) {
				addrs, err := resolver.LookupTXT(ctx, tc.domain)
				require.NoError(t, err)

				require.ElementsMatch(t, addrs, tc.result)

			}(txttc)
		}

		for _, iptc := range ipTestCases {
			go func(tc *ipLookupTestCase) {
				addrs, err := resolver.LookupIPAddr(ctx, tc.domain)
				require.NoError(t, err)

				require.ElementsMatch(t, addrs, tc.result)
			}(iptc)
		}
	}
}

// mockBasicResolverForDomains mocks the resolver for the ip and txt lookup test cases.
func mockBasicResolverForDomains(t *testing.T,
	resolver *mocknetwork.BasicResolver,
	ipLookupTestCases map[string]*ipLookupTestCase,
	txtLookupTestCases map[string]*txtLookupTestCase,
	happyPath bool,
	times int) *sync.WaitGroup {

	// keeping track of number of times each domain is requested.
	ipRequested := make(map[string]int)
	txtRequested := make(map[string]int)

	wg := &sync.WaitGroup{}
	fmt.Println("wg: ", times*(len(txtLookupTestCases)+len(ipLookupTestCases)))
	wg.Add(times * (len(txtLookupTestCases) + len(ipLookupTestCases))) // each test case requested `times` times!

	mu := sync.Mutex{}
	resolver.On("LookupIPAddr", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		// method should be called on expected parameters
		_, ok := args[0].(context.Context)
		require.True(t, ok)

		domain, ok := args[1].(string)
		require.True(t, ok)

		// requested domain should be expected.
		// number of requests should not exceed the `times`.
		_, ok = ipLookupTestCases[domain]
		require.True(t, ok)

		count, ok := ipRequested[domain]
		if !ok {
			count = 0
		}
		require.Less(t, count, times)

		count++
		ipRequested[domain] = count

		wg.Done()

	}).Return(
		func(ctx context.Context, domain string) []net.IPAddr {
			if !happyPath {
				return nil
			}
			return ipLookupTestCases[domain].result
		},
		func(ctx context.Context, domain string) error {
			if !happyPath {
				return fmt.Errorf("error")
			}
			return nil
		})

	resolver.On("LookupTXT", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		// method should be called on expected parameters
		_, ok := args[0].(context.Context)
		require.True(t, ok)

		domain, ok := args[1].(string)
		require.True(t, ok)

		// requested domain should be expected.
		// number of requests should not exceed the `times`.
		_, ok = txtLookupTestCases[domain]
		require.True(t, ok)

		count, ok := txtRequested[domain]
		if !ok {
			count = 0
		}
		require.Less(t, count, times)

		count++
		txtRequested[domain] = count

		wg.Done()

	}).Return(
		func(ctx context.Context, domain string) []string {
			if !happyPath {
				return nil
			}
			return txtLookupTestCases[domain].result
		},
		func(ctx context.Context, domain string) error {
			if !happyPath {
				return fmt.Errorf("error")
			}
			return nil
		})

	return wg
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
