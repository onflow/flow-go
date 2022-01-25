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
	count := 501
	ipFixtures := network.IpLookupFixture(count)
	txtFixtures := network.TxtLookupFixture(count)
	cache := stdmap.NewDNSCache(500, unittest.Logger())

	// cache must be initially empty
	ips, txts := cache.Size()
	require.Equal(t, uint(0), ips)
	require.Equal(t, uint(0), txts)

	wg := sync.WaitGroup{}
	wg.Add(2 * count)

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
	require.Equal(t, uint(500), ips)
	require.Equal(t, uint(500), txts)
}
