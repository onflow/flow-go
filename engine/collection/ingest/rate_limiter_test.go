package ingest

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestLimiterOK(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()
	addr2 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	l := NewAddressRateLimiter(numPerSec)

	require.False(t, l.IsRateLimited(addr1))
	require.True(t, l.IsRateLimited(addr1)) // rate limitted

	require.False(t, l.IsRateLimited(addr2)) // addr1 limitted does not affect addr2
	require.True(t, l.IsRateLimited(addr2))  // addr2 rate limited
}

// verify that if wait long enough after rate limited
func TestLimiterWaitLongEnough(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	l := NewAddressRateLimiter(numPerSec)

	require.False(t, l.IsRateLimited(addr1))

	time.Sleep(time.Millisecond * 1100) // wait 1.1 second
	require.False(t, l.IsRateLimited(addr1))
	require.True(t, l.IsRateLimited(addr1))
}

func TestLimiterCleanup(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()

	numPerSec := rate.Limit(1)
	l := NewAddressRateLimiter(numPerSec)

	l.Cleanup()
	require.False(t, l.IsRateLimited(addr1))
	l.Cleanup()
	require.True(t, l.IsRateLimited(addr1))

	time.Sleep(time.Millisecond * 1100)
	l.Cleanup()

	require.Equal(t, 0, len(l.limiters))
	require.False(t, l.IsRateLimited(addr1))
	require.True(t, l.IsRateLimited(addr1))
}
