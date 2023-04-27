package internal_test

import (
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

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
