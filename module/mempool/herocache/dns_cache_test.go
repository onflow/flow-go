package herocache_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/network"
)

// TestDNSCache_Concurrent checks the correctness of cache under concurrent insertions.
func TestDNSCache_Concurrent(t *testing.T) {
	total := 700             // total entries to store (i.e., 700 ip and 700 txt domains)
	sizeLimit := uint32(500) // cache size limit (i.e., 500 ip and 500 txt domains)

	ipFixtures := network.IpLookupFixture(total)
	txtFixtures := network.TxtLookupFixture(total)

	cache := herocache.NewDNSCache(sizeLimit, unittest.Logger())

	// cache must be initially empty
	ips, txts := cache.Size()
	require.Equal(t, uint(0), ips)
	require.Equal(t, uint(0), txts)

	// adding 700 txt and 700 ip domains to cache
	testConcurrentAddToCache(t, cache, ipFixtures, txtFixtures)

	// cache must be full up to its limit
	ips, txts = cache.Size()
	require.Equal(t, uint(sizeLimit), ips)
	require.Equal(t, uint(sizeLimit), txts)

	// only 500 txt and 500 ip domains must be retrievable
	testRetrievalMatchCount(t, cache, ipFixtures, txtFixtures, int(sizeLimit))
}

// TestDNSCache_Concurrent checks the correctness of cache under concurrent insertions.
func TestDNSCache_LRU(t *testing.T) {
	total := 700             // total entries to store (i.e., 700 ip and 700 txt domains)
	sizeLimit := uint32(500) // cache size limit (i.e., 500 ip and 500 txt domains)

	ipFixtures := network.IpLookupListFixture(total)
	txtFixtures := network.TxtLookupListFixture(total)

	cache := herocache.NewDNSCache(sizeLimit, unittest.Logger())

	// cache must be initially empty
	ips, txts := cache.Size()
	require.Equal(t, uint(0), ips)
	require.Equal(t, uint(0), txts)

	// adding 700 txt and 700 ip domains to cache
	for _, fixture := range ipFixtures {
		require.True(t, cache.PutIpDomain(fixture.Domain, fixture.TimeStamp, fixture.Result))
	}

	for _, fixture := range txtFixtures {
		require.True(t, cache.PutTxtDomain(fixture.Domain, fixture.TimeStamp, fixture.Result))
	}

	// cache must be full up to its limit
	ips, txts = cache.Size()
	require.Equal(t, uint(sizeLimit), ips)
	require.Equal(t, uint(sizeLimit), txts)

	// only last 500 ip domains and txt must be retained in the DNS cache
	for i := 0; i < total; i++{
		if i < total-int(sizeLimit) {
			// old dns entries must be ejected
			ip, _, ok := cache.GetIpDomain(ipFixtures[].Domain)
		}
	}
}

// testConcurrentAddToCache is a test helper that concurrently adds ip and txt domains concurrently to the cache and evaluates
// each domain is retrievable right after it has been added.
func testSequentialAddToCache(t *testing.T,
	cache *herocache.DNSCache,
	ipTestCases map[string]*network.IpLookupTestCase,
	txtTestCases map[string]*network.TxtLookupTestCase) {

	for _, fixture := range ipTestCases {
		require.True(t, cache.PutIpDomain(fixture.Domain, fixture.TimeStamp, fixture.Result))
	}

	for _, fixture := range txtTestCases {
		require.True(t, cache.PutTxtDomain(fixture.Domain, fixture.TimeStamp, fixture.Result))
	}
}

// testMatchCount is a test helper that checks specified number of txt and ip domains are retrievable from the cache.
// The `count` parameter specifies number of expected matches from txt and ip domains, separately.
func testRetrievalMatchCount(t *testing.T,
	cache *herocache.DNSCache,
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
