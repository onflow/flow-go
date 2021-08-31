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

const happyPath = true

// TestResolver_HappyPath evaluates once the request for a domain gets cached, the subsequent requests are going through the cache
// instead of going through the underlying basic resolver, and hence through the network.
func TestResolver_HappyPath(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	resolver, err := dns.NewResolver(metrics.NewNoopCollector(), dns.WithBasicResolver(&basicResolver))
	require.NoError(t, err)

	size := 10 // 10 text and 10 ip domains.
	times := 5 // each domain is queried 5 times.
	txtTestCases := txtLookupFixture(size)
	ipTestCases := ipLookupFixture(size)

	// each domain is resolved only once through the underlying resolver, and then is cached for subsequent times.
	resolverWG := mockBasicResolverForDomains(t, &basicResolver, ipTestCases, txtTestCases, happyPath, 1)
	queryWG := queryResolver(t, times, resolver, txtTestCases, ipTestCases, happyPath)

	unittest.RequireReturnsBefore(t, resolverWG.Wait, 1*time.Second, "could not resolve all expected domains")
	unittest.RequireReturnsBefore(t, queryWG.Wait, 1*time.Second, "could not perform all queries on time")
}

// TestResolver_CacheExpiry evaluates that cached dns entries get expired after their time-to-live is passed.
func TestResolver_CacheExpiry(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	resolver, err := dns.NewResolver(
		metrics.NewNoopCollector(),
		dns.WithBasicResolver(&basicResolver),
		dns.WithTTL(1*time.Second)) // cache timeout set to 1 seconds for this test

	require.NoError(t, err)

	size := 2  // we have 10 txt and 10 ip lookup test cases
	times := 5 // each domain is queried for resolution 5 times
	txtTestCases := txtLookupFixture(size)
	ipTestCase := ipLookupFixture(size)

	resolverWG := mockBasicResolverForDomains(t, &basicResolver, ipTestCase, txtTestCases, happyPath, 2)

	queryWG := queryResolver(t, times, resolver, txtTestCases, ipTestCase, happyPath)
	unittest.RequireReturnsBefore(t, queryWG.Wait, 1*time.Second, "could not perform all queries on time")

	time.Sleep(2 * time.Second) // waits enough for cache to get invalidated

	queryWG = queryResolver(t, times, resolver, txtTestCases, ipTestCase, happyPath)
	unittest.RequireReturnsBefore(t, resolverWG.Wait, 1*time.Second, "could not resolve all expected domains")
	unittest.RequireReturnsBefore(t, queryWG.Wait, 1*time.Second, "could not perform all queries on time")
}

// TestResolver_Error evaluates that when the underlying resolver returns an error, the resolver itself does not cache the result.
func TestResolver_Error(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	resolver, err := dns.NewResolver(metrics.NewNoopCollector(), dns.WithBasicResolver(&basicResolver))
	require.NoError(t, err)

	// one test case for txt and one for ip
	times := 5 // each test case tried 5 times
	txtTestCases := txtLookupFixture(1)
	ipTestCase := ipLookupFixture(1)

	// mocks underlying basic resolver invoked 5 times per domain and returns an error each time.
	// this evaluates that upon returning an error, the result is not cached, so the next invocation again goes
	// through the resolver.
	resolverWG := mockBasicResolverForDomains(t, &basicResolver, ipTestCase, txtTestCases, !happyPath, times)
	queryWG := queryResolver(t, times, resolver, txtTestCases, ipTestCase, !happyPath)

	unittest.RequireReturnsBefore(t, resolverWG.Wait, 1*time.Second, "could not resolve all expected domains")
	unittest.RequireReturnsBefore(t, queryWG.Wait, 1*time.Second, "could not perform all queries on time")
}

type ipLookupTestCase struct {
	domain string
	result []net.IPAddr
}

type txtLookupTestCase struct {
	domain string
	result []string
}

// queryResolver concurrently requests each test case for the specified number of times. The returned wait group will be released when
// all queries have been resolved.
func queryResolver(t *testing.T,
	times int,
	resolver *madns.Resolver,
	txtTestCases map[string]*txtLookupTestCase,
	ipTestCases map[string]*ipLookupTestCase,
	happyPath bool) *sync.WaitGroup {

	ctx := context.Background()
	wg := &sync.WaitGroup{}
	wg.Add(times * (len(txtTestCases) + len(ipTestCases)))

	for _, txttc := range txtTestCases {
		cacheAndQuery(t, func(domain string) (interface{}, error) {
			return resolver.LookupTXT(ctx, domain)
		}, txttc.domain, txttc.result, times, wg, happyPath)
	}

	for _, iptc := range ipTestCases {
		cacheAndQuery(t, func(domain string) (interface{}, error) {
			return resolver.LookupIPAddr(ctx, domain)
		}, iptc.domain, iptc.result, times, wg, happyPath)
	}

	return wg
}

// cacheAndQuery makes a dns query for each of domains first so that the result gets cache, and then it performs
// concurrent queries for each test case for the specified number of times. The wait group is released when all
// queries resolved.
func cacheAndQuery(t *testing.T,
	resolver func(domain string) (interface{}, error),
	domain string,
	result interface{},
	times int,
	wg *sync.WaitGroup,
	happyPath bool) {

	firstCallDone := make(chan interface{})

	for i := 0; i < times; i++ {
		go func(index int) {
			if index != 0 {
				// other invocations (except first one) of each test
				// wait for the first time to get through and
				// cached and then go concurrently.
				<-firstCallDone
			}

			addrs, err := resolver(domain)

			if happyPath {
				require.NoError(t, err)
				require.ElementsMatch(t, addrs, result)
			} else {
				require.Error(t, err)
			}

			if index == 0 {
				close(firstCallDone) // now lets other invocations go
			}

			wg.Done()

		}(i)
	}
}

// mockBasicResolverForDomains mocks the resolver for the ip and txt lookup test cases, it makes sure that no domain is requested more than
// the number of times specified.
// Returned wait group is released when resolver is queried for `times * (len(ipLookupTestCases) + len(txtLookupTestCases))` times.
func mockBasicResolverForDomains(t *testing.T,
	resolver *mocknetwork.BasicResolver,
	ipLookupTestCases map[string]*ipLookupTestCase,
	txtLookupTestCases map[string]*txtLookupTestCase,
	happyPath bool,
	times int) *sync.WaitGroup {

	// keeping track of requested domains
	ipRequested := make(map[string]int)
	txtRequested := make(map[string]int)

	wg := &sync.WaitGroup{}
	wg.Add(times * (len(ipLookupTestCases) + len(txtLookupTestCases)))

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
		_, ok = ipLookupTestCases[domain]
		require.True(t, ok)

		// requested domain should be only requested once through underlying resolver
		count, ok := ipRequested[domain]
		if !ok {
			count = 0
		}
		count++
		require.LessOrEqual(t, count, times)
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
		_, ok = txtLookupTestCases[domain]
		require.True(t, ok)

		// requested domain should be only requested once through underlying resolver
		count, ok := txtRequested[domain]
		if !ok {
			count = 0
		}
		count++
		require.LessOrEqual(t, count, times)
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
