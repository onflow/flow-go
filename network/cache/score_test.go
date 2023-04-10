package netcache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/metrics"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubSpamRecordCache_Add tests the Add method of the GossipSubSpamRecordCache. It tests
// adding a new record to the cache.
func TestGossipSubSpamRecordCache_Add(t *testing.T) {
	// create a new instance of GossipSubSpamRecordCache.
	cache := netcache.NewGossipSubSpamRecordCache(100, unittest.Logger(), metrics.NewNoopCollector())

	// tests adding a new record to the cache.
	require.True(t, cache.Add("peer0", netcache.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// tests updating an existing record in the cache.
	require.False(t, cache.Add("peer0", netcache.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// makes the cache full.
	for i := 1; i < 100; i++ {
		require.True(t, cache.Add(peer.ID(fmt.Sprintf("peer%d", i)), netcache.GossipSubSpamRecord{
			Decay:   0.1,
			Penalty: 0.5,
		}))
	}

	// adding a new record to the cache should fail.
	require.False(t, cache.Add("peer101", netcache.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// retrieving an existing record should work.
	for i := 0; i < 100; i++ {
		record, err, ok := cache.Get(peer.ID(fmt.Sprintf("peer%d", i)))
		require.True(t, ok)
		require.NoError(t, err)

		require.Equal(t, 0.1, record.Decay)
		require.Equal(t, 0.5, record.Penalty)
	}

	// yet attempting on adding an existing record should fail.
	require.False(t, cache.Add("peer1", netcache.GossipSubSpamRecord{
		Decay:   0.2,
		Penalty: 0.8,
	}))
}

// TestGossipSubSpamRecordCache_Concurrent_Add tests if the cache can be added and retrieved concurrently.
// It updates the cache with a number of records concurrently and then checks if the cache
// can retrieve all records.
func TestGossipSubSpamRecordCache_Concurrent_Add(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector())

	// defines the number of records to update.
	numRecords := 100

	// uses a wait group to wait for all goroutines to finish.
	var wg sync.WaitGroup
	wg.Add(numRecords)

	// adds the records concurrently.
	for i := 0; i < numRecords; i++ {
		go func(num int) {
			defer wg.Done()
			peerID := fmt.Sprintf("peer%d", num)
			added := cache.Add(peer.ID(peerID), netcache.GossipSubSpamRecord{
				Decay:   0.1 * float64(num),
				Penalty: float64(num),
			})
			require.True(t, added)
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
		require.Equal(t, expectedScore, record.Penalty,
			"Get() returned incorrect penalty for record %s: expected %f, got %f", peerID, expectedScore, record.Penalty)
		expectedDecay := 0.1 * float64(i)
		require.Equal(t, expectedDecay, record.Decay,
			"Get() returned incorrect decay for record %s: expected %f, got %f", peerID, expectedDecay, record.Decay)
	}
}

// TestAdjust tests the Adjust method of the GossipSubSpamRecordCache. It tests if the cache can adjust
// the score of an existing record and fail to adjust the score of a non-existing record.
func TestAdjust(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector())

	peerID := "peer1"

	// tests adjusting the score of an existing record.
	require.True(t, cache.Add(peer.ID(peerID), netcache.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))
	record, err := cache.Adjust(peer.ID(peerID), func(record netcache.GossipSubSpamRecord) netcache.GossipSubSpamRecord {
		record.Penalty = 0.7
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 0.7, record.Penalty) // checks if the score is adjusted correctly.

	// tests adjusting the score of a non-existing record.
	record, err = cache.Adjust(peer.ID("peer2"), func(record netcache.GossipSubSpamRecord) netcache.GossipSubSpamRecord {
		require.Fail(t, "the function should not be called for a non-existing record")
		return record
	})
	require.Error(t, err)
	require.Nil(t, record)
}

// TestConcurrentAdjust tests if the cache can be adjusted concurrently. It adjusts the cache
// with a number of records concurrently and then checks if the cache can retrieve all records.
func TestConcurrentAdjust(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector())

	// defines the number of records to update.
	numRecords := 100

	// adds all records to the cache.
	for i := 0; i < numRecords; i++ {
		peerID := fmt.Sprintf("peer%d", i)
		err := cache.Add(peer.ID(peerID), netcache.GossipSubSpamRecord{
			Decay:   0.1 * float64(i),
			Penalty: float64(i),
		})
		require.True(t, err)
	}

	// uses a wait group to wait for all goroutines to finish.
	var wg sync.WaitGroup
	wg.Add(numRecords)

	// Adjust the records concurrently.
	for i := 0; i < numRecords; i++ {
		go func(num int) {
			defer wg.Done()
			peerID := fmt.Sprintf("peer%d", num)
			_, err := cache.Adjust(peer.ID(peerID), func(record netcache.GossipSubSpamRecord) netcache.GossipSubSpamRecord {
				record.Penalty = 0.7 * float64(num)
				record.Decay = 0.1 * float64(num)
				return record
			})
			require.NoError(t, err)
		}(i)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not adjust all records concurrently on time")

	// checks if the cache can retrieve all records.
	for i := 0; i < numRecords; i++ {
		peerID := fmt.Sprintf("peer%d", i)
		record, err, found := cache.Get(peer.ID(peerID))
		require.True(t, found)
		require.NoError(t, err)

		expectedScore := 0.7 * float64(i)
		require.Equal(t, expectedScore, record.Penalty,
			"Get() returned incorrect Penalty for record %s: expected %f, got %f", peerID, expectedScore, record.Penalty)
		expectedDecay := 0.1 * float64(i)
		require.Equal(t, expectedDecay, record.Decay,
			"Get() returned incorrect Decay for record %s: expected %f, got %f", peerID, expectedDecay, record.Decay)
	}
}

// TestAdjustWithPreprocess tests the AdjustAndPreprocess method of the GossipSubSpamRecordCache. It tests
// when the cache has preprocessor functions, all preprocessor functions are called after
// the adjustment function is called.
// Also, it tests if the pre-processor functions are called in the order they are added.
func TestAdjustWithPreprocess(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			record.Penalty += 1.5
			return record, nil
		}, func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			record.Penalty *= 2
			return record, nil
		})

	peerID := "peer1"
	// adds a record to the cache.
	require.True(t, cache.Add(peer.ID(peerID), netcache.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// tests adjusting the score of an existing record.
	record, err := cache.Adjust(peer.ID(peerID), func(record netcache.GossipSubSpamRecord) netcache.GossipSubSpamRecord {
		record.Penalty = 0.7
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 4.4, record.Penalty) // (0.7 + 1.5) * 2 = 4.4
	require.Equal(t, 0.1, record.Decay)   // checks if the decay is not changed.
}

// TestAdjustWithPreprocessError tests the AdjustAndPreprocess method of the GossipSubSpamRecordCache.
// It tests if any of the preprocessor functions returns an error, the adjustment function effect
// is reverted, and the error is returned.
func TestAdjustWithPreprocessError(t *testing.T) {
	secondPreprocessorCalled := false
	cache := netcache.NewGossipSubSpamRecordCache(200,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		// the first preprocessor function adds 1.5 to the score.
		func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			return record, nil
		},
		// the second preprocessor function returns an error on the first call.
		func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			if !secondPreprocessorCalled {
				secondPreprocessorCalled = true
				return record, fmt.Errorf("error")
			}
			return record, nil
		})

	peerID := "peer1"
	// adds a record to the cache.
	require.True(t, cache.Add(peer.ID(peerID), netcache.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// tests adjusting the score of an existing record.
	record, err := cache.Adjust(peer.ID(peerID), func(record netcache.GossipSubSpamRecord) netcache.GossipSubSpamRecord {
		record.Penalty = 0.7
		return record
	})
	// since the second preprocessor function returns an error, the adjustment function effect should be reverted.
	// the error should be returned.
	require.Error(t, err)
	require.Nil(t, record)

	// checks if the record is not changed.
	record, err, found := cache.Get(peer.ID(peerID))
	require.True(t, found)
	require.NoError(t, err)
	require.Equal(t, 0.5, record.Penalty)
}

// TestAppScoreRecordStoredByValue tests if the cache stores the GossipSubSpamRecord by value.
// It updates the cache with a record and then modifies the record. It then checks if the
// record in the cache is still the original record. This is a desired behavior that
// is guaranteed by the HeroCache library. In other words, we don't desire the records to be
// externally mutable after they are added to the cache (unless by a subsequent call to Update).
func TestAppScoreRecordStoredByValue(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector())

	peerID := "peer1"
	added := cache.Add(peer.ID(peerID), netcache.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	})
	require.True(t, added)

	// get the record from the cache
	record, err, found := cache.Get(peer.ID(peerID))
	require.True(t, found)
	require.NoError(t, err)

	// modify the record
	record.Decay = 0.2
	record.Penalty = 0.8

	// get the record from the cache again
	record, err, found = cache.Get(peer.ID(peerID))
	require.True(t, found)
	require.NoError(t, err)

	// check if the record is still the same
	require.Equal(t, 0.1, record.Decay)
	require.Equal(t, 0.5, record.Penalty)
}

// TestAppScoreCache_Get_WithPreprocessors tests if the cache applies the preprocessors to the records
// before returning them. It adds a record to the cache and then checks if the preprocessors were
// applied to the record. It also checks if the preprocessors were applied in the correct order.
// The first preprocessor adds 1 to the score and the second preprocessor multiplies the score by 2.
// Therefore, the expected score is 4.
// Note that the preprocessors are applied in the order they are passed to the cache.
func TestAppScoreCache_Get_WithPreprocessors(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector(),
		// first preprocessor: adds 1 to the score.
		func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			record.Penalty++
			return record, nil
		},
		// second preprocessor: multiplies the score by 2
		func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			record.Penalty *= 2
			return record, nil
		},
	)

	record := netcache.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}
	added := cache.Add("peerA", record)
	assert.True(t, added)

	// verifies that the preprocessors were called and the score was updated accordingly.
	cachedRecord, err, ok := cache.Get("peerA")
	assert.NoError(t, err)
	assert.True(t, ok)

	// expected score is 4: the first preprocessor adds 1 to the score and the second preprocessor multiplies the score by 2.
	// (1 + 1) * 2 = 4
	assert.Equal(t, 4.0, cachedRecord.Penalty) // score should be updated
	assert.Equal(t, 0.5, cachedRecord.Decay)   // decay should not be modified
}

// TestAppScoreCache_Update_PreprocessingError tests if the cache returns an error if one of the preprocessors returns an error.
// It adds a record to the cache and then checks if the cache returns an error if one of the preprocessors returns an error.
// It also checks if a preprocessor is failed, the subsequent preprocessors are not called, and the original record is returned.
// In other words, the Get method acts atomically on the record for applying the preprocessors. If one of the preprocessors
// fails, the record is returned without applying the subsequent preprocessors.
func TestAppScoreCache_Update_PreprocessingError(t *testing.T) {
	secondPreprocessorCalledCount := 0
	thirdPreprocessorCalledCount := 0

	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector(),
		// first preprocessor: adds 1 to the score.
		func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			record.Penalty++
			return record, nil
		},
		// second preprocessor: multiplies the score by 2 (this preprocessor returns an error on the second call)
		func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			secondPreprocessorCalledCount++
			if secondPreprocessorCalledCount < 2 {
				// on the first call, the preprocessor is successful
				return record, nil
			} else {
				// on the second call, the preprocessor returns an error
				return netcache.GossipSubSpamRecord{}, fmt.Errorf("error in preprocessor")
			}
		},
		// since second preprocessor returns an error on the second call, the third preprocessor should not be called more than once..
		func(record netcache.GossipSubSpamRecord, lastUpdated time.Time) (netcache.GossipSubSpamRecord, error) {
			thirdPreprocessorCalledCount++
			require.Less(t, secondPreprocessorCalledCount, 2)
			return record, nil
		},
	)

	record := netcache.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}
	added := cache.Add("peerA", record)
	assert.True(t, added)

	// verifies that the preprocessors were called and the score was updated accordingly.
	cachedRecord, err, ok := cache.Get("peerA")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 2.0, cachedRecord.Penalty) // score should be updated by the first preprocessor (1 + 1 = 2)
	assert.Equal(t, 0.5, cachedRecord.Decay)

	// query the cache again that should trigger the second preprocessor to return an error.
	cachedRecord, err, ok = cache.Get("peerA")
	require.Error(t, err)
	assert.False(t, ok)
	assert.Nil(t, cachedRecord)

	// verifies that the third preprocessor was not called.
	assert.Equal(t, 1, thirdPreprocessorCalledCount)
	// verifies that the second preprocessor was called only twice (one success, and one failure).
	assert.Equal(t, 2, secondPreprocessorCalledCount)
}

// TestAppScoreCache_Get_WithNoPreprocessors tests when no preprocessors are provided to the cache constructor
// that the cache returns the original record without any modifications.
func TestAppScoreCache_Get_WithNoPreprocessors(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector())

	record := netcache.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}
	added := cache.Add("peerA", record)
	assert.True(t, added)

	// verifies that no preprocessors were called and the score was not updated.
	cachedRecord, err, ok := cache.Get("peerA")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1.0, cachedRecord.Penalty)
	assert.Equal(t, 0.5, cachedRecord.Decay)
}

// TestAppScoreCache_DuplicateAdd_Sequential tests if the cache returns false when a duplicate record is added to the cache.
// This test evaluates that the cache deduplicates the records based on their peer id and not content, and hence
// each peer id can only be added once to the cache. We use this feature to check if a peer is already in the cache, and
// if not initializing its record.
func TestAppScoreCache_DuplicateAdd_Sequential(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector())

	record := netcache.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}
	added := cache.Add("peerA", record)
	assert.True(t, added)

	// verifies that the cache returns false when a duplicate record is added.
	added = cache.Add("peerA", record)
	assert.False(t, added)

	// verifies that the cache deduplicates the records based on their peer id and not content.
	record.Penalty = 2
	added = cache.Add("peerA", record)
	assert.False(t, added)
}

// TestAppScoreCache_DuplicateAdd_Concurrent tests if the cache returns false when a duplicate record is added to the cache.
// Test is the concurrent version of TestAppScoreCache_DuplicateAdd_Sequential.
func TestAppScoreCache_DuplicateAdd_Concurrent(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector())

	successAdd := atomic.Int32{}
	successAdd.Store(0)

	record1 := netcache.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}

	record2 := netcache.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 2,
	}

	wg := sync.WaitGroup{} // wait group to wait for all goroutines to finish.
	wg.Add(2)
	// adds a record to the cache concurrently.
	add := func(record netcache.GossipSubSpamRecord) {
		added := cache.Add("peerA", record)
		if added {
			successAdd.Inc()
		}
		wg.Done()
	}

	go add(record1)
	go add(record2)

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not add records to the cache")

	// verifies that only one of the records was added to the cache.
	assert.Equal(t, int32(1), successAdd.Load())
}
