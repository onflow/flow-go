package internal_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/alsp/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewSpamRecordCache tests the creation of a new SpamRecordCache.
// It ensures that the returned cache is not nil. It does not test the
// functionality of the cache.
func TestNewSpamRecordCache(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)
	require.Equalf(t, uint(0), cache.Size(), "cache size must be 0")
}

// protocolSpamRecordFixture creates a new protocol spam record with the given origin id.
// Args:
// - id: the origin id of the spam record.
// Returns:
// - alsp.ProtocolSpamRecord, the created spam record.
// Note that the returned spam record is not a valid spam record. It is used only for testing.
func protocolSpamRecordFixture(id flow.Identifier) alsp.ProtocolSpamRecord {
	return alsp.ProtocolSpamRecord{
		OriginId:      id,
		Decay:         1000,
		CutoffCounter: 0,
		Penalty:       0,
	}
}

// TestSpamRecordCache_Init tests the Init method of the SpamRecordCache.
// It ensures that the method returns true when a new record is initialized
// and false when an existing record is initialized.
func TestSpamRecordCache_Init(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)
	require.Zerof(t, cache.Size(), "expected cache to be empty")

	originID1 := unittest.IdentifierFixture()
	originID2 := unittest.IdentifierFixture()

	// test initializing a spam record for an origin ID that doesn't exist in the cache
	initialized := cache.Init(originID1)
	require.True(t, initialized, "expected record to be initialized")
	record1, ok := cache.Get(originID1)
	require.True(t, ok, "expected record to exist")
	require.NotNil(t, record1, "expected non-nil record")
	require.Equal(t, originID1, record1.OriginId, "expected record to have correct origin ID")
	require.Equal(t, cache.Size(), uint(1), "expected cache to have one record")

	// test initializing a spam record for an origin ID that already exists in the cache
	initialized = cache.Init(originID1)
	require.False(t, initialized, "expected record not to be initialized")
	record1Again, ok := cache.Get(originID1)
	require.True(t, ok, "expected record to still exist")
	require.NotNil(t, record1Again, "expected non-nil record")
	require.Equal(t, originID1, record1Again.OriginId, "expected record to have correct origin ID")
	require.Equal(t, record1, record1Again, "expected records to be the same")
	require.Equal(t, cache.Size(), uint(1), "expected cache to still have one record")

	// test initializing a spam record for another origin ID
	initialized = cache.Init(originID2)
	require.True(t, initialized, "expected record to be initialized")
	record2, ok := cache.Get(originID2)
	require.True(t, ok, "expected record to exist")
	require.NotNil(t, record2, "expected non-nil record")
	require.Equal(t, originID2, record2.OriginId, "expected record to have correct origin ID")
	require.Equal(t, cache.Size(), uint(2), "expected cache to have two records")
}

// TestSpamRecordCache_Adjust tests the Adjust method of the SpamRecordCache.
// The test covers the following scenarios:
// 1. Adjusting a spam record for an existing origin ID.
// 2. Attempting to adjust a spam record for a non-existing origin ID.
// 3. Attempting to adjust a spam record with an adjustFunc that returns an error.
func TestSpamRecordCache_Adjust(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originID1 := unittest.IdentifierFixture()
	originID2 := unittest.IdentifierFixture()

	// initialize spam records for originID1 and originID2
	require.True(t, cache.Init(originID1))
	require.True(t, cache.Init(originID2))

	// test adjusting the spam record for an existing origin ID
	adjustFunc := func(record alsp.ProtocolSpamRecord) (alsp.ProtocolSpamRecord, error) {
		record.Penalty -= 10
		return record, nil
	}
	penalty, err := cache.Adjust(originID1, adjustFunc)
	require.NoError(t, err)
	require.Equal(t, -10.0, penalty)

	record1, ok := cache.Get(originID1)
	require.True(t, ok)
	require.NotNil(t, record1)
	require.Equal(t, -10.0, record1.Penalty)

	// test adjusting the spam record for a non-existing origin ID
	originID3 := unittest.IdentifierFixture()
	_, err = cache.Adjust(originID3, adjustFunc)
	require.Error(t, err)

	// test adjusting the spam record with an adjustFunc that returns an error
	adjustFuncError := func(record alsp.ProtocolSpamRecord) (alsp.ProtocolSpamRecord, error) {
		return record, errors.New("adjustment error")
	}
	_, err = cache.Adjust(originID1, adjustFuncError)
	require.Error(t, err)

	// even though the adjustFunc returned an error, the record should be intact.
	record1, ok = cache.Get(originID1)
	require.True(t, ok)
	require.NotNil(t, record1)
	require.Equal(t, -10.0, record1.Penalty)
}

// TestSpamRecordCache_Identities tests the Identities method of the SpamRecordCache.
// The test covers the following scenarios:
// 1. Initializing the cache with multiple spam records.
// 2. Checking if the Identities method returns the correct set of origin IDs.
func TestSpamRecordCache_Identities(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	// initialize spam records for a few origin IDs
	originID1 := unittest.IdentifierFixture()
	originID2 := unittest.IdentifierFixture()
	originID3 := unittest.IdentifierFixture()

	require.True(t, cache.Init(originID1))
	require.True(t, cache.Init(originID2))
	require.True(t, cache.Init(originID3))

	// check if the Identities method returns the correct set of origin IDs
	identities := cache.Identities()
	require.Equal(t, 3, len(identities))

	identityMap := make(map[flow.Identifier]struct{})
	for _, id := range identities {
		identityMap[id] = struct{}{}
	}

	require.Contains(t, identityMap, originID1)
	require.Contains(t, identityMap, originID2)
	require.Contains(t, identityMap, originID3)
}

// TestSpamRecordCache_Remove tests the Remove method of the SpamRecordCache.
// The test covers the following scenarios:
// 1. Initializing the cache with multiple spam records.
// 2. Removing a spam record and checking if it is removed correctly.
// 3. Ensuring the other spam records are still in the cache after removal.
// 4. Attempting to remove a non-existent origin ID.
func TestSpamRecordCache_Remove(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	// initialize spam records for a few origin IDs
	originID1 := unittest.IdentifierFixture()
	originID2 := unittest.IdentifierFixture()
	originID3 := unittest.IdentifierFixture()

	require.True(t, cache.Init(originID1))
	require.True(t, cache.Init(originID2))
	require.True(t, cache.Init(originID3))

	// remove originID1 and check if the record is removed
	require.True(t, cache.Remove(originID1))
	_, exists := cache.Get(originID1)
	require.False(t, exists)

	// check if the other origin IDs are still in the cache
	_, exists = cache.Get(originID2)
	require.True(t, exists)
	_, exists = cache.Get(originID3)
	require.True(t, exists)

	// attempt to remove a non-existent origin ID
	originID4 := unittest.IdentifierFixture()
	require.False(t, cache.Remove(originID4))
}

// TestSpamRecordCache_EdgeCasesAndInvalidInputs tests the edge cases and invalid inputs for SpamRecordCache methods.
// The test covers the following scenarios:
// 1. Initializing a spam record multiple times.
// 2. Adjusting a non-existent spam record.
// 3. Removing a spam record multiple times.
func TestSpamRecordCache_EdgeCasesAndInvalidInputs(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	// 1. initializing a spam record multiple times
	originID1 := unittest.IdentifierFixture()
	require.True(t, cache.Init(originID1))
	require.False(t, cache.Init(originID1))

	// 2. Test adjusting a non-existent spam record
	originID2 := unittest.IdentifierFixture()
	_, err := cache.Adjust(originID2, func(record alsp.ProtocolSpamRecord) (alsp.ProtocolSpamRecord, error) {
		record.Penalty -= 10
		return record, nil
	})
	require.Error(t, err)

	// 3. Test removing a spam record multiple times
	originID3 := unittest.IdentifierFixture()
	require.True(t, cache.Init(originID3))
	require.True(t, cache.Remove(originID3))
	require.False(t, cache.Remove(originID3))
}

// TestSpamRecordCache_ConcurrentInitialization tests the concurrent initialization of spam records.
// The test covers the following scenarios:
// 1. Multiple goroutines initializing spam records for different origin IDs.
// 2. Ensuring that all spam records are correctly initialized.
func TestSpamRecordCache_ConcurrentInitialization(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(10)

	var wg sync.WaitGroup
	wg.Add(len(originIDs))

	for _, originID := range originIDs {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.Init(id)
		}(originID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that all spam records are correctly initialized
	for _, originID := range originIDs {
		record, found := cache.Get(originID)
		require.True(t, found)
		require.NotNil(t, record)
		require.Equal(t, originID, record.OriginId)
	}
}

// TestSpamRecordCache_ConcurrentSameRecordInitialization tests the concurrent initialization of the same spam record.
// The test covers the following scenarios:
// 1. Multiple goroutines attempting to initialize the same spam record concurrently.
// 2. Only one goroutine successfully initializes the record, and others receive false on initialization.
// 3. The record is correctly initialized in the cache and can be retrieved using the Get method.
func TestSpamRecordCache_ConcurrentSameRecordInitialization(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originID := unittest.IdentifierFixture()
	const concurrentAttempts = 10

	var wg sync.WaitGroup
	wg.Add(concurrentAttempts)

	successCount := atomic.Int32{}

	for i := 0; i < concurrentAttempts; i++ {
		go func() {
			defer wg.Done()
			initSuccess := cache.Init(originID)
			if initSuccess {
				successCount.Inc()
			}
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that only one goroutine successfully initialized the record
	require.Equal(t, int32(1), successCount.Load())

	// ensure that the record is correctly initialized in the cache
	record, found := cache.Get(originID)
	require.True(t, found)
	require.NotNil(t, record)
	require.Equal(t, originID, record.OriginId)
}

// TestSpamRecordCache_ConcurrentRemoval tests the concurrent removal of spam records for different origin IDs.
// The test covers the following scenarios:
// 1. Multiple goroutines removing spam records for different origin IDs concurrently.
// 2. The records are correctly removed from the cache.
func TestSpamRecordCache_ConcurrentRemoval(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) alsp.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(10)
	for _, originID := range originIDs {
		cache.Init(originID)
	}

	var wg sync.WaitGroup
	wg.Add(len(originIDs))

	for _, originID := range originIDs {
		go func(id flow.Identifier) {
			defer wg.Done()
			removed := cache.Remove(id)
			require.True(t, removed)
		}(originID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that the records are correctly removed from the cache
	for _, originID := range originIDs {
		_, found := cache.Get(originID)
		require.False(t, found)
	}

	// ensure that the cache is empty
	require.Equal(t, uint(0), cache.Size())
}
