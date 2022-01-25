package stdmap_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/network"
)

func TestDNSCache(t *testing.T) {
	total := 700             // total entries to store (i.e., 700 ip and 700 txt domains)
	sizeLimit := uint32(500) // cache size limit (i.e., 500 ip and 500 txt domains)

	ipFixtures := network.IpLookupFixture(total)
	txtFixtures := network.TxtLookupFixture(total)

	cache := stdmap.NewDNSCache(sizeLimit, unittest.Logger())

	// cache must be initially empty
	ips, txts := cache.Size()
	require.Equal(t, uint(0), ips)
	require.Equal(t, uint(0), txts)

	// adding 700 txt and 700 ip domains to cache
	testAddToCache(t, cache, ipFixtures, txtFixtures)

	// cache must be full up to its limit
	ips, txts = cache.Size()
	require.Equal(t, uint(sizeLimit), ips)
	require.Equal(t, uint(sizeLimit), txts)

	// only 500 txt and 500 ip domains must be retrievable
	testRetrievalMatchCount(t, cache, ipFixtures, txtFixtures, int(sizeLimit))
}

// testAddToCache is a test helper that adds ip and txt domains concurrently to the cache and evaluates
// each domain is retrievable right after it has been added.
func testAddToCache(t *testing.T,
	cache *stdmap.DNSCache,
	ipTestCases map[string]*network.IpLookupTestCase,
	txtTestCases map[string]*network.TxtLookupTestCase) {

	wg := sync.WaitGroup{}
	wg.Add(len(ipTestCases) + len(txtTestCases))

	for _, fixture := range ipTestCases {
		go func(fixture *network.IpLookupTestCase) {
			require.True(t, cache.PutIpDomain(fixture.Domain, fixture.TimeStamp, fixture.Result))
			addresses, timestamp, ok := cache.GetIpDomain(fixture.Domain)
			require.True(t, ok)

			require.Equal(t, fixture.TimeStamp, timestamp)
			require.Equal(t, fixture.Result, addresses)

			wg.Done()
		}(fixture)
	}

	for _, fixture := range txtTestCases {
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
}

// testMatchCount is a test helper that checks specified number of txt and ip domains are retrievable from the cache.
// The `count` parameter specifies number of expected matches from txt and ip domains, separately.
func testRetrievalMatchCount(t *testing.T,
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
