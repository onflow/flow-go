package limiters

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrencyLimiter_Allow_WithinLimit verifies that Allow executes the function
// and returns true when below the concurrency limit.
func TestConcurrencyLimiter_Allow_WithinLimit(t *testing.T) {
	limiter, err := NewConcurrencyLimiter(2)
	require.NoError(t, err)
	executed := false
	allowed := limiter.Allow(func() { executed = true })
	require.True(t, allowed)
	assert.True(t, executed)
}

// TestConcurrencyLimiter_Allow_AtLimit verifies that Allow returns false without
// executing the function when the concurrency limit is reached.
func TestConcurrencyLimiter_Allow_AtLimit(t *testing.T) {
	limiter, err := NewConcurrencyLimiter(1)
	require.NoError(t, err)

	// Block the first call inside Allow so the counter stays at 1.
	started := make(chan struct{})
	unblock := make(chan struct{})

	var wg sync.WaitGroup
	wg.Go(func() {
		limiter.Allow(func() {
			close(started)
			<-unblock
		})
	})

	// Wait until the goroutine is inside fn (counter == 1).
	<-started

	// A second call must be rejected.
	executed := false
	allowed := limiter.Allow(func() { executed = true })
	assert.False(t, allowed)
	assert.False(t, executed)

	close(unblock)
	wg.Wait()
}

// TestConcurrencyLimiter_Allow_CounterDecrement verifies that the internal counter
// returns to zero after Allow completes so subsequent calls are accepted again.
func TestConcurrencyLimiter_Allow_CounterDecrement(t *testing.T) {
	limiter, err := NewConcurrencyLimiter(1)
	require.NoError(t, err)

	allowed := limiter.Allow(func() {})
	require.True(t, allowed)

	// Counter must be decremented; second call should succeed.
	allowed = limiter.Allow(func() {})
	assert.True(t, allowed)
}

// TestConcurrencyLimiter_Allow_CounterDecrementOnPanic verifies that the internal
// counter is decremented even when the supplied function panics.
func TestConcurrencyLimiter_Allow_CounterDecrementOnPanic(t *testing.T) {
	limiter, err := NewConcurrencyLimiter(1)
	require.NoError(t, err)

	require.Panics(t, func() {
		limiter.Allow(func() { panic("boom") })
	})

	// Counter must have been decremented via defer; next call must succeed.
	executed := false
	allowed := limiter.Allow(func() { executed = true })
	assert.True(t, allowed)
	assert.True(t, executed)
}

// TestConcurrencyLimiter_NewZeroLimit verifies that the constructor returns an error
// when the limit is zero.
func TestConcurrencyLimiter_NewZeroLimit(t *testing.T) {
	_, err := NewConcurrencyLimiter(0)
	assert.Error(t, err)
}

// TestConcurrencyLimiter_Allow_ConcurrentCalls verifies that at most maxConcurrent
// goroutines execute fn simultaneously across a burst of concurrent callers.
func TestConcurrencyLimiter_Allow_ConcurrentCalls(t *testing.T) {
	const maxConcurrent = 5
	const totalGoroutines = 50

	limiter, err := NewConcurrencyLimiter(maxConcurrent)
	require.NoError(t, err)

	var (
		peak    atomic.Int32 // highest observed concurrent executions
		current atomic.Int32
		wg      sync.WaitGroup
	)

	start := make(chan struct{})

	for i := 0; i < totalGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			limiter.Allow(func() {
				n := current.Add(1)
				// Record peak without locking; a slightly stale read is acceptable
				// because we only care about the maximum ever observed.
				for {
					old := peak.Load()
					if n <= old || peak.CompareAndSwap(old, n) {
						break
					}
				}
				time.Sleep(time.Millisecond) // hold the slot briefly
				current.Add(-1)
			})
		}()
	}

	close(start)
	wg.Wait()

	assert.LessOrEqual(t, peak.Load(), int32(maxConcurrent),
		"peak concurrent executions must not exceed maxConcurrent")
}
