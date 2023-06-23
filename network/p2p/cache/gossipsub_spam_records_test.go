package cache_test

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
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubSpamRecordCache_Add tests the Add method of the GossipSubSpamRecordCache. It tests
// adding a new record to the cache.
func TestGossipSubSpamRecordCache_Add(t *testing.T) {
	// create a new instance of GossipSubSpamRecordCache.
	cache := netcache.NewGossipSubSpamRecordCache(100, unittest.Logger(), metrics.NewNoopCollector())

	// tests adding a new record to the cache.
	require.True(t, cache.Add("peer0", p2p.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// tests updating an existing record in the cache.
	require.False(t, cache.Add("peer0", p2p.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// makes the cache full.
	for i := 1; i < 100; i++ {
		require.True(t, cache.Add(peer.ID(fmt.Sprintf("peer%d", i)), p2p.GossipSubSpamRecord{
			Decay:   0.1,
			Penalty: 0.5,
		}))
	}

	// adding a new record to the cache should fail.
	require.False(t, cache.Add("peer101", p2p.GossipSubSpamRecord{
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
	require.False(t, cache.Add("peer1", p2p.GossipSubSpamRecord{
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
			added := cache.Add(peer.ID(peerID), p2p.GossipSubSpamRecord{
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

		expectedPenalty := float64(i)
		require.Equal(t, expectedPenalty, record.Penalty,
			"Get() returned incorrect penalty for record %s: expected %f, got %f", peerID, expectedPenalty, record.Penalty)
		expectedDecay := 0.1 * float64(i)
		require.Equal(t, expectedDecay, record.Decay,
			"Get() returned incorrect decay for record %s: expected %f, got %f", peerID, expectedDecay, record.Decay)
	}
}

// TestGossipSubSpamRecordCache_Update tests the Update method of the GossipSubSpamRecordCache. It tests if the cache can update
// the penalty of an existing record and fail to update the penalty of a non-existing record.
func TestGossipSubSpamRecordCache_Update(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector())

	peerID := "peer1"

	// tests updateing the penalty of an existing record.
	require.True(t, cache.Add(peer.ID(peerID), p2p.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))
	record, err := cache.Update(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 0.7
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 0.7, record.Penalty) // checks if the penalty is updateed correctly.

	// tests updating the penalty of a non-existing record.
	record, err = cache.Update(peer.ID("peer2"), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		require.Fail(t, "the function should not be called for a non-existing record")
		return record
	})
	require.Error(t, err)
	require.Nil(t, record)
}

// TestGossipSubSpamRecordCache_Concurrent_Update tests if the cache can be updated concurrently. It updates the cache
// with a number of records concurrently and then checks if the cache can retrieve all records.
func TestGossipSubSpamRecordCache_Concurrent_Update(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector())

	// defines the number of records to update.
	numRecords := 100

	// adds all records to the cache, sequentially.
	for i := 0; i < numRecords; i++ {
		peerID := fmt.Sprintf("peer%d", i)
		err := cache.Add(peer.ID(peerID), p2p.GossipSubSpamRecord{
			Decay:   0.1 * float64(i),
			Penalty: float64(i),
		})
		require.True(t, err)
	}

	// uses a wait group to wait for all goroutines to finish.
	var wg sync.WaitGroup
	wg.Add(numRecords)

	// updates the records concurrently.
	for i := 0; i < numRecords; i++ {
		go func(num int) {
			defer wg.Done()
			peerID := fmt.Sprintf("peer%d", num)
			_, err := cache.Update(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
				record.Penalty = 0.7 * float64(num)
				record.Decay = 0.1 * float64(num)
				return record
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

		expectedPenalty := 0.7 * float64(i)
		require.Equal(t, expectedPenalty, record.Penalty,
			"Get() returned incorrect Penalty for record %s: expected %f, got %f", peerID, expectedPenalty, record.Penalty)
		expectedDecay := 0.1 * float64(i)
		require.Equal(t, expectedDecay, record.Decay,
			"Get() returned incorrect Decay for record %s: expected %f, got %f", peerID, expectedDecay, record.Decay)
	}
}

// TestGossipSubSpamRecordCache_Update_With_Preprocess tests Update method of the GossipSubSpamRecordCache when the cache
// has preprocessor functions.
// It tests when the cache has preprocessor functions, all preprocessor functions are called prior to the update function.
// Also, it tests if the pre-processor functions are called in the order they are added.
func TestGossipSubSpamRecordCache_Update_With_Preprocess(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			record.Penalty += 1.5
			return record, nil
		}, func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			record.Penalty *= 2
			return record, nil
		})

	peerID := "peer1"
	// adds a record to the cache.
	require.True(t, cache.Add(peer.ID(peerID), p2p.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// tests updating the penalty of an existing record.
	record, err := cache.Update(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty += 0.7
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 4.7, record.Penalty) // (0.5+1.5) * 2 + 0.7 = 4.7
	require.Equal(t, 0.1, record.Decay)   // checks if the decay is not changed.
}

// TestGossipSubSpamRecordCache_Update_Preprocess_Error tests the Update method of the GossipSubSpamRecordCache.
// It tests if any of the preprocessor functions returns an error, the update function effect
// is reverted, and the error is returned.
func TestGossipSubSpamRecordCache_Update_Preprocess_Error(t *testing.T) {
	secondPreprocessorCalled := false
	cache := netcache.NewGossipSubSpamRecordCache(200,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		// the first preprocessor function does not return an error.
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			return record, nil
		},
		// the second preprocessor function returns an error on the first call and nil on the second call onwards.
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			if !secondPreprocessorCalled {
				secondPreprocessorCalled = true
				return record, fmt.Errorf("error")
			}
			return record, nil
		})

	peerID := "peer1"
	// adds a record to the cache.
	require.True(t, cache.Add(peer.ID(peerID), p2p.GossipSubSpamRecord{
		Decay:   0.1,
		Penalty: 0.5,
	}))

	// tests updating the penalty of an existing record.
	record, err := cache.Update(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 0.7
		return record
	})
	// since the second preprocessor function returns an error, the update function effect should be reverted.
	// the error should be returned.
	require.Error(t, err)
	require.Nil(t, record)

	// checks if the record is not changed.
	record, err, found := cache.Get(peer.ID(peerID))
	require.True(t, found)
	require.NoError(t, err)
	require.Equal(t, 0.5, record.Penalty) // checks if the penalty is not changed.
	require.Equal(t, 0.1, record.Decay)   // checks if the decay is not changed.
}

// TestGossipSubSpamRecordCache_ByValue tests if the cache stores the GossipSubSpamRecord by value.
// It updates the cache with a record and then modifies the record externally.
// It then checks if the record in the cache is still the original record.
// This is a desired behavior that is guaranteed by the underlying HeroCache library.
// In other words, we don't desire the records to be externally mutable after they are added to the cache (unless by a subsequent call to Update).
func TestGossipSubSpamRecordCache_ByValue(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector())

	peerID := "peer1"
	added := cache.Add(peer.ID(peerID), p2p.GossipSubSpamRecord{
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

// TestGossipSubSpamRecordCache_Get_With_Preprocessors tests if the cache applies the preprocessors to the records
// before returning them.
func TestGossipSubSpamRecordCache_Get_With_Preprocessors(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector(),
		// first preprocessor: adds 1 to the penalty.
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			record.Penalty++
			return record, nil
		},
		// second preprocessor: multiplies the penalty by 2
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			record.Penalty *= 2
			return record, nil
		},
	)

	record := p2p.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}
	added := cache.Add("peerA", record)
	assert.True(t, added)

	// verifies that the preprocessors were called and the record was updated accordingly.
	cachedRecord, err, ok := cache.Get("peerA")
	assert.NoError(t, err)
	assert.True(t, ok)

	// expected penalty is 4: the first preprocessor adds 1 to the penalty and the second preprocessor multiplies the penalty by 2.
	// (1 + 1) * 2 = 4
	assert.Equal(t, 4.0, cachedRecord.Penalty) // penalty should be updated
	assert.Equal(t, 0.5, cachedRecord.Decay)   // decay should not be modified
}

// TestGossipSubSpamRecordCache_Get_Preprocessor_Error tests if the cache returns an error if one of the preprocessors returns an error upon a Get.
// It adds a record to the cache and then checks if the cache returns an error upon a Get if one of the preprocessors returns an error.
// It also checks if a preprocessor is failed, the subsequent preprocessors are not called, and the original record is returned.
// In other words, the Get method acts atomically on the record for applying the preprocessors. If one of the preprocessors
// fails, the record is returned without applying the subsequent preprocessors.
func TestGossipSubSpamRecordCache_Get_Preprocessor_Error(t *testing.T) {
	secondPreprocessorCalledCount := 0
	thirdPreprocessorCalledCount := 0

	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector(),
		// first preprocessor: adds 1 to the penalty.
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			record.Penalty++
			return record, nil
		},
		// second preprocessor: multiplies the penalty by 2 (this preprocessor returns an error on the second call)
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			secondPreprocessorCalledCount++
			if secondPreprocessorCalledCount < 2 {
				// on the first call, the preprocessor is successful
				return record, nil
			} else {
				// on the second call, the preprocessor returns an error
				return p2p.GossipSubSpamRecord{}, fmt.Errorf("error in preprocessor")
			}
		},
		// since second preprocessor returns an error on the second call, the third preprocessor should not be called more than once..
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			thirdPreprocessorCalledCount++
			require.Less(t, secondPreprocessorCalledCount, 2)
			return record, nil
		},
	)

	record := p2p.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}
	added := cache.Add("peerA", record)
	assert.True(t, added)

	// verifies that the preprocessors were called and the penalty was updated accordingly.
	cachedRecord, err, ok := cache.Get("peerA")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 2.0, cachedRecord.Penalty) // penalty should be updated by the first preprocessor (1 + 1 = 2)
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

// TestGossipSubSpamRecordCache_Get_Without_Preprocessors tests when no preprocessors are provided to the cache constructor
// that the cache returns the original record without any modifications.
func TestGossipSubSpamRecordCache_Get_Without_Preprocessors(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector())

	record := p2p.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}
	added := cache.Add("peerA", record)
	assert.True(t, added)

	// verifies that no preprocessors were called and the record was not updated.
	cachedRecord, err, ok := cache.Get("peerA")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1.0, cachedRecord.Penalty)
	assert.Equal(t, 0.5, cachedRecord.Decay)
}

// TestGossipSubSpamRecordCache_Duplicate_Add_Sequential tests if the cache returns false when a duplicate record is added to the cache.
// This test evaluates that the cache de-duplicates the records based on their peer id and not content, and hence
// each peer id can only be added once to the cache.
func TestGossipSubSpamRecordCache_Duplicate_Add_Sequential(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector())

	record := p2p.GossipSubSpamRecord{
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

// TestGossipSubSpamRecordCache_Duplicate_Add_Concurrent tests if the cache returns false when a duplicate record is added to the cache.
// Test is the concurrent version of TestAppScoreCache_DuplicateAdd_Sequential.
func TestGossipSubSpamRecordCache_Duplicate_Add_Concurrent(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector())

	successAdd := atomic.Int32{}
	successAdd.Store(0)

	record1 := p2p.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 1,
	}

	record2 := p2p.GossipSubSpamRecord{
		Decay:   0.5,
		Penalty: 2,
	}

	wg := sync.WaitGroup{} // wait group to wait for all goroutines to finish.
	wg.Add(2)
	// adds a record to the cache concurrently.
	add := func(record p2p.GossipSubSpamRecord) {
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
