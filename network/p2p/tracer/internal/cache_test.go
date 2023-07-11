package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCache_Init tests the init method of the rpcSentCache.
// It ensures that the method returns true when a new record is initialized
// and false when an existing record is initialized.
func TestCache_Init(t *testing.T) {
	cache := cacheFixture(t, 100, zerolog.Nop(), metrics.NewNoopCollector())
	controlMsgType := p2pmsg.CtrlMsgIHave
	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()

	// test initializing a record for an ID that doesn't exist in the cache
	initialized := cache.init(id1, controlMsgType)
	require.True(t, initialized, "expected record to be initialized")
	require.True(t, cache.has(id1), "expected record to exist")

	// test initializing a record for an ID that already exists in the cache
	initialized = cache.init(id1, controlMsgType)
	require.False(t, initialized, "expected record not to be initialized")
	require.True(t, cache.has(id1), "expected record to exist")

	// test initializing a record for another ID
	initialized = cache.init(id2, controlMsgType)
	require.True(t, initialized, "expected record to be initialized")
	require.True(t, cache.has(id2), "expected record to exist")
}

// TestCache_ConcurrentInit tests the concurrent initialization of records.
// The test covers the following scenarios:
// 1. Multiple goroutines initializing records for different ids.
// 2. Ensuring that all records are correctly initialized.
func TestCache_ConcurrentInit(t *testing.T) {
	cache := cacheFixture(t, 100, zerolog.Nop(), metrics.NewNoopCollector())
	controlMsgType := p2pmsg.CtrlMsgIHave
	ids := unittest.IdentifierListFixture(10)

	var wg sync.WaitGroup
	wg.Add(len(ids))

	for _, id := range ids {
		go func(id flow.Identifier) {
			defer wg.Done()
			cache.init(id, controlMsgType)
		}(id)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that all records are correctly initialized
	for _, id := range ids {
		require.True(t, cache.has(id))
	}
}

// TestCache_ConcurrentSameRecordInit tests the concurrent initialization of the same record.
// The test covers the following scenarios:
// 1. Multiple goroutines attempting to initialize the same record concurrently.
// 2. Only one goroutine successfully initializes the record, and others receive false on initialization.
// 3. The record is correctly initialized in the cache and can be retrieved using the Get method.
func TestCache_ConcurrentSameRecordInit(t *testing.T) {
	cache := cacheFixture(t, 100, zerolog.Nop(), metrics.NewNoopCollector())
	controlMsgType := p2pmsg.CtrlMsgIHave
	id := unittest.IdentifierFixture()
	const concurrentAttempts = 10

	var wg sync.WaitGroup
	wg.Add(concurrentAttempts)

	successGauge := atomic.Int32{}

	for i := 0; i < concurrentAttempts; i++ {
		go func() {
			defer wg.Done()
			initSuccess := cache.init(id, controlMsgType)
			if initSuccess {
				successGauge.Inc()
			}
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

	// ensure that only one goroutine successfully initialized the record
	require.Equal(t, int32(1), successGauge.Load())

	// ensure that the record is correctly initialized in the cache
	require.True(t, cache.has(id))
}

// cacheFixture returns a new *RecordCache.
func cacheFixture(t *testing.T, sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *rpcSentCache {
	config := &rpcCtrlMsgSentCacheConfig{
		sizeLimit: sizeLimit,
		logger:    logger,
		collector: collector,
	}
	r := newRPCSentCache(config)
	// expect cache to be empty
	require.Equalf(t, uint(0), r.size(), "cache size must be 0")
	require.NotNil(t, r)
	return r
}
