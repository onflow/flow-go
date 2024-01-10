package ingest_test

import (
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/time/rate"
)

var _ access.RateLimiter = (*ingest.AddressRateLimiter)(nil)

func TestLimiterAddRemoveAddress(t *testing.T) {
	t.Parallel()

	good1 := unittest.RandomAddressFixture()
	limited1 := unittest.RandomAddressFixture()
	limited2 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	l := ingest.NewAddressRateLimiter(numPerSec)

	require.False(t, l.IsRateLimited(good1))
	require.False(t, l.IsRateLimited(good1)) // address are not limited

	l.AddAddress(limited1)
	require.False(t, l.IsRateLimited(limited1)) // address 1 is not limitted on the first call
	require.True(t, l.IsRateLimited(limited1))  // limitted on the second call immediately
	require.True(t, l.IsRateLimited(limited1))  // limitted on the second call immediately

	require.False(t, l.IsRateLimited(good1))
	require.False(t, l.IsRateLimited(good1)) // address are not limited

	l.AddAddress(limited2)
	require.False(t, l.IsRateLimited(limited2)) // address 1 is not limitted on the first call
	require.True(t, l.IsRateLimited(limited2))  // limitted on the second call immediately
	require.True(t, l.IsRateLimited(limited2))  // limitted on the second call immediately

	l.RemoveAddress(limited1) // after remove the limit, it no longer limited
	require.False(t, l.IsRateLimited(limited1))
	require.False(t, l.IsRateLimited(limited1))

	// but limit2 is still limited
	require.True(t, l.IsRateLimited(limited2))
}

// verify that if wait long enough after rate limited
func TestLimiterWaitLongEnough(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	l := ingest.NewAddressRateLimiter(numPerSec)

	l.AddAddress(addr1)
	require.False(t, l.IsRateLimited(addr1))
	require.True(t, l.IsRateLimited(addr1))

	time.Sleep(time.Millisecond * 1100) // wait 1.1 second
	require.False(t, l.IsRateLimited(addr1))
	require.True(t, l.IsRateLimited(addr1))
}

func TestLimiterConcurrentSafe(t *testing.T) {
	t.Parallel()
	good1 := unittest.RandomAddressFixture()
	limited1 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	l := ingest.NewAddressRateLimiter(numPerSec)

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
