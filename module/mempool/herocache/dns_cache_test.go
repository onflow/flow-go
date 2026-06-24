package herocache_test

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/network"
)

// TestDNSCache_HappyPath checks the correctness storing and retrieving from dns cache.
func TestDNSCache_HappyPath(t *testing.T) {
	total := 700             // total entries to store (i.e., 700 ip domains and 700 txt records)
	sizeLimit := uint32(500) // cache size limit (i.e., 500 ip domains and 500 txt records)

	ipFixtures := network.IpLookupFixture(total)
	txtFixtures := network.TxtLookupFixture(total)

	cache := herocache.NewDNSCache(sizeLimit, unittest.Logger(), metrics.NewNoopCollector(), metrics.NewNoopCollector())

	// cache must be initially empty
	ips, txts := cache.Size()
	require.Equal(t, uint(0), ips)
	require.Equal(t, uint(0), txts)

	// adding 700 txt records and 700 ip domains to cache
	testAddToCache(t, cache, ipFixtures, txtFixtures)

	// cache must be full up to its limit
	ips, txts = cache.Size()
	require.Equal(t, uint(sizeLimit), ips)
	require.Equal(t, uint(sizeLimit), txts)

	// only 500 txt records and 500 ip domains must be retrievable
	testRetrievalMatchCount(t, cache, ipFixtures, txtFixtures, int(sizeLimit))
}

// TestDNSCache_Update checks the correctness of updating dns records.
func TestDNSCache_Update(t *testing.T) {
	ipFixture := []net.IPAddr{network.NetIPAddrFixture()}
	ipDomain := "ip-domain"

	txtFixture := []string{network.TxtIPFixture()}
	txtDomain := "txt-domain"

	cache := herocache.NewDNSCache(10,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector())

	// adding records to dns cache
	require.True(t, cache.PutIpDomain(ipDomain, ipFixture, int64(0)))
	require.True(t, cache.PutTxtRecord(txtDomain, txtFixture, int64(0)))

	// locking both txt and ip records before updating
	// to later check an update unlocks them.
	locked, err := cache.LockIPDomain(ipDomain)
	require.NoError(t, err)
	require.True(t, locked)
	locked, err = cache.LockTxtRecord(txtDomain)
	require.NoError(t, err)
	require.True(t, locked)

	// updating ip record
	updatedIpFixture := []net.IPAddr{network.NetIPAddrFixture()}
	updatedIpTimestamp := int64(10)
	require.NoError(t, cache.UpdateIPDomain(ipDomain, updatedIpFixture, updatedIpTimestamp))

	// updating txt record
	updatedTxtFixture := []string{network.TxtIPFixture()}
	updatedTxtTimestamp := int64(12)
	require.NoError(t, cache.UpdateTxtRecord(txtDomain, updatedTxtFixture, updatedTxtTimestamp))

	// sanity checking of the updated records
	// updated ip record
	ipRecord, ok := cache.GetDomainIp(ipDomain)
	require.True(t, ok)
	require.Equal(t, ipRecord.Addresses, updatedIpFixture)
	require.Equal(t, ipRecord.Domain, ipDomain)
	require.False(t, ipRecord.Locked) // an update must unlock it.
	require.Equal(t, ipRecord.Timestamp, updatedIpTimestamp)

	// updated txt record
	txtRecord, ok := cache.GetTxtRecord(txtDomain)
	require.True(t, ok)
	require.Equal(t, txtRecord.Records, updatedTxtFixture)
	require.Equal(t, txtRecord.Txt, txtDomain)
	require.False(t, txtRecord.Locked) // an update must unlock it.
	require.Equal(t, txtRecord.Timestamp, updatedTxtTimestamp)
}

// TestDNSCache_Lock evaluates that locking a txt (or ip) record can be done successfully once, and
// attempts to lock and already locked record fail.
// It also evaluates that a locked record can be retrieved successfully.
func TestDNSCache_Lock(t *testing.T) {
	ipFixture := []net.IPAddr{network.NetIPAddrFixture()}
	ipDomain := "ip-domain"

	txtFixture := []string{network.TxtIPFixture()}
	txtDomain := "txt-domain"

	cache := herocache.NewDNSCache(
		10,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector())

	// adding records to dns cache
	require.True(t, cache.PutIpDomain(ipDomain, ipFixture, int64(0)))
	require.True(t, cache.PutTxtRecord(txtDomain, txtFixture, int64(0)))

	// locks ip record
	locked, err := cache.LockIPDomain(ipDomain)
	require.NoError(t, err)
	require.True(t, locked) // first locking attempt must go through
	locked, err = cache.LockIPDomain(ipDomain)
	require.NoError(t, err)
	require.False(t, locked) // other locking attempts must fail

	// locks txt record
	locked, err = cache.LockTxtRecord(txtDomain)
	require.NoError(t, err)
	require.True(t, locked) // first locking attempt must go through
	locked, err = cache.LockTxtRecord(txtDomain)
	require.NoError(t, err)
	require.False(t, locked) // other locking attempts must fail

	// locked ip record must be retrievable
	ipRecord, ok := cache.GetDomainIp(ipDomain)
	require.True(t, ok)
	require.Equal(t, ipRecord.Addresses, ipFixture)
	require.Equal(t, ipRecord.Domain, ipDomain)
	require.True(t, ipRecord.Locked)
	require.Equal(t, ipRecord.Timestamp, int64(0))

	// locked txt record must be retrievable
	txtRecord, ok := cache.GetTxtRecord(txtDomain)
	require.True(t, ok)
	require.Equal(t, txtRecord.Records, txtFixture)
	require.Equal(t, txtRecord.Txt, txtDomain)
	require.True(t, txtRecord.Locked)
	require.Equal(t, txtRecord.Timestamp, int64(0))
}

// TestDNSCache_LRU checks the correctness of cache against LRU ejection.
func TestDNSCache_LRU(t *testing.T) {
	total := 700             // total entries to store (i.e., 700 ip and 700 txt domains)
	sizeLimit := uint32(500) // cache size limit (i.e., 500 ip and 500 txt domains)

	ipFixtures := network.IpLookupListFixture(total)
	txtFixtures := network.TxtLookupListFixture(total)

	cache := herocache.NewDNSCache(sizeLimit, unittest.Logger(), metrics.NewNoopCollector(), metrics.NewNoopCollector())

	// cache must be initially empty
	ips, txts := cache.Size()
	require.Equal(t, uint(0), ips)
	require.Equal(t, uint(0), txts)

	// adding 700 txt and 700 ip domains to cache
	for _, fixture := range ipFixtures {
		require.True(t, cache.PutIpDomain(fixture.Domain, fixture.Result, fixture.TimeStamp))
	}

	for _, fixture := range txtFixtures {
		require.True(t, cache.PutTxtRecord(fixture.Txt, fixture.Records, fixture.TimeStamp))
	}

	// cache must be full up to its limit
	ips, txts = cache.Size()
	require.Equal(t, uint(sizeLimit), ips)
	require.Equal(t, uint(sizeLimit), txts)

	// only last 500 ip domains and txt records must be retained in the DNS cache
	for i := 0; i < total; i++ {
		if i < total-int(sizeLimit) {
			// old dns entries must be ejected
			// ip
			ipRecord, ok := cache.GetDomainIp(ipFixtures[i].Domain)
			require.False(t, ok)
			require.Nil(t, ipRecord)
			// txt records
			txt, ok := cache.GetTxtRecord(txtFixtures[i].Txt)
			require.False(t, ok)
			require.Nil(t, txt)

			continue
		}

		// new dns entries must be persisted
		// ip
		ipRecord, ok := cache.GetDomainIp(ipFixtures[i].Domain)
		require.True(t, ok)
		require.Equal(t, ipFixtures[i].Result, ipRecord.Addresses)
		require.Equal(t, ipFixtures[i].TimeStamp, ipRecord.Timestamp)
		// txt records
		txtRecord, ok := cache.GetTxtRecord(txtFixtures[i].Txt)
		require.True(t, ok)
		require.Equal(t, txtFixtures[i].Records, txtRecord.Records)
		require.Equal(t, txtFixtures[i].TimeStamp, txtRecord.Timestamp)
	}
}

// testAddToCache is a test helper that adds ip and txt records to the cache.
func testAddToCache(t *testing.T,
	cache *herocache.DNSCache,
	ipTestCases map[string]*network.IpLookupTestCase,
	txtTestCases map[string]*network.TxtLookupTestCase) {

	wg := sync.WaitGroup{}
	wg.Add(len(ipTestCases) + len(txtTestCases))

	for _, fixture := range ipTestCases {
		require.True(t, cache.PutIpDomain(fixture.Domain, fixture.Result, fixture.TimeStamp))
	}

	for _, fixture := range txtTestCases {
		require.True(t, cache.PutTxtRecord(fixture.Txt, fixture.Records, fixture.TimeStamp))
	}
}

// TestDNSCache_Remove checks the correctness of cache against removal.
func TestDNSCache_Remove(t *testing.T) {
	total := 30              // total entries to store (i.e., 700 ip domains and 700 txt records)
	sizeLimit := uint32(500) // cache size limit (i.e., 500 ip domains and 500 txt records)

	ipFixtures := network.IpLookupListFixture(total)
	txtFixtures := network.TxtLookupListFixture(total)

	cache := herocache.NewDNSCache(sizeLimit, unittest.Logger(), metrics.NewNoopCollector(), metrics.NewNoopCollector())

	// cache must be initially empty
	ips, txts := cache.Size()
	require.Equal(t, uint(0), ips)
	require.Equal(t, uint(0), txts)

	// adding 700 txt records and 700 ip domains to cache
	for _, fixture := range ipFixtures {
		require.True(t, cache.PutIpDomain(fixture.Domain, fixture.Result, fixture.TimeStamp))
	}

	for _, fixture := range txtFixtures {
		require.True(t, cache.PutTxtRecord(fixture.Txt, fixture.Records, fixture.TimeStamp))
	}

	// cache must be full up to its limit
	ips, txts = cache.Size()
	require.Equal(t, uint(total), ips)
	require.Equal(t, uint(total), txts)

	// removes a single ip domains and txt records
	require.True(t, cache.RemoveIp(ipFixtures[0].Domain))
	require.True(t, cache.RemoveTxt(txtFixtures[0].Txt))
	// repeated attempts on removing already removed entries must return false.
	require.False(t, cache.RemoveIp(ipFixtures[0].Domain))
	require.False(t, cache.RemoveTxt(txtFixtures[0].Txt))

	// size must be updated post removal
	ips, txts = cache.Size()
	require.Equal(t, uint(total-1), ips)
	require.Equal(t, uint(total-1), txts)

	// only last 500 ip domains and txt records must be retained in the DNS cache
	for i := 0; i < total; i++ {
		if i == 0 {
			// removed entries must no longer exist.
			// ip
			ipRecord, ok := cache.GetDomainIp(ipFixtures[i].Domain)
			require.False(t, ok)
			require.Nil(t, ipRecord)
			// txt records
			txtRecord, ok := cache.GetTxtRecord(txtFixtures[i].Txt)
			require.False(t, ok)
			require.Nil(t, txtRecord)

			continue
		}

		// other entries must be existing.
		// ip
		ipRecord, ok := cache.GetDomainIp(ipFixtures[i].Domain)
		require.True(t, ok)
		require.Equal(t, ipFixtures[i].Result, ipRecord.Addresses)
		require.Equal(t, ipFixtures[i].TimeStamp, ipRecord.Timestamp)
		// txt records
		txtRecord, ok := cache.GetTxtRecord(txtFixtures[i].Txt)
		require.True(t, ok)
		require.Equal(t, txtFixtures[i].Records, txtRecord.Records)
		require.Equal(t, txtFixtures[i].TimeStamp, txtRecord.Timestamp)
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
		ipRecord, ok := cache.GetDomainIp(tc.Domain)
		if !ok {
			continue
		}
		require.True(t, ok)

		require.Equal(t, tc.TimeStamp, ipRecord.Timestamp)
		require.Equal(t, tc.Result, ipRecord.Addresses)
		actualCount++
	}
	require.Equal(t, count, actualCount)

	// checking txt records
	actualCount = 0
	for _, tc := range txtTestCases {
		txtRecord, ok := cache.GetTxtRecord(tc.Txt)
		if !ok {
			continue
		}
		require.True(t, ok)

		require.Equal(t, tc.TimeStamp, txtRecord.Timestamp)
		require.Equal(t, tc.Records, txtRecord.Records)
		actualCount++
	}
	require.Equal(t, count, actualCount)

}
