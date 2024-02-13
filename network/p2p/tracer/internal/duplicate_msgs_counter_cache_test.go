package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

const defaultDecay = .99
const defaultSkipDecayThreshold = 0.1

// TestDuplicateMessageTrackerCache_Init tests the Init method of the RecordCache.
// It ensures that the method returns true when a new record is initialized
// and false when an existing record is initialized.
func TestDuplicateMessageTrackerCache_Init(t *testing.T) {
	cache := duplicateMessageTrackerCacheFixture(t, 100, defaultDecay, defaultSkipDecayThreshold, zerolog.Nop(), metrics.NewNoopCollector())

	peerID1 := unittest.PeerIdFixture(t)
	peerID2 := unittest.PeerIdFixture(t)

	// test initializing a record for an node ID that doesn't exist in the cache
	gauge, ok, err := cache.GetWithInit(peerID1)
	require.NoError(t, err)
	require.True(t, ok, "expected record to exist")
	require.Zerof(t, gauge, "expected gauge to be 0")
	require.Equal(t, uint(1), cache.c.Size(), "expected cache to have one additional record")

	// test initializing a record for an node ID that already exists in the cache
	gaugeAgain, ok, err := cache.GetWithInit(peerID1)
	require.NoError(t, err)
	require.True(t, ok, "expected record to still exist")
	require.Zerof(t, gaugeAgain, "expected same gauge to be 0")
	require.Equal(t, gauge, gaugeAgain, "expected records to be the same")
	require.Equal(t, uint(1), cache.c.Size(), "expected cache to still have one additional record")

	// test initializing a record for another node ID
	gauge2, ok, err := cache.GetWithInit(peerID2)
	require.NoError(t, err)
	require.True(t, ok, "expected record to exist")
	require.Zerof(t, gauge2, "expected second gauge to be 0")
	require.Equal(t, uint(2), cache.c.Size(), "expected cache to have two additional records")
}

// TestDuplicateMessageTrackerCache_ConcurrentInit tests the concurrent initialization of records.
// The test covers the following scenarios:
// 1. Multiple goroutines initializing records for different node IDs.
// 2. Ensuring that all records are correctly initialized.
func TestDuplicateMessageTrackerCache_ConcurrentInit(t *testing.T) {
	cache := duplicateMessageTrackerCacheFixture(t, 100, defaultDecay, defaultSkipDecayThreshold, zerolog.Nop(), metrics.NewNoopCollector())

	peerIDs := unittest.PeerIdFixtures(t, 10)

	var wg sync.WaitGroup
	wg.Add(len(peerIDs))

	for _, peerID := range peerIDs {
		go func(id peer.ID) {
			defer wg.Done()
			gauge, found, err := cache.GetWithInit(id)
			require.NoError(t, err)
			require.True(t, found)
			require.Zerof(t, gauge, "expected all gauge values to be initialized to 0")
		}(peerID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")
}

// TestDuplicateMessageTrackerCache_ConcurrentSameRecordInit tests the concurrent initialization of the same record.
// The test covers the following scenarios:
// 1. Multiple goroutines attempting to initialize the same record concurrently.
// 2. Only one goroutine successfully initializes the record, and others receive false on initialization.
// 3. The record is correctly initialized in the cache and can be retrieved using the GetWithInit method.
func TestDuplicateMessageTrackerCache_ConcurrentSameRecordInit(t *testing.T) {
	cache := duplicateMessageTrackerCacheFixture(t, 100, defaultDecay, defaultSkipDecayThreshold, zerolog.Nop(), metrics.NewNoopCollector())

	peerID := unittest.PeerIdFixture(t)
	const concurrentAttempts = 10

	var wg sync.WaitGroup
	wg.Add(concurrentAttempts)

	for i := 0; i < concurrentAttempts; i++ {
		go func() {
			defer wg.Done()
			gauge, found, err := cache.GetWithInit(peerID)
			require.NoError(t, err)
			require.True(t, found)
			require.Zero(t, gauge)
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that only one goroutine successfully initialized the record
	require.Equal(t, uint(1), cache.c.Size())
}

// TestDuplicateMessageTrackerCache_DuplicateMessageReceived tests the DuplicateMessageReceived method of the RecordCache.
// The test covers the following scenarios:
// 1. Updating a record gauge for an existing peer ID.
// 2. Attempting to update a record gauge  for a non-existing peer ID should not result in error. DuplicateMessageReceived should always attempt to initialize the gauge.
// 3. Multiple updates on the same record only initialize the record once.
func TestDuplicateMessageTrackerCache_DuplicateMessageReceived(t *testing.T) {
	cache := duplicateMessageTrackerCacheFixture(t, 100, defaultDecay, defaultSkipDecayThreshold, zerolog.Nop(), metrics.NewNoopCollector())

	peerID1 := unittest.PeerIdFixture(t)
	peerID2 := unittest.PeerIdFixture(t)

	gauge, err := cache.DuplicateMessageReceived(peerID1)
	require.NoError(t, err)
	require.Equal(t, float64(1), gauge)

	// get will apply a slightl decay resulting
	// in a gauge value less than gauge which is 1 but greater than 0.9
	currentGauge, ok, err := cache.GetWithInit(peerID1)
	require.NoError(t, err)
	require.True(t, ok)
	require.LessOrEqual(t, currentGauge, gauge)
	require.Greater(t, currentGauge, 0.9)

	_, ok, err = cache.GetWithInit(peerID2)
	require.NoError(t, err)
	require.True(t, ok)

	// test adjusting the spam record for a non-existing node ID
	peerID3 := unittest.PeerIdFixture(t)
	gauge3, err := cache.DuplicateMessageReceived(peerID3)
	require.NoError(t, err)
	require.Equal(t, float64(1), gauge3)

	// when updated the value should be incremented from 1 -> 2 and slightly decayed resulting
	// in a gauge value less than 2 but greater than 1.9
	gauge3, err = cache.DuplicateMessageReceived(peerID3)
	require.NoError(t, err)
	require.LessOrEqual(t, gauge3, 2.0)
	require.Greater(t, gauge3, 1.9)
}

// TestDuplicateMessageTrackerCache_ConcurrentDuplicateMessageReceived tests the concurrent adjustments and reads of records for different
// node IDs. The test covers the following scenarios:
// 1. Multiple goroutines adjusting records for different peer IDs concurrently.
// 2. Multiple goroutines getting records for different peer IDs concurrently.
// 3. The adjusted records are correctly updated in the cache.
// 4. Ensure records are decayed as expected.
func TestDuplicateMessageTrackerCache_ConcurrentDuplicateMessageReceived(t *testing.T) {
	cache := duplicateMessageTrackerCacheFixture(t, 100, defaultDecay, defaultSkipDecayThreshold, zerolog.Nop(), metrics.NewNoopCollector())

	peerIDs := unittest.PeerIdFixtures(t, 10)
	for _, peerID := range peerIDs {
		_, ok, err := cache.GetWithInit(peerID)
		require.NoError(t, err)
		require.True(t, ok)
	}

	var wg sync.WaitGroup
	wg.Add(len(peerIDs) * 2)

	for _, peerID := range peerIDs {
		// adjust spam records concurrently
		go func(id peer.ID) {
			defer wg.Done()
			_, err := cache.DuplicateMessageReceived(id)
			require.NoError(t, err)
		}(peerID)

		// get spam records concurrently
		go func(id peer.ID) {
			defer wg.Done()
			_, found, err := cache.GetWithInit(id)
			require.NoError(t, err)
			require.True(t, found)
		}(peerID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that the records are correctly updated in the cache
	for _, nodeID := range peerIDs {
		gauge, found, err := cache.GetWithInit(nodeID)
		require.NoError(t, err)
		require.True(t, found)
		// slight decay will result in 0.9 < gauge < 1
		require.LessOrEqual(t, gauge, 1.0)
		require.Greater(t, gauge, 0.9)
	}
}

// TestDuplicateMessageTrackerCache_UpdateDecay ensures that a counter value in the record cache is eventually decayed back to 0 after some time.
func TestDuplicateMessageTrackerCache_Decay(t *testing.T) {
	cache := duplicateMessageTrackerCacheFixture(t, 100, 0.09, defaultSkipDecayThreshold, zerolog.Nop(), metrics.NewNoopCollector())

	peerID := unittest.PeerIdFixture(t)

	// initialize spam records for peerID and nodeID2
	gauge, err := cache.DuplicateMessageReceived(peerID)
	require.Equal(t, float64(1), gauge)
	require.NoError(t, err)
	gauge, ok, err := cache.GetWithInit(peerID)
	require.True(t, ok)
	require.NoError(t, err)
	// gauge should have been delayed slightly
	require.True(t, gauge < float64(1))

	time.Sleep(time.Second)

	gauge, ok, err = cache.GetWithInit(peerID)
	require.True(t, ok)
	require.NoError(t, err)
	// gauge should have been delayed slightly, but closer to 0
	require.Less(t, gauge, 0.1)
}

// rpcSentCacheFixture returns a new *DuplicateMessageTrackerCache.
func duplicateMessageTrackerCacheFixture(t *testing.T, sizeLimit uint32, decay, skipDecayThreshold float64, logger zerolog.Logger, collector module.HeroCacheMetrics) *DuplicateMessageTrackerCache {
	r := NewDuplicateMessageTrackerCache(sizeLimit, decay, skipDecayThreshold, logger, collector)
	// expect cache to be empty
	require.Equalf(t, uint(0), r.c.Size(), "cache size must be 0")
	require.NotNil(t, r)
	return r
}
