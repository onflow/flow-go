package netcache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
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
		PeerID: "peer1",
		Decay:  0.1,
		Score:  0.5,
	}))

	// tests updating an existing entry in the cache.
	require.NoError(t, cache.Update(netcache.AppScoreRecord{
		PeerID: "peer1",
		Decay:  0.1,
		Score:  0.5,
	}))

	// makes the cache full.
	for i := 0; i < 100; i++ {
		require.NoError(t, cache.Update(netcache.AppScoreRecord{
			PeerID: peer.ID(fmt.Sprintf("peer%d", i)),
			Decay:  0.1,
			Score:  0.5,
		}))
	}

	// adding a new entry to the cache should fail.
	require.Error(t, cache.Update(netcache.AppScoreRecord{
		PeerID: "peer101",
		Decay:  0.1,
		Score:  0.5,
	}))

	// retrieving an existing entity should work.
	for i := 0; i < 100; i++ {
		record, err, ok := cache.Get(peer.ID(fmt.Sprintf("peer%d", i)))
		require.True(t, ok)
		require.NoError(t, err)

		require.Equal(t, 0.1, record.Decay)
		require.Equal(t, 0.5, record.Score)
	}

	// yet updating an existing entry should still work.
	require.NoError(t, cache.Update(netcache.AppScoreRecord{
		PeerID: "peer1",
		Decay:  0.2,
		Score:  0.8,
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
				PeerID: peer.ID(peerID),
				Decay:  0.1 * float64(num),
				Score:  float64(num),
			})
			require.NoError(t, err)
		}(i)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not update all records concurrently on time")

	// checks if the cache can retrieve all records.
	for i := 0; i < numRecords; i++ {
		peerID := fmt.Sprintf("peer%d", i)
		record, err, found := cache.Get(peer.ID(peerID))
		require.True(t, found)
		require.NoError(t, err)

		expectedScore := float64(i)
		require.Equal(t, expectedScore, record.Score,
			"Get() returned incorrect Score for record %s: expected %f, got %f", peerID, expectedScore, record.Score)
		expectedDecay := 0.1 * float64(i)
		require.Equal(t, expectedDecay, record.Decay,
			"Get() returned incorrect Decay for record %s: expected %f, got %f", peerID, expectedDecay, record.Decay)
	}
}

// TestAppScoreRecordStoredByValue tests if the cache stores the AppScoreRecord by value.
// It updates the cache with a record and then modifies the record. It then checks if the
// record in the cache is still the original record. This is a desired behavior that
// is guaranteed by the HeroCache library. In other words, we don't desire the records to be
// externally mutable after they are added to the cache (unless by a subsequent call to Update).
func TestAppScoreRecordStoredByValue(t *testing.T) {
	cache := netcache.NewAppScoreCache(200, unittest.Logger(), metrics.NewNoopCollector())

	peerID := "peer1"
	err := cache.Update(netcache.AppScoreRecord{
		PeerID: peer.ID(peerID),
		Decay:  0.1,
		Score:  0.5,
	})
	require.NoError(t, err)

	// get the record from the cache
	record, err, found := cache.Get(peer.ID(peerID))
	require.True(t, found)

	// modify the record
	record.Decay = 0.2
	record.Score = 0.8

	// get the record from the cache again
	record, err, found = cache.Get(peer.ID(peerID))
	require.True(t, found)

	// check if the record is still the same
	require.Equal(t, 0.1, record.Decay)
	require.Equal(t, 0.5, record.Score)
}

// TestAppScoreCache_Get_WithPreprocessors tests if the cache applies the preprocessors to the records
// before returning them. It adds a record to the cache and then checks if the preprocessors were
// applied to the record. It also checks if the preprocessors were applied in the correct order.
// The first preprocessor adds 1 to the score and the second preprocessor multiplies the score by 2.
// Therefore, the expected score is 4.
// Note that the preprocessors are applied in the order they are passed to the cache.
func TestAppScoreCache_Get_WithPreprocessors(t *testing.T) {
	cache := netcache.NewAppScoreCache(10, unittest.Logger(), metrics.NewNoopCollector(),
		// first preprocessor: adds 1 to the score.
		func(record netcache.AppScoreRecord, lastUpdated time.Time) netcache.AppScoreRecord {
			record.Score++
			return record
		},
		// second preprocessor: multiplies the score by 2
		func(record netcache.AppScoreRecord, lastUpdated time.Time) netcache.AppScoreRecord {
			record.Score *= 2
			return record
		},
	)

	record := netcache.AppScoreRecord{
		PeerID: "peerA",
		Decay:  0.5,
		Score:  1,
	}
	err := cache.Update(record)
	assert.NoError(t, err)

	// verifies that the preprocessors were called and the score was updated accordingly.
	cachedRecord, err, ok := cache.Get("peerA")
	assert.NoError(t, err)
	assert.True(t, ok)

	// expected score is 4: the first preprocessor adds 1 to the score and the second preprocessor multiplies the score by 2.
	// (1 + 1) * 2 = 4
	assert.Equal(t, 4.0, cachedRecord.Score)               // score should be updated
	assert.Equal(t, 0.5, cachedRecord.Decay)               // decay should not be modified
	assert.Equal(t, peer.ID("peerA"), cachedRecord.PeerID) // peerID should not be modified
}

// TestAppScoreCache_Get_WithNoPreprocessors tests when no preprocessors are provided to the cache constructor
// that the cache returns the original record without any modifications.
func TestAppScoreCache_Get_WithNoPreprocessors(t *testing.T) {
	cache := netcache.NewAppScoreCache(10, unittest.Logger(), metrics.NewNoopCollector())

	record := netcache.AppScoreRecord{
		PeerID: "peerA",
		Decay:  0.5,
		Score:  1,
	}
	err := cache.Update(record)
	assert.NoError(t, err)

	// verifies that no preprocessors were called and the score was not updated.
	cachedRecord, err, ok := cache.Get("peerA")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1.0, cachedRecord.Score)
	assert.Equal(t, 0.5, cachedRecord.Decay)
	assert.Equal(t, peer.ID("peerA"), cachedRecord.PeerID)
}
