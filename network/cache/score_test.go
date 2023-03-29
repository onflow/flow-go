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
	require.NoError(t, cache.Update(netcache.AppScoreRecord{
		PeerID:      "peer1",
		Decay:       0.1,
		Score:       0.5,
		LastUpdated: time.Now(),
	}))

	// tests updating an existing entry in the cache.
	require.NoError(t, cache.Update(netcache.AppScoreRecord{
		PeerID:      "peer1",
		Decay:       0.1,
		Score:       0.5,
		LastUpdated: time.Now(),
	}))

	// makes the cache full.
	for i := 0; i < 100; i++ {
		require.NoError(t, cache.Update(netcache.AppScoreRecord{
			PeerID:      peer.ID(fmt.Sprintf("peer%d", i)),
			Decay:       0.1,
			Score:       0.5,
			LastUpdated: time.Now(),
		}))
	}

	// adding a new entry to the cache should fail.
	require.Error(t, cache.Update(netcache.AppScoreRecord{
		PeerID:      "peer101",
		Decay:       0.1,
		Score:       0.5,
		LastUpdated: time.Now(),
	}))

	// retrieving an existing entity should work.
	for i := 0; i < 100; i++ {
		record, ok := cache.Get(peer.ID(fmt.Sprintf("peer%d", i)))
		require.True(t, ok)

		require.Equal(t, 0.1, record.Decay)
		require.Equal(t, 0.5, record.Score)
		require.GreaterOrEqual(t, time.Now(), record.LastUpdated)
	}

	// yet updating an existing entry should still work.
	require.NoError(t, cache.Update(netcache.AppScoreRecord{
		PeerID:      "peer1",
		Decay:       0.2,
		Score:       0.8,
		LastUpdated: time.Now(),
	}))
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
			err := cache.Update(netcache.AppScoreRecord{
				PeerID:      peer.ID(peerID),
				Decay:       0.1 * float64(num),
				Score:       float64(num),
				LastUpdated: time.Now(),
			})
			require.NoError(t, err)
		}(i)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not update all records concurrently on time")

	// checks if the cache can retrieve all records.
	for i := 0; i < numRecords; i++ {
		peerID := fmt.Sprintf("peer%d", i)
		record, found := cache.Get(peer.ID(peerID))
		require.True(t, found)

		expectedScore := float64(i)
		require.Equal(t, expectedScore, record.Score,
			"Get() returned incorrect Score for record %s: expected %f, got %f", peerID, expectedScore, record.Score)
		expectedDecay := 0.1 * float64(i)
		require.Equal(t, expectedDecay, record.Decay,
			"Get() returned incorrect Decay for record %s: expected %f, got %f", peerID, expectedDecay, record.Decay)
		require.GreaterOrEqual(t, time.Now(), record.LastUpdated)
	}
}
