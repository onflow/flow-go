package timeoutcollector

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTimeoutObjectsCache_View tests that View returns same value that was set by constructor
func TestTimeoutObjectsCache_View(t *testing.T) {
	view := uint64(100)
	cache := NewTimeoutObjectsCache(view)
	require.Equal(t, view, cache.View())
}

// TestTimeoutObjectsCache_AddTimeoutObjectRepeatedTimeout tests that AddTimeoutObject skips duplicated timeouts
func TestTimeoutObjectsCache_AddTimeoutObjectRepeatedTimeout(t *testing.T) {
	t.Parallel()

	view := uint64(100)
	cache := NewTimeoutObjectsCache(view)
	timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(view))

	require.NoError(t, cache.AddTimeoutObject(timeout))
	err := cache.AddTimeoutObject(timeout)
	require.ErrorIs(t, err, ErrRepeatedTimeout)
	require.Len(t, cache.All(), 1)
}

// TestTimeoutObjectsCache_AddTimeoutObjectIncompatibleView tests that adding timeout with incompatible view results in error
func TestTimeoutObjectsCache_AddTimeoutObjectIncompatibleView(t *testing.T) {
	t.Parallel()

	view := uint64(100)
	cache := NewTimeoutObjectsCache(view)
	timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(view + 1))
	err := cache.AddTimeoutObject(timeout)
	require.ErrorIs(t, err, ErrTimeoutForIncompatibleView)
}

// TestTimeoutObjectsCache_GetTimeout tests that GetTimeout method
func TestTimeoutObjectsCache_GetTimeout(t *testing.T) {
	view := uint64(100)
	knownTimeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(view))
	doubleTimeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(view),
		helper.WithTimeoutObjectSignerID(knownTimeout.SignerID))

	cache := NewTimeoutObjectsCache(view)

	// unknown timeout
	timeout, found := cache.GetTimeoutObject(unittest.IdentifierFixture())
	require.Nil(t, timeout)
	require.False(t, found)

	// known timeout
	err := cache.AddTimeoutObject(knownTimeout)
	require.NoError(t, err)
	timeout, found = cache.GetTimeoutObject(knownTimeout.SignerID)
	require.Equal(t, knownTimeout, timeout)
	require.True(t, found)

	// for a signer ID with a known timeout, the cache should memorize the _first_ encountered timeout
	err = cache.AddTimeoutObject(doubleTimeout)
	require.True(t, model.IsDoubleTimeoutError(err))
	timeout, found = cache.GetTimeoutObject(doubleTimeout.SignerID)
	require.Equal(t, knownTimeout, timeout)
	require.True(t, found)
}

// TestTimeoutObjectsCache_All tests that All returns previously added timeouts.
func TestTimeoutObjectsCache_All(t *testing.T) {
	t.Parallel()

	view := uint64(100)
	cache := NewTimeoutObjectsCache(view)
	expectedTimeouts := make([]*model.TimeoutObject, 5)
	for i := range expectedTimeouts {
		timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(view))
		expectedTimeouts[i] = timeout
		require.NoError(t, cache.AddTimeoutObject(timeout))
	}
	require.ElementsMatch(t, expectedTimeouts, cache.All())
}

// BenchmarkAdd measured the time it takes to add `numberTimeouts` concurrently to the TimeoutObjectsCache.
// On MacBook with Intel i7-7820HQ CPU @ 2.90GHz:
// adding 1 million timeouts in total, with 20 threads concurrently, took 0.48s
func BenchmarkAdd(b *testing.B) {
	numberTimeouts := 1_000_000
	threads := 20

	// Setup: create worker routines and timeouts to feed
	view := uint64(10)
	cache := NewTimeoutObjectsCache(view)

	var start sync.WaitGroup
	start.Add(threads)
	var done sync.WaitGroup
	done.Add(threads)

	n := numberTimeouts / threads

	for ; threads > 0; threads-- {
		go func(i int) {
			// create timeouts and signal ready
			timeouts := make([]model.TimeoutObject, 0, n)
			for len(timeouts) < n {
				t := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(view))
				timeouts = append(timeouts, *t)
			}
			start.Done()

			// Wait for last worker routine to signal ready. Then,
			// feed all timeouts into cache
			start.Wait()
			for _, v := range timeouts {
				err := cache.AddTimeoutObject(&v)
				require.NoError(b, err)
			}
			done.Done()
		}(threads)
	}
	start.Wait()
	t1 := time.Now()
	done.Wait()
	duration := time.Since(t1)
	fmt.Printf("=> adding %d timeouts to Cache took %f seconds\n", cache.Size(), duration.Seconds())
}
