package netcache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAppScoreCache_Update tests the Update method of the AppScoreCache. It tests if the cache
// can add a new entry, update an existing entry, and fail to add a new entry when the cache is full.
func TestAppScoreCache_Update(t *testing.T) {
	// create a new instance of AppScoreCache.
	cache := netcache.NewAppScoreCache(100, unittest.Logger(), metrics.NewNoopCollector())

	// tests adding a new entry to the cache.
	require.NoError(t, cache.Update("peer1", 0.1, 0.5))

	// tests updating an existing entry in the cache.
	require.NoError(t, cache.Update("peer1", 0.2, 0.8))

	// makes the cache full.
	for i := 0; i < 100; i++ {
		require.NoError(t, cache.Update(peer.ID(fmt.Sprintf("peer%d", i)), 0.1, 0.5))
	}

	// adding a new entry to the cache should fail.
	require.Error(t, cache.Update("peer101", 0.1, 0.5))

	// retrieving an existing entity should work.
	for i := 0; i < 100; i++ {
		score, decay, ok := cache.Get(peer.ID(fmt.Sprintf("peer%d", i)))
		require.True(t, ok)

		require.Equal(t, 0.1, decay)
		require.Equal(t, 0.5, score)
	}

	// yet updating an existing entry should still work.
	require.NoError(t, cache.Update("peer1", 0.2, 0.8))

}

// TestConcurrentUpdateAndGet tests if the cache can be updated and retrieved concurrently.
// It updates the cache with a number of records concurrently and then checks if the cache
// can retrieve all records.
func TestConcurrentUpdateAndGet(t *testing.T) {
	cache := netcache.NewAppScoreCache(200, unittest.Logger(), metrics.NewNoopCollector())

	// defines the number of records to update.
	numRecords := 100

	// uses a wait group to wait for all goroutines to finish.
	var wg sync.WaitGroup
	wg.Add(numRecords)

	// Update the records concurrently.
	for i := 0; i < numRecords; i++ {
		go func(num int) {
			defer wg.Done()
			peerID := fmt.Sprintf("peer%d", num)
			err := cache.Update(peer.ID(peerID), 0.1*float64(num), float64(num))
			require.NoError(t, err)
		}(i)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not update all records concurrently on time")

	// checks if the cache can retrieve all records.
	for i := 0; i < numRecords; i++ {
		peerID := fmt.Sprintf("peer%d", i)
		score, decay, found := cache.Get(peer.ID(peerID))
		require.True(t, found)

		expectedScore := float64(i)
		require.Equal(t, expectedScore, score, "Get() returned incorrect score for record %s: expected %f, got %f", peerID, expectedScore, score)
		expectedDecay := 0.1 * float64(i)
		require.Equal(t, expectedDecay, decay, "Get() returned incorrect decay for record %s: expected %f, got %f", peerID, expectedDecay, decay)
	}
}
