package dns

import (
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/utils/unittest"
)

func BenchmarkCache(b *testing.B) {
	b.StopTimer()

	c := newCache()
	testCases := ipLookupFixture(100_000)
	wg := &sync.WaitGroup{}

	b.StartTimer()

	for i := 0; i < b.N; i++ {

		wg.Add(3 * len(testCases))

		for _, tc := range testCases {
			domain := tc.domain
			result := tc.result

			go func() {

				c.updateIPCache(domain, result)
				wg.Done()

			}()

			go func() {

				c.resolveIPCache(domain)
				wg.Done()

			}()

			go func() {

				c.invalidateIPCacheEntry(domain)
				wg.Done()

			}()
		}

		unittest.RequireReturnsBefore(b, wg.Wait, 1*time.Minute, "could not complete operations on time")
	}
}
