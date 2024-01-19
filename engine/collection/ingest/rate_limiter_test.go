package ingest_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

var _ access.RateLimiter = (*ingest.AddressRateLimiter)(nil)

func TestLimiterAddRemoveAddress(t *testing.T) {
	t.Parallel()

	good1 := unittest.RandomAddressFixture()
	limited1 := unittest.RandomAddressFixture()
	limited2 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	burst := 1
	l := ingest.NewAddressRateLimiter(numPerSec, burst)

	require.False(t, l.IsRateLimited(good1))
	require.False(t, l.IsRateLimited(good1)) // address are not limited

	l.AddAddress(limited1)
	require.Equal(t, []flow.Address{limited1}, l.GetAddresses())

	require.False(t, l.IsRateLimited(limited1)) // address 1 is not limited on the first call
	require.True(t, l.IsRateLimited(limited1))  // limited on the second call immediately
	require.True(t, l.IsRateLimited(limited1))  // limited on the second call immediately

	require.False(t, l.IsRateLimited(good1))
	require.False(t, l.IsRateLimited(good1)) // address are not limited

	l.AddAddress(limited2)
	list := l.GetAddresses()
	require.Len(t, list, 2)
	require.Contains(t, list, limited1)

	require.False(t, l.IsRateLimited(limited2)) // address 2 is not limited on the first call
	require.True(t, l.IsRateLimited(limited2))  // limited on the second call immediately
	require.True(t, l.IsRateLimited(limited2))  // limited on the second call immediately

	l.RemoveAddress(limited1) // after remove the limit, it no longer limited
	require.False(t, l.IsRateLimited(limited1))
	require.False(t, l.IsRateLimited(limited1))

	// but limit2 is still limited
	require.True(t, l.IsRateLimited(limited2))
}

func TestLimiterBurst(t *testing.T) {
	t.Parallel()

	limited1 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	burst := 3
	l := ingest.NewAddressRateLimiter(numPerSec, burst)

	l.AddAddress(limited1)
	for i := 0; i < burst; i++ {
		require.False(t, l.IsRateLimited(limited1), fmt.Sprintf("%v-nth call", i))
	}

	require.True(t, l.IsRateLimited(limited1)) // limited
	require.True(t, l.IsRateLimited(limited1)) // limited
}

// verify that if wait long enough after rate limited
func TestLimiterWaitLongEnough(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()

	// with limit set to 10, it means we allow 10 messages per second,
	// and with burst set to 1, it means we only allow 1 message at a time,
	// so the limit is 1 message per 100 milliseconds.
	// Note rate.Limit(0.1) is not to set 1 message per 100 milliseconds, but
	// 1 message per 10 seconds.
	numPerSec := rate.Limit(10)
	burst := 1
	l := ingest.NewAddressRateLimiter(numPerSec, burst)

	l.AddAddress(addr1)
	require.False(t, l.IsRateLimited(addr1))
	require.True(t, l.IsRateLimited(addr1))

	// check every 10 Millisecond then after 100 Millisecond it should be allowed
	require.Eventually(t, func() bool {
		return l.Allow(addr1)
	}, 110*time.Millisecond, 10*time.Millisecond)

	// block until another 100 ms
	require.False(t, l.Allow(addr1))
}

func TestLimiterConcurrentSafe(t *testing.T) {
	t.Parallel()
	good1 := unittest.RandomAddressFixture()
	limited1 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	burst := 1
	l := ingest.NewAddressRateLimiter(numPerSec, burst)

	l.AddAddress(limited1)

	wg := sync.WaitGroup{}
	wg.Add(2)

	succeed := atomic.NewUint64(0)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		ok := l.IsRateLimited(limited1)
		if ok {
			succeed.Add(1)
		}
		require.False(t, l.IsRateLimited(good1)) // never limited
	}(&wg)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		ok := l.IsRateLimited(limited1)
		if ok {
			succeed.Add(1)
		}
		require.False(t, l.IsRateLimited(good1)) // never limited
	}(&wg)

	wg.Wait()
	require.Equal(t, uint64(1), succeed.Load()) // should only succeed once
}
