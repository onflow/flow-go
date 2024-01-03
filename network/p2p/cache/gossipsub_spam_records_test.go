package cache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubSpamRecordCache_Add tests the Add method of the GossipSubSpamRecordCache. It tests
// adding a new record to the cache.
func TestGossipSubSpamRecordCache_Add(t *testing.T) {
	// create a new instance of GossipSubSpamRecordCache.
	cache := netcache.NewGossipSubSpamRecordCache(100, unittest.Logger(), metrics.NewNoopCollector(), func() p2p.GossipSubSpamRecord {
		return p2p.GossipSubSpamRecord{
			Decay:               0,
			Penalty:             0,
			LastDecayAdjustment: time.Now(),
		}
	})

	adjustedEntity, err := cache.Adjust("peer0", func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Decay = 0.1
		record.Penalty = 0.5

		return record
	})
	require.NoError(t, err)
	require.Equal(t, 0.1, adjustedEntity.Decay)
	require.Equal(t, 0.5, adjustedEntity.Penalty)

	// makes the cache full.
	for i := 1; i <= 100; i++ {
		adjustedEntity, err := cache.Adjust(peer.ID(fmt.Sprintf("peer%d", i)), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
			record.Decay = 0.1
			record.Penalty = 0.5

			return record
		})

		require.NoError(t, err)
		require.Equal(t, 0.1, adjustedEntity.Decay)
	}

	// retrieving an existing record should work.
	for i := 1; i <= 100; i++ {
		record, err, ok := cache.Get(peer.ID(fmt.Sprintf("peer%d", i)))
		require.True(t, ok, fmt.Sprintf("record for peer%d should exist", i))
		require.NoError(t, err)

		require.Equal(t, 0.1, record.Decay)
		require.Equal(t, 0.5, record.Penalty)
	}

	// since cache is LRU, the first record should be evicted.
	_, err, ok := cache.Get("peer0")
	require.False(t, ok)
	require.NoError(t, err)
}

// TestGossipSubSpamRecordCache_Concurrent_Adjust tests if the cache can be adjusted and retrieved concurrently.
// It updates the cache with a number of records concurrently and then checks if the cache
// can retrieve all records.
func TestGossipSubSpamRecordCache_Concurrent_Adjust(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector(), func() p2p.GossipSubSpamRecord {
		return p2p.GossipSubSpamRecord{
			Decay:               0,
			Penalty:             0,
			LastDecayAdjustment: time.Now(),
		}
	})

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
			adjustedEntity, err := cache.Adjust(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
				record.Decay = 0.1 * float64(num)
				record.Penalty = float64(num)

				return record
			})

			require.NoError(t, err)
			require.Equal(t, 0.1*float64(num), adjustedEntity.Decay)
			require.Equal(t, float64(num), adjustedEntity.Penalty)
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

// TestGossipSubSpamRecordCache_Adjust tests the Update method of the GossipSubSpamRecordCache. It tests if the cache can adjust
// the penalty of an existing record and add a new record.
func TestGossipSubSpamRecordCache_Adjust(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector(), func() p2p.GossipSubSpamRecord {
		return p2p.GossipSubSpamRecord{
			Decay:               0,
			Penalty:             0,
			LastDecayAdjustment: time.Now(),
		}
	})

	peerID := "peer1"

	// test adjusting a non-existing record.
	record, err := cache.Adjust(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 0.7
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 0.7, record.Penalty) // checks if the penalty is updateed correctly.

	// test adjusting an existing record.
	record, err = cache.Adjust(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 0.8
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 0.8, record.Penalty) // checks if the penalty is updateed correctly.
}

// TestGossipSubSpamRecordCache_Adjust_With_Preprocess tests Update method of the GossipSubSpamRecordCache when the cache
// has preprocessor functions.
// It tests when the cache has preprocessor functions, all preprocessor functions are called prior to the update function.
// Also, it tests if the pre-processor functions are called in the order they are added.
func TestGossipSubSpamRecordCache_Adjust_With_Preprocess(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(200,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:               0,
				Penalty:             0,
				LastDecayAdjustment: time.Now(),
			}
		},
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			record.Penalty += 1.5
			return record, nil
		}, func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			record.Penalty *= 2
			return record, nil
		})

	peerID := "peer1"

	// test adjusting a non-existing record.
	record, err := cache.Adjust(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 0.5
		record.Decay = 0.1
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 0.5, record.Penalty) // checks if the penalty is updateed correctly.
	require.Equal(t, 0.1, record.Decay)   // checks if the decay is updateed correctly.

	// tests updating the penalty of an existing record.
	record, err = cache.Adjust(peer.ID(peerID), func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty += 0.7
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 4.7, record.Penalty) // (0.5+1.5) * 2 + 0.7 = 4.7
	require.Equal(t, 0.1, record.Decay)   // checks if the decay is not changed.
}

// TestGossipSubSpamRecordCache_Adjust_Preprocess_Error tests the Adjust method of the GossipSubSpamRecordCache.
// It tests if any of the preprocessor functions returns an error, the Adjust function effect
// is reverted, and the error is returned.
func TestGossipSubSpamRecordCache_Adjust_Preprocess_Error(t *testing.T) {
	secondPreprocessorCalled := 0
	cache := netcache.NewGossipSubSpamRecordCache(200,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:               0,
				Penalty:             0,
				LastDecayAdjustment: time.Now(),
			}
		},
		// the first preprocessor function does not return an error.
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			return record, nil
		},
		// the second preprocessor function returns an error on the second call, and does not return an error on any other call.
		// this means that adjustment should be successful on the first call, and should fail on the second call.
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			secondPreprocessorCalled++
			if secondPreprocessorCalled == 2 {
				return record, fmt.Errorf("some error")
			}
			return record, nil

		})

	peerID := unittest.PeerIdFixture(t)

	// tests adjusting the penalty of a non-existing record; the record should be initiated and the penalty should be updated.
	record, err := cache.Adjust(peerID, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 0.5
		record.Decay = 0.1
		return record
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, 0.5, record.Penalty) // checks if the penalty is not changed.
	require.Equal(t, 0.1, record.Decay)   // checks if the decay is not changed.

	// tests adjusting the penalty of an existing record.
	record, err = cache.Adjust(peerID, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 0.7
		return record
	})
	// since the second preprocessor function returns an error, the update function effect should be reverted.
	// the error should be returned.
	require.Error(t, err)
	require.Nil(t, record)

	// checks if the record is not changed.
	record, err, found := cache.Get(peerID)
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
	cache := netcache.NewGossipSubSpamRecordCache(200, unittest.Logger(), metrics.NewNoopCollector(), func() p2p.GossipSubSpamRecord {
		return p2p.GossipSubSpamRecord{
			Decay:               0,
			Penalty:             0,
			LastDecayAdjustment: time.Now(),
		}
	})

	peerID := unittest.PeerIdFixture(t)
	// adjusts a non-existing record, which should initiate the record.
	record, err := cache.Adjust(peerID, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 0.5
		record.Decay = 0.1
		return record
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, 0.5, record.Penalty) // checks if the penalty is not changed.
	require.Equal(t, 0.1, record.Decay)   // checks if the decay is not changed.

	// get the record from the cache
	record, err, found := cache.Get(peerID)
	require.True(t, found)
	require.NoError(t, err)

	// modify the record
	record.Decay = 0.2
	record.Penalty = 0.8

	// get the record from the cache again
	record, err, found = cache.Get(peerID)
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
		func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:               0,
				Penalty:             0,
				LastDecayAdjustment: time.Now(),
			}
		},
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

	peerId := unittest.PeerIdFixture(t)
	adjustedRecord, err := cache.Adjust(peerId, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 1
		record.Decay = 0.5
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 1.0, adjustedRecord.Penalty)

	// verifies that the preprocessors were called and the record was updated accordingly.
	cachedRecord, err, ok := cache.Get(peerId)
	require.NoError(t, err)
	require.True(t, ok)

	// expected penalty is 4: the first preprocessor adds 1 to the penalty and the second preprocessor multiplies the penalty by 2.
	// (1 + 1) * 2 = 4
	require.Equal(t, 4.0, cachedRecord.Penalty) // penalty should be updated
	require.Equal(t, 0.5, cachedRecord.Decay)   // decay should not be modified
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
		func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:               0,
				Penalty:             0,
				LastDecayAdjustment: time.Now(),
			}
		},
		// first preprocessor: adds 1 to the penalty.
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			record.Penalty++
			return record, nil
		},
		// second preprocessor: multiplies the penalty by 2 (this preprocessor returns an error on the third call and forward)
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			secondPreprocessorCalledCount++
			if secondPreprocessorCalledCount < 3 {
				// on the first call, the preprocessor is successful
				return record, nil
			} else {
				// on the second call, the preprocessor returns an error
				return p2p.GossipSubSpamRecord{}, fmt.Errorf("error in preprocessor")
			}
		},
		// since second preprocessor returns an error on the second call, the third preprocessor should not be called more than once.
		func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
			thirdPreprocessorCalledCount++
			require.Less(t, secondPreprocessorCalledCount, 3)
			return record, nil
		},
	)

	peerId := unittest.PeerIdFixture(t)
	adjustedRecord, err := cache.Adjust(peerId, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 1
		record.Decay = 0.5
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 1.0, adjustedRecord.Penalty)
	require.Equal(t, 0.5, adjustedRecord.Decay)

	// verifies that the preprocessors were called and the penalty was updated accordingly.
	cachedRecord, err, ok := cache.Get(peerId)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 2.0, cachedRecord.Penalty) // penalty should be updated by the first preprocessor (1 + 1 = 2)
	require.Equal(t, 0.5, cachedRecord.Decay)

	// query the cache again that should trigger the second preprocessor to return an error.
	cachedRecord, err, ok = cache.Get(peerId)
	require.Error(t, err)
	require.False(t, ok)
	require.Nil(t, cachedRecord)

	// verifies that the third preprocessor was called only twice (two success calls).
	require.Equal(t, 2, thirdPreprocessorCalledCount)
	// verifies that the second preprocessor was called three times (two success calls and one failure call).
	require.Equal(t, 3, secondPreprocessorCalledCount)
}

// TestGossipSubSpamRecordCache_Get_Without_Preprocessors tests when no preprocessors are provided to the cache constructor
// that the cache returns the original record without any modifications.
func TestGossipSubSpamRecordCache_Get_Without_Preprocessors(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector(), func() p2p.GossipSubSpamRecord {
		return p2p.GossipSubSpamRecord{
			Decay:               0,
			Penalty:             0,
			LastDecayAdjustment: time.Now(),
		}
	})

	peerId := unittest.PeerIdFixture(t)
	adjustedRecord, err := cache.Adjust(peerId, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 1
		record.Decay = 0.5
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 1.0, adjustedRecord.Penalty)
	require.Equal(t, 0.5, adjustedRecord.Decay)

	// verifies that no preprocessors were called and the record was not updated.
	cachedRecord, err, ok := cache.Get(peerId)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1.0, cachedRecord.Penalty)
	require.Equal(t, 0.5, cachedRecord.Decay)
}

// TestGossipSubSpamRecordCache_Duplicate_Add_Sequential tests if the cache returns false when a duplicate record is added to the cache.
// This test evaluates that the cache de-duplicates the records based on their peer id and not content, and hence
// each peer id can only be added once to the cache.
func TestGossipSubSpamRecordCache_Duplicate_Adjust_Sequential(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector(), func() p2p.GossipSubSpamRecord {
		return p2p.GossipSubSpamRecord{
			Decay:               0,
			Penalty:             0,
			LastDecayAdjustment: time.Now(),
		}
	})

	peerId := unittest.PeerIdFixture(t)
	adjustedRecord, err := cache.Adjust(peerId, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 1
		record.Decay = 0.5
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 1.0, adjustedRecord.Penalty)
	require.Equal(t, 0.5, adjustedRecord.Decay)

	// duplicate adjust should return the same record.
	adjustedRecord, err = cache.Adjust(peerId, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 1
		record.Decay = 0.5
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 1.0, adjustedRecord.Penalty)
	require.Equal(t, 0.5, adjustedRecord.Decay)

	// verifies that the cache deduplicates the records based on their peer id and not content.
	adjustedRecord, err = cache.Adjust(peerId, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = 3
		record.Decay = 2
		return record
	})
	require.NoError(t, err)
	require.Equal(t, 3.0, adjustedRecord.Penalty)
	require.Equal(t, 2.0, adjustedRecord.Decay)
}

// // TestGossipSubSpamRecordCache_Duplicate_Add_Concurrent tests if the cache returns false when a duplicate record is added to the cache.
// // Test is the concurrent version of TestAppScoreCache_DuplicateAdd_Sequential.
// func TestGossipSubSpamRecordCache_Duplicate_Add_Concurrent(t *testing.T) {
// 	cache := netcache.NewGossipSubSpamRecordCache(10, unittest.Logger(), metrics.NewNoopCollector())
//
// 	successAdd := atomic.Int32{}
// 	successAdd.Store(0)
//
// 	record1 := p2p.GossipSubSpamRecord{
// 		Decay:   0.5,
// 		Penalty: 1,
// 	}
//
// 	record2 := p2p.GossipSubSpamRecord{
// 		Decay:   0.5,
// 		Penalty: 2,
// 	}
//
// 	wg := sync.WaitGroup{} // wait group to wait for all goroutines to finish.
// 	wg.Add(2)
// 	// adds a record to the cache concurrently.
// 	add := func(record p2p.GossipSubSpamRecord) {
// 		added := cache.Add("peerA", record)
// 		if added {
// 			successAdd.Inc()
// 		}
// 		wg.Done()
// 	}
//
// 	go add(record1)
// 	go add(record2)
//
// 	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "could not add records to the cache")
//
// 	// verifies that only one of the records was added to the cache.
// 	assert.Equal(t, int32(1), successAdd.Load())
// }
