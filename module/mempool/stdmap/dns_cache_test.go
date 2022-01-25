package stdmap_test

import (
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/network"
)

func TestDNSCache(t *testing.T) {
	count := 501
	ipFixtures := network.IpLookupFixture(count)
	txtFixtures := network.TxtLookupFixture(count)
	cache := stdmap.NewDNSCache(500, unittest.Logger())

	wg := sync.WaitGroup{}
	wg.Add(2 * count)

	for _, fixture := range ipFixtures {
		go func(fixture *network.IpLookupTestCase) {
			cache.PutIpDomain(fixture.Domain, 1, fixture.Result)
			wg.Done()
		}(fixture)
	}

	for _, fixture := range txtFixtures {
		go func(fixture *network.TxtLookupTestCase) {
			cache.PutTxtDomain(fixture.Domain, 1, fixture.Result)
			wg.Done()
		}(fixture)
	}

	unittest.RequireReturnsBefore(t, wg.Done, 1*time.Second, "could not write all test cases on time")
}
