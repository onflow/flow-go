package stdmap_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/network"
)

func TestDNSCache(t *testing.T) {
	total := 501
	sizeLimit := uint32(500)
	ipFixtures := network.IpLookupFixture(total)
	txtFixtures := network.TxtLookupFixture(total)
	cache := stdmap.NewDNSCache(sizeLimit, unittest.Logger())

	// cache must be initially empty
	ips, txts := cache.Size()
	require.Equal(t, uint(0), ips)
	require.Equal(t, uint(0), txts)

	wg := sync.WaitGroup{}
	wg.Add(2 * total)

	for _, fixture := range ipFixtures {
		go func(fixture *network.IpLookupTestCase) {
			require.True(t, cache.PutIpDomain(fixture.Domain, fixture.TimeStamp, fixture.Result))
			addresses, timestamp, ok := cache.GetIpDomain(fixture.Domain)
			require.True(t, ok)

			require.Equal(t, fixture.TimeStamp, timestamp)
			require.Equal(t, fixture.Result, addresses)

			wg.Done()
		}(fixture)
	}

	for _, fixture := range txtFixtures {
		go func(fixture *network.TxtLookupTestCase) {
			require.True(t, cache.PutTxtDomain(fixture.Domain, fixture.TimeStamp, fixture.Result))

			addresses, timestamp, ok := cache.GetTxtDomain(fixture.Domain)
			require.True(t, ok)

			require.Equal(t, fixture.TimeStamp, timestamp)
			require.Equal(t, fixture.Result, addresses)
			wg.Done()
		}(fixture)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "could not write all test cases on time")

	// cache must be full up to its limit
	ips, txts = cache.Size()
	fmt.Println("ips", ips, "txts", txts)
	require.Equal(t, uint(sizeLimit), ips)
	require.Equal(t, uint(sizeLimit), txts)

	// 500 txt and 500 ip domains must be retrievable
	testMatchCount(t, cache, ipFixtures, txtFixtures, int(sizeLimit))
}

// testMatchCount is a test helper that checks specified number of txt and ip domains are retrievable from the cache.
// The `count` parameter specifies number of expected matches from txt and ip domains, separately.
func testMatchCount(t *testing.T,
	cache *stdmap.DNSCache,
	ipTestCases map[string]*network.IpLookupTestCase,
	txtTestCases map[string]*network.TxtLookupTestCase,
	count int) {

	// checking ip domains
	actualCount := 0
	for _, tc := range ipTestCases {
		addresses, timestamp, ok := cache.GetIpDomain(tc.Domain)
		if !ok {
			continue
		}
		require.True(t, ok)

		require.Equal(t, tc.TimeStamp, timestamp)
		require.Equal(t, tc.Result, addresses)
		actualCount++
	}
	require.Equal(t, count, actualCount)

	// checking txt domains
	actualCount = 0
	for _, tc := range txtTestCases {
		addresses, timestamp, ok := cache.GetTxtDomain(tc.Domain)
		if !ok {
			continue
		}
		require.True(t, ok)

		require.Equal(t, tc.TimeStamp, timestamp)
		require.Equal(t, tc.Result, addresses)
		actualCount++
	}
	require.Equal(t, count, actualCount)

}
