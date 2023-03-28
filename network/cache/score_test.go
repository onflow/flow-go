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
