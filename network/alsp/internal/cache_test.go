package internal_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/alsp/internal"
	"github.com/onflow/flow-go/network/alsp/model"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewSpamRecordCache tests the creation of a new SpamRecordCache.
// It ensures that the returned cache is not nil. It does not test the
// functionality of the cache.
func TestNewSpamRecordCache(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
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
func protocolSpamRecordFixture(id flow.Identifier) model.ProtocolSpamRecord {
	return model.ProtocolSpamRecord{
		OriginId:      id,
		Decay:         1000,
		CutoffCounter: 0,
		Penalty:       0,
	}
}

// TestSpamRecordCache_Adjust_Init tests that when the Adjust function is called
// on a record that does not exist in the cache, the record is initialized and
// the adjust function is applied to the initialized record.
func TestSpamRecordCache_Adjust_Init(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()

	recordFactoryCalled := 0
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		require.Less(t, recordFactoryCalled, 2, "record factory must be called only twice")
		return protocolSpamRecordFixture(id)
	}
	adjustFuncIncrement := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		record.Penalty += 1
		return record, nil
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)
	require.Zerof(t, cache.Size(), "expected cache to be empty")

	originID1 := unittest.IdentifierFixture()
	originID2 := unittest.IdentifierFixture()

	// adjusting a spam record for an origin ID that does not exist in the cache should initialize the record.
	initializedPenalty, err := cache.AdjustWithInit(originID1, adjustFuncIncrement)
	require.NoError(t, err, "expected no error")
	require.Equal(t, float64(1), initializedPenalty, "expected initialized penalty to be 1")

	record1, ok := cache.Get(originID1)
	require.True(t, ok, "expected record to exist")
	require.NotNil(t, record1, "expected non-nil record")
	require.Equal(t, originID1, record1.OriginId, "expected record to have correct origin ID")
	require.False(t, record1.DisallowListed, "expected record to not be disallow listed")
	require.Equal(t, cache.Size(), uint(1), "expected cache to have one record")

	// adjusting a spam record for an origin ID that already exists in the cache should not initialize the record,
	// but should apply the adjust function to the existing record.
	initializedPenalty, err = cache.AdjustWithInit(originID1, adjustFuncIncrement)
	require.NoError(t, err, "expected no error")
	require.Equal(t, float64(2), initializedPenalty, "expected initialized penalty to be 2")
	record1Again, ok := cache.Get(originID1)
	require.True(t, ok, "expected record to still exist")
	require.NotNil(t, record1Again, "expected non-nil record")
	require.Equal(t, originID1, record1Again.OriginId, "expected record to have correct origin ID")
	require.False(t, record1Again.DisallowListed, "expected record not to be disallow listed")
	require.Equal(t, cache.Size(), uint(1), "expected cache to still have one record")

	// adjusting a spam record for a different origin ID should initialize the record.
	// this is to ensure that the record factory is called only once.
	initializedPenalty, err = cache.AdjustWithInit(originID2, adjustFuncIncrement)
	require.NoError(t, err, "expected no error")
	require.Equal(t, float64(1), initializedPenalty, "expected initialized penalty to be 1")
	record2, ok := cache.Get(originID2)
	require.True(t, ok, "expected record to exist")
	require.NotNil(t, record2, "expected non-nil record")
	require.Equal(t, originID2, record2.OriginId, "expected record to have correct origin ID")
	require.False(t, record2.DisallowListed, "expected record not to be disallow listed")
	require.Equal(t, cache.Size(), uint(2), "expected cache to have two records")
}

// TestSpamRecordCache_Adjust tests the Adjust method of the SpamRecordCache.
// The test covers the following scenarios:
// 1. Adjusting a spam record for an existing origin ID.
// 2. Attempting to adjust a spam record with an adjustFunc that returns an error.
func TestSpamRecordCache_Adjust_Error(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originID1 := unittest.IdentifierFixture()
	originID2 := unittest.IdentifierFixture()

	// initialize spam records for originID1 and originID2
	penalty, err := cache.AdjustWithInit(originID1, adjustFnNoOp)
	require.NoError(t, err, "expected no error")
	require.Equal(t, 0.0, penalty, "expected penalty to be 0")
	penalty, err = cache.AdjustWithInit(originID2, adjustFnNoOp)
	require.NoError(t, err, "expected no error")
	require.Equal(t, 0.0, penalty, "expected penalty to be 0")

	// test adjusting the spam record for an existing origin ID
	adjustFunc := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		record.Penalty -= 10
		return record, nil
	}
	penalty, err = cache.AdjustWithInit(originID1, adjustFunc)
	require.NoError(t, err)
	require.Equal(t, -10.0, penalty)

	record1, ok := cache.Get(originID1)
	require.True(t, ok)
	require.NotNil(t, record1)
	require.Equal(t, -10.0, record1.Penalty)

	// test adjusting the spam record with an adjustFunc that returns an error
	adjustFuncError := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, errors.New("adjustment error")
	}
	_, err = cache.AdjustWithInit(originID1, adjustFuncError)
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
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originID1 := unittest.IdentifierFixture()
	originID2 := unittest.IdentifierFixture()
	originID3 := unittest.IdentifierFixture()

	// initialize spam records for a few origin IDs
	_, err := cache.AdjustWithInit(originID1, adjustFnNoOp)
	require.NoError(t, err)
	_, err = cache.AdjustWithInit(originID2, adjustFnNoOp)
	require.NoError(t, err)
	_, err = cache.AdjustWithInit(originID3, adjustFnNoOp)
	require.NoError(t, err)

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
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originID1 := unittest.IdentifierFixture()
	originID2 := unittest.IdentifierFixture()
	originID3 := unittest.IdentifierFixture()

	// initialize spam records for a few origin IDs
	_, err := cache.AdjustWithInit(originID1, adjustFnNoOp)
	require.NoError(t, err)
	_, err = cache.AdjustWithInit(originID2, adjustFnNoOp)
	require.NoError(t, err)
	_, err = cache.AdjustWithInit(originID3, adjustFnNoOp)
	require.NoError(t, err)

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
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	// 1. initializing a spam record multiple times
	originID1 := unittest.IdentifierFixture()

	_, err := cache.AdjustWithInit(originID1, adjustFnNoOp)
	require.NoError(t, err)
	_, err = cache.AdjustWithInit(originID1, adjustFnNoOp)
	require.NoError(t, err)

	// 2. Test adjusting a non-existent spam record
	originID2 := unittest.IdentifierFixture()
	initialPenalty, err := cache.AdjustWithInit(originID2, func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		record.Penalty -= 10
		return record, nil
	})
	require.NoError(t, err)
	require.Equal(t, float64(-10), initialPenalty)

	// 3. Test removing a spam record multiple times
	originID3 := unittest.IdentifierFixture()
	_, err = cache.AdjustWithInit(originID3, adjustFnNoOp)
	require.NoError(t, err)
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
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(10)

	var wg sync.WaitGroup
	wg.Add(len(originIDs))

	for _, originID := range originIDs {
		go func(id flow.Identifier) {
			defer wg.Done()
			penalty, err := cache.AdjustWithInit(id, adjustFnNoOp)
			require.NoError(t, err)
			require.Equal(t, float64(0), penalty)
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

// TestSpamRecordCache_ConcurrentSameRecordAdjust tests the concurrent adjust of the same spam record.
// The test covers the following scenarios:
// 1. Multiple goroutines attempting to adjust the same spam record concurrently.
// 2. Only one of the adjust operations succeeds on initializing the record.
// 3. The rest of the adjust operations only update the record (no initialization).
func TestSpamRecordCache_ConcurrentSameRecordAdjust(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFn := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		record.Penalty -= 1.0
		record.DisallowListed = true
		record.Decay += 1.0
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originID := unittest.IdentifierFixture()
	const concurrentAttempts = 10

	var wg sync.WaitGroup
	wg.Add(concurrentAttempts)

	for i := 0; i < concurrentAttempts; i++ {
		go func() {
			defer wg.Done()
			penalty, err := cache.AdjustWithInit(originID, adjustFn)
			require.NoError(t, err)
			require.Less(t, penalty, 0.0) // penalty should be negative
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that the record is correctly initialized and adjusted in the cache
	initDecay := model.SpamRecordFactory()(originID).Decay
	record, found := cache.Get(originID)
	require.True(t, found)
	require.NotNil(t, record)
	require.Equal(t, concurrentAttempts*-1.0, record.Penalty)
	require.Equal(t, initDecay+concurrentAttempts*1.0, record.Decay)
	require.True(t, record.DisallowListed)
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
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(10)
	for _, originID := range originIDs {
		penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
		require.NoError(t, err)
		require.Equal(t, float64(0), penalty)
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

// TestSpamRecordCache_ConcurrentUpdatesAndReads tests the concurrent adjustments and reads of spam records for different
// origin IDs. The test covers the following scenarios:
// 1. Multiple goroutines adjusting spam records for different origin IDs concurrently.
// 2. Multiple goroutines getting spam records for different origin IDs concurrently.
// 3. The adjusted records are correctly updated in the cache.
func TestSpamRecordCache_ConcurrentUpdatesAndReads(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(10)
	for _, originID := range originIDs {
		penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
		require.NoError(t, err)
		require.Equal(t, float64(0), penalty)
	}

	var wg sync.WaitGroup
	wg.Add(len(originIDs) * 2)

	adjustFunc := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		record.Penalty -= 1
		return record, nil
	}

	for _, originID := range originIDs {
		// adjust spam records concurrently
		go func(id flow.Identifier) {
			defer wg.Done()
			_, err := cache.AdjustWithInit(id, adjustFunc)
			require.NoError(t, err)
		}(originID)

		// get spam records concurrently
		go func(id flow.Identifier) {
			defer wg.Done()
			record, found := cache.Get(id)
			require.True(t, found)
			require.NotNil(t, record)
		}(originID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that the records are correctly updated in the cache
	for _, originID := range originIDs {
		record, found := cache.Get(originID)
		require.True(t, found)
		require.Equal(t, -1.0, record.Penalty)
	}
}

// TestSpamRecordCache_ConcurrentInitAndRemove tests the concurrent initialization and removal of spam records for different
// origin IDs. The test covers the following scenarios:
// 1. Multiple goroutines initializing spam records for different origin IDs concurrently.
// 2. Multiple goroutines removing spam records for different origin IDs concurrently.
// 3. The initialized records are correctly added to the cache.
// 4. The removed records are correctly removed from the cache.
func TestSpamRecordCache_ConcurrentInitAndRemove(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(20)
	originIDsToAdd := originIDs[:10]
	originIDsToRemove := originIDs[10:]

	for _, originID := range originIDsToRemove {
		penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
		require.NoError(t, err)
		require.Equal(t, float64(0), penalty)
	}

	var wg sync.WaitGroup
	wg.Add(len(originIDs))

	// initialize spam records concurrently
	for _, originID := range originIDsToAdd {
		originID := originID // capture range variable
		go func() {
			defer wg.Done()
			penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
			require.NoError(t, err)
			require.Equal(t, float64(0), penalty)
		}()
	}

	// remove spam records concurrently
	for _, originID := range originIDsToRemove {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.Remove(id)
		}(originID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that the initialized records are correctly added to the cache
	for _, originID := range originIDsToAdd {
		record, found := cache.Get(originID)
		require.True(t, found)
		require.NotNil(t, record)
	}

	// ensure that the removed records are correctly removed from the cache
	for _, originID := range originIDsToRemove {
		_, found := cache.Get(originID)
		require.False(t, found)
	}
}

// TestSpamRecordCache_ConcurrentInitRemoveAdjust tests the concurrent initialization, removal, and adjustment of spam
// records for different origin IDs. The test covers the following scenarios:
// 1. Multiple goroutines initializing spam records for different origin IDs concurrently.
// 2. Multiple goroutines removing spam records for different origin IDs concurrently.
// 3. Multiple goroutines adjusting spam records for different origin IDs concurrently.
func TestSpamRecordCache_ConcurrentInitRemoveAdjust(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(30)
	originIDsToAdd := originIDs[:10]
	originIDsToRemove := originIDs[10:20]
	originIDsToAdjust := originIDs[20:]

	for _, originID := range originIDsToRemove {
		penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
		require.NoError(t, err)
		require.Equal(t, float64(0), penalty)
	}

	adjustFunc := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		record.Penalty -= 1
		return record, nil
	}

	var wg sync.WaitGroup
	wg.Add(len(originIDs))

	// Initialize spam records concurrently
	for _, originID := range originIDsToAdd {
		originID := originID // capture range variable
		go func() {
			defer wg.Done()
			penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
			require.NoError(t, err)
			require.Equal(t, float64(0), penalty)
		}()
	}

	// Remove spam records concurrently
	for _, originID := range originIDsToRemove {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.Remove(id)
		}(originID)
	}

	// Adjust spam records concurrently
	for _, originID := range originIDsToAdjust {
		go func(id flow.Identifier) {
			defer wg.Done()
			_, _ = cache.AdjustWithInit(id, adjustFunc)
		}(originID)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")
}

// TestSpamRecordCache_ConcurrentInitRemoveAndAdjust tests the concurrent initialization, removal, and adjustment of spam
// records for different origin IDs. The test covers the following scenarios:
// 1. Multiple goroutines initializing spam records for different origin IDs concurrently.
// 2. Multiple goroutines removing spam records for different origin IDs concurrently.
// 3. Multiple goroutines adjusting spam records for different origin IDs concurrently.
// 4. The initialized records are correctly added to the cache.
// 5. The removed records are correctly removed from the cache.
// 6. The adjusted records are correctly updated in the cache.
func TestSpamRecordCache_ConcurrentInitRemoveAndAdjust(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(30)
	originIDsToAdd := originIDs[:10]
	originIDsToRemove := originIDs[10:20]
	originIDsToAdjust := originIDs[20:]

	for _, originID := range originIDsToRemove {
		penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
		require.NoError(t, err)
		require.Equal(t, float64(0), penalty)
	}

	for _, originID := range originIDsToAdjust {
		penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
		require.NoError(t, err)
		require.Equal(t, float64(0), penalty)
	}

	var wg sync.WaitGroup
	wg.Add(len(originIDs))

	// initialize spam records concurrently
	for _, originID := range originIDsToAdd {
		originID := originID
		go func() {
			defer wg.Done()
			penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
			require.NoError(t, err)
			require.Equal(t, float64(0), penalty)
		}()
	}

	// remove spam records concurrently
	for _, originID := range originIDsToRemove {
		originID := originID
		go func() {
			defer wg.Done()
			cache.Remove(originID)
		}()
	}

	// adjust spam records concurrently
	for _, originID := range originIDsToAdjust {
		originID := originID
		go func() {
			defer wg.Done()
			_, err := cache.AdjustWithInit(originID, func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
				record.Penalty -= 1
				return record, nil
			})
			require.NoError(t, err)
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that the initialized records are correctly added to the cache
	for _, originID := range originIDsToAdd {
		record, found := cache.Get(originID)
		require.True(t, found)
		require.NotNil(t, record)
	}

	// ensure that the removed records are correctly removed from the cache
	for _, originID := range originIDsToRemove {
		_, found := cache.Get(originID)
		require.False(t, found)
	}

	// ensure that the adjusted records are correctly updated in the cache
	for _, originID := range originIDsToAdjust {
		record, found := cache.Get(originID)
		require.True(t, found)
		require.NotNil(t, record)
		require.Equal(t, -1.0, record.Penalty)
	}
}

// TestSpamRecordCache_ConcurrentIdentitiesAndOperations tests the concurrent calls to Identities method while
// other goroutines are initializing or removing spam records. The test covers the following scenarios:
// 1. Multiple goroutines initializing spam records for different origin IDs concurrently.
// 2. Multiple goroutines removing spam records for different origin IDs concurrently.
// 3. Multiple goroutines calling Identities method concurrently.
func TestSpamRecordCache_ConcurrentIdentitiesAndOperations(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	recordFactory := func(id flow.Identifier) model.ProtocolSpamRecord {
		return protocolSpamRecordFixture(id)
	}
	adjustFnNoOp := func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		return record, nil // no-op
	}

	cache := internal.NewSpamRecordCache(sizeLimit, logger, collector, recordFactory)
	require.NotNil(t, cache)

	originIDs := unittest.IdentifierListFixture(20)
	originIDsToAdd := originIDs[:10]
	originIDsToRemove := originIDs[10:20]

	for _, originID := range originIDsToRemove {
		penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
		require.NoError(t, err)
		require.Equal(t, float64(0), penalty)
	}

	var wg sync.WaitGroup
	wg.Add(len(originIDs) + 10)

	// initialize spam records concurrently
	for _, originID := range originIDsToAdd {
		originID := originID
		go func() {
			defer wg.Done()
			penalty, err := cache.AdjustWithInit(originID, adjustFnNoOp)
			require.NoError(t, err)
			require.Equal(t, float64(0), penalty)
			retrieved, ok := cache.Get(originID)
			require.True(t, ok)
			require.NotNil(t, retrieved)
		}()
	}

	// remove spam records concurrently
	for _, originID := range originIDsToRemove {
		originID := originID
		go func() {
			defer wg.Done()
			require.True(t, cache.Remove(originID))
			retrieved, ok := cache.Get(originID)
			require.False(t, ok)
			require.Nil(t, retrieved)
		}()
	}

	// call Identities method concurrently
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			ids := cache.Identities()
			// the number of returned IDs should be less than or equal to the number of origin IDs
			require.True(t, len(ids) <= len(originIDs))
			// the returned IDs should be a subset of the origin IDs
			for _, id := range ids {
				require.Contains(t, originIDs, id)
			}
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "timed out waiting for goroutines to finish")
}
