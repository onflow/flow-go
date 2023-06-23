package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

const defaultDecay = 0.99

// TestRecordCache_Init tests the Init method of the RecordCache.
// It ensures that the method returns true when a new record is initialized
// and false when an existing record is initialized.
func TestRecordCache_Init(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeID1 := unittest.IdentifierFixture()
	nodeID2 := unittest.IdentifierFixture()

	// test initializing a record for an node ID that doesn't exist in the cache
	initialized := cache.Init(nodeID1)
	require.True(t, initialized, "expected record to be initialized")
	gauge, ok, err := cache.Get(nodeID1)
	require.NoError(t, err)
	require.True(t, ok, "expected record to exist")
	require.Zerof(t, gauge, "expected gauge to be 0")
	require.Equal(t, uint(1), cache.Size(), "expected cache to have one additional record")

	// test initializing a record for an node ID that already exists in the cache
	initialized = cache.Init(nodeID1)
	require.False(t, initialized, "expected record not to be initialized")
	gaugeAgain, ok, err := cache.Get(nodeID1)
	require.NoError(t, err)
	require.True(t, ok, "expected record to still exist")
	require.Zerof(t, gaugeAgain, "expected same gauge to be 0")
	require.Equal(t, gauge, gaugeAgain, "expected records to be the same")
	require.Equal(t, uint(1), cache.Size(), "expected cache to still have one additional record")

	// test initializing a record for another node ID
	initialized = cache.Init(nodeID2)
	require.True(t, initialized, "expected record to be initialized")
	gauge2, ok, err := cache.Get(nodeID2)
	require.NoError(t, err)
	require.True(t, ok, "expected record to exist")
	require.Zerof(t, gauge2, "expected second gauge to be 0")
	require.Equal(t, uint(2), cache.Size(), "expected cache to have two additional records")
}

// TestRecordCache_ConcurrentInit tests the concurrent initialization of records.
// The test covers the following scenarios:
// 1. Multiple goroutines initializing records for different node IDs.
// 2. Ensuring that all records are correctly initialized.
func TestRecordCache_ConcurrentInit(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeIDs := unittest.IdentifierListFixture(10)

	var wg sync.WaitGroup
	wg.Add(len(nodeIDs))

	for _, nodeID := range nodeIDs {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.Init(id)
		}(nodeID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that all records are correctly initialized
	for _, nodeID := range nodeIDs {
		gauge, found, err := cache.Get(nodeID)
		require.NoError(t, err)
		require.True(t, found)
		require.Zerof(t, gauge, "expected all gauge values to be initialized to 0")
	}
}

// TestRecordCache_ConcurrentSameRecordInit tests the concurrent initialization of the same record.
// The test covers the following scenarios:
// 1. Multiple goroutines attempting to initialize the same record concurrently.
// 2. Only one goroutine successfully initializes the record, and others receive false on initialization.
// 3. The record is correctly initialized in the cache and can be retrieved using the Get method.
func TestRecordCache_ConcurrentSameRecordInit(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeID := unittest.IdentifierFixture()
	const concurrentAttempts = 10

	var wg sync.WaitGroup
	wg.Add(concurrentAttempts)

	successGauge := atomic.Int32{}

	for i := 0; i < concurrentAttempts; i++ {
		go func() {
			defer wg.Done()
			initSuccess := cache.Init(nodeID)
			if initSuccess {
				successGauge.Inc()
			}
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that only one goroutine successfully initialized the record
	require.Equal(t, int32(1), successGauge.Load())

	// ensure that the record is correctly initialized in the cache
	gauge, found, err := cache.Get(nodeID)
	require.NoError(t, err)
	require.True(t, found)
	require.Zero(t, gauge)
}

// TestRecordCache_ReceivedClusterPrefixedMessage tests the ReceivedClusterPrefixedMessage method of the RecordCache.
// The test covers the following scenarios:
// 1. Updating a record gauge for an existing node ID.
// 2. Attempting to update a record gauge  for a non-existing node ID should not result in error. ReceivedClusterPrefixedMessage should always attempt to initialize the gauge.
// 3. Multiple updates on the same record only initialize the record once.
func TestRecordCache_ReceivedClusterPrefixedMessage(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeID1 := unittest.IdentifierFixture()
	nodeID2 := unittest.IdentifierFixture()

	// initialize spam records for nodeID1 and nodeID2
	require.True(t, cache.Init(nodeID1))
	require.True(t, cache.Init(nodeID2))

	gauge, err := cache.ReceivedClusterPrefixedMessage(nodeID1)
	require.NoError(t, err)
	require.Equal(t, float64(1), gauge)

	// get will apply a slightl decay resulting
	// in a gauge value less than gauge which is 1 but greater than 0.9
	currentGauge, ok, err := cache.Get(nodeID1)
	require.NoError(t, err)
	require.True(t, ok)
	require.LessOrEqual(t, currentGauge, gauge)
	require.Greater(t, currentGauge, 0.9)

	// test adjusting the spam record for a non-existing node ID
	nodeID3 := unittest.IdentifierFixture()
	gauge3, err := cache.ReceivedClusterPrefixedMessage(nodeID3)
	require.NoError(t, err)
	require.Equal(t, float64(1), gauge3)

	// when updated the value should be incremented from 1 -> 2 and slightly decayed resulting
	// in a gauge value less than 2 but greater than 1.9
	gauge3, err = cache.ReceivedClusterPrefixedMessage(nodeID3)
	require.NoError(t, err)
	require.LessOrEqual(t, gauge3, 2.0)
	require.Greater(t, gauge3, 1.9)
}

// TestRecordCache_UpdateDecay ensures that a gauge in the record cache is eventually decayed back to 0 after some time.
func TestRecordCache_Decay(t *testing.T) {
	cache := cacheFixture(t, 100, 0.09, zerolog.Nop(), metrics.NewNoopCollector())

	nodeID1 := unittest.IdentifierFixture()

	// initialize spam records for nodeID1 and nodeID2
	require.True(t, cache.Init(nodeID1))
	gauge, err := cache.ReceivedClusterPrefixedMessage(nodeID1)
	require.Equal(t, float64(1), gauge)
	require.NoError(t, err)
	gauge, ok, err := cache.Get(nodeID1)
	require.True(t, ok)
	require.NoError(t, err)
	// gauge should have been delayed slightly
	require.True(t, gauge < float64(1))

	time.Sleep(time.Second)

	gauge, ok, err = cache.Get(nodeID1)
	require.True(t, ok)
	require.NoError(t, err)
	// gauge should have been delayed slightly, but closer to 0
	require.Less(t, gauge, 0.1)
}

// TestRecordCache_Identities tests the NodeIDs method of the RecordCache.
// The test covers the following scenarios:
// 1. Initializing the cache with multiple records.
// 2. Checking if the NodeIDs method returns the correct set of node IDs.
func TestRecordCache_Identities(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	// initialize spam records for a few node IDs
	nodeID1 := unittest.IdentifierFixture()
	nodeID2 := unittest.IdentifierFixture()
	nodeID3 := unittest.IdentifierFixture()

	require.True(t, cache.Init(nodeID1))
	require.True(t, cache.Init(nodeID2))
	require.True(t, cache.Init(nodeID3))

	// check if the NodeIDs method returns the correct set of node IDs
	identities := cache.NodeIDs()
	require.Equal(t, 3, len(identities))
	require.ElementsMatch(t, identities, []flow.Identifier{nodeID1, nodeID2, nodeID3})
}

// TestRecordCache_Remove tests the Remove method of the RecordCache.
// The test covers the following scenarios:
// 1. Initializing the cache with multiple records.
// 2. Removing a record and checking if it is removed correctly.
// 3. Ensuring the other records are still in the cache after removal.
// 4. Attempting to remove a non-existent node ID.
func TestRecordCache_Remove(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	// initialize spam records for a few node IDs
	nodeID1 := unittest.IdentifierFixture()
	nodeID2 := unittest.IdentifierFixture()
	nodeID3 := unittest.IdentifierFixture()

	require.True(t, cache.Init(nodeID1))
	require.True(t, cache.Init(nodeID2))
	require.True(t, cache.Init(nodeID3))

	numOfIds := uint(3)
	require.Equal(t, numOfIds, cache.Size(), fmt.Sprintf("expected size of the cache to be %d", numOfIds))
	// remove nodeID1 and check if the record is removed
	require.True(t, cache.Remove(nodeID1))
	require.NotContains(t, nodeID1, cache.NodeIDs())

	// check if the other node IDs are still in the cache
	_, exists, err := cache.Get(nodeID2)
	require.NoError(t, err)
	require.True(t, exists)
	_, exists, err = cache.Get(nodeID3)
	require.NoError(t, err)
	require.True(t, exists)

	// attempt to remove a non-existent node ID
	nodeID4 := unittest.IdentifierFixture()
	require.False(t, cache.Remove(nodeID4))
}

// TestRecordCache_ConcurrentRemove tests the concurrent removal of records for different node IDs.
// The test covers the following scenarios:
// 1. Multiple goroutines removing records for different node IDs concurrently.
// 2. The records are correctly removed from the cache.
func TestRecordCache_ConcurrentRemove(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeIDs := unittest.IdentifierListFixture(10)
	for _, nodeID := range nodeIDs {
		cache.Init(nodeID)
	}

	var wg sync.WaitGroup
	wg.Add(len(nodeIDs))

	for _, nodeID := range nodeIDs {
		go func(id flow.Identifier) {
			defer wg.Done()
			removed := cache.Remove(id)
			require.True(t, removed)
			require.NotContains(t, id, cache.NodeIDs())
		}(nodeID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	require.Equal(t, uint(0), cache.Size())
}

// TestRecordCache_ConcurrentUpdatesAndReads tests the concurrent adjustments and reads of records for different
// node IDs. The test covers the following scenarios:
// 1. Multiple goroutines adjusting records for different node IDs concurrently.
// 2. Multiple goroutines getting records for different node IDs concurrently.
// 3. The adjusted records are correctly updated in the cache.
func TestRecordCache_ConcurrentUpdatesAndReads(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeIDs := unittest.IdentifierListFixture(10)
	for _, nodeID := range nodeIDs {
		cache.Init(nodeID)
	}

	var wg sync.WaitGroup
	wg.Add(len(nodeIDs) * 2)

	for _, nodeID := range nodeIDs {
		// adjust spam records concurrently
		go func(id flow.Identifier) {
			defer wg.Done()
			_, err := cache.ReceivedClusterPrefixedMessage(id)
			require.NoError(t, err)
		}(nodeID)

		// get spam records concurrently
		go func(id flow.Identifier) {
			defer wg.Done()
			_, found, err := cache.Get(id)
			require.NoError(t, err)
			require.True(t, found)
		}(nodeID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that the records are correctly updated in the cache
	for _, nodeID := range nodeIDs {
		gauge, found, err := cache.Get(nodeID)
		require.NoError(t, err)
		require.True(t, found)
		// slight decay will result in 0.9 < gauge < 1
		require.LessOrEqual(t, gauge, 1.0)
		require.Greater(t, gauge, 0.9)
	}
}

// TestRecordCache_ConcurrentInitAndRemove tests the concurrent initialization and removal of records for different
// node IDs. The test covers the following scenarios:
// 1. Multiple goroutines initializing records for different node IDs concurrently.
// 2. Multiple goroutines removing records for different node IDs concurrently.
// 3. The initialized records are correctly added to the cache.
// 4. The removed records are correctly removed from the cache.
func TestRecordCache_ConcurrentInitAndRemove(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeIDs := unittest.IdentifierListFixture(20)
	nodeIDsToAdd := nodeIDs[:10]
	nodeIDsToRemove := nodeIDs[10:]

	for _, nodeID := range nodeIDsToRemove {
		cache.Init(nodeID)
	}

	var wg sync.WaitGroup
	wg.Add(len(nodeIDs))

	// initialize spam records concurrently
	for _, nodeID := range nodeIDsToAdd {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.Init(id)
		}(nodeID)
	}

	// remove spam records concurrently
	for _, nodeID := range nodeIDsToRemove {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.Remove(id)
			require.NotContains(t, id, cache.NodeIDs())
		}(nodeID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that the initialized records are correctly added to the cache
	// and removed records are correctly removed from the cache
	require.ElementsMatch(t, nodeIDsToAdd, cache.NodeIDs())
}

// TestRecordCache_ConcurrentInitRemoveUpdate tests the concurrent initialization, removal, and adjustment of
// records for different node IDs. The test covers the following scenarios:
// 1. Multiple goroutines initializing records for different node IDs concurrently.
// 2. Multiple goroutines removing records for different node IDs concurrently.
// 3. Multiple goroutines adjusting records for different node IDs concurrently.
func TestRecordCache_ConcurrentInitRemoveUpdate(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeIDs := unittest.IdentifierListFixture(30)
	nodeIDsToAdd := nodeIDs[:10]
	nodeIDsToRemove := nodeIDs[10:20]
	nodeIDsToAdjust := nodeIDs[20:]

	for _, nodeID := range nodeIDsToRemove {
		cache.Init(nodeID)
	}

	var wg sync.WaitGroup
	wg.Add(len(nodeIDs))

	// Initialize spam records concurrently
	for _, nodeID := range nodeIDsToAdd {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.Init(id)
		}(nodeID)
	}

	// Remove spam records concurrently
	for _, nodeID := range nodeIDsToRemove {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.Remove(id)
			require.NotContains(t, id, cache.NodeIDs())
		}(nodeID)
	}

	// Adjust spam records concurrently
	for _, nodeID := range nodeIDsToAdjust {
		go func(id flow.Identifier) {
			defer wg.Done()
			_, _ = cache.ReceivedClusterPrefixedMessage(id)
		}(nodeID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")
	require.ElementsMatch(t, append(nodeIDsToAdd, nodeIDsToAdjust...), cache.NodeIDs())
}

// TestRecordCache_EdgeCasesAndInvalidInputs tests the edge cases and invalid inputs for RecordCache methods.
// The test covers the following scenarios:
// 1. Initializing a record multiple times.
// 2. Adjusting a non-existent record.
// 3. Removing a record multiple times.
func TestRecordCache_EdgeCasesAndInvalidInputs(t *testing.T) {
	cache := cacheFixture(t, 100, defaultDecay, zerolog.Nop(), metrics.NewNoopCollector())

	nodeIDs := unittest.IdentifierListFixture(20)
	nodeIDsToAdd := nodeIDs[:10]
	nodeIDsToRemove := nodeIDs[10:20]

	for _, nodeID := range nodeIDsToRemove {
		cache.Init(nodeID)
	}

	var wg sync.WaitGroup
	wg.Add(len(nodeIDs) + 10)

	// initialize spam records concurrently
	for _, nodeID := range nodeIDsToAdd {
		go func(id flow.Identifier) {
			defer wg.Done()
			require.True(t, cache.Init(id))
			retrieved, ok, err := cache.Get(id)
			require.NoError(t, err)
			require.True(t, ok)
			require.Zero(t, retrieved)
		}(nodeID)
	}

	// remove spam records concurrently
	for _, nodeID := range nodeIDsToRemove {
		go func(id flow.Identifier) {
			defer wg.Done()
			require.True(t, cache.Remove(id))
			require.NotContains(t, id, cache.NodeIDs())
		}(nodeID)
	}

	// call NodeIDs method concurrently
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			ids := cache.NodeIDs()
			// the number of returned IDs should be less than or equal to the number of node IDs
			require.True(t, len(ids) <= len(nodeIDs))
			// the returned IDs should be a subset of the node IDs
			for _, id := range ids {
				require.Contains(t, nodeIDs, id)
			}
		}()
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "timed out waiting for goroutines to finish")
}

// recordFixture creates a new record entity with the given node id.
// Args:
// - id: the node id of the record.
// Returns:
// - RecordEntity: the created record entity.
func recordEntityFixture(id flow.Identifier) ClusterPrefixedMessagesReceivedRecord {
	return ClusterPrefixedMessagesReceivedRecord{NodeID: id, Gauge: 0.0, lastUpdated: time.Now()}
}

// cacheFixture returns a new *RecordCache.
func cacheFixture(t *testing.T, sizeLimit uint32, recordDecay float64, logger zerolog.Logger, collector module.HeroCacheMetrics) *RecordCache {
	recordFactory := func(id flow.Identifier) ClusterPrefixedMessagesReceivedRecord {
		return recordEntityFixture(id)
	}
	config := &RecordCacheConfig{
		sizeLimit:   sizeLimit,
		logger:      logger,
		collector:   collector,
		recordDecay: recordDecay,
	}
	r, err := NewRecordCache(config, recordFactory)
	require.NoError(t, err)
	// expect cache to be empty
	require.Equalf(t, uint(0), r.Size(), "cache size must be 0")
	require.NotNil(t, r)
	return r
}
