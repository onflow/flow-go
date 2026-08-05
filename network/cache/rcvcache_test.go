package netcache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/network/message"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/network/channels"

	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/module/metrics"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

type ReceiveCacheTestSuite struct {
	suite.Suite
	c    *netcache.ReceiveCache
	size int
}

func TestReceiveCacheTestSuite(t *testing.T) {
	suite.Run(t, new(ReceiveCacheTestSuite))
}

// SetupTest creates a new cache
func (r *ReceiveCacheTestSuite) SetupTest() {
	const size = 10

	c := netcache.NewHeroReceiveCache(size, unittest.Logger(), metrics.NewNoopCollector())

	r.c = c
	r.size = size
}

// TestSingleElementAdd adds a single element to the cache and verifies its existence.
func (r *ReceiveCacheTestSuite) TestSingleElementAdd() {
	eventID, err := message.EventId(channels.Channel("0"), []byte("event-1"))
	require.NoError(r.T(), err)

	assert.True(r.Suite.T(), r.c.Add(eventID))
	assert.False(r.Suite.T(), r.c.Add(eventID))

	// same channel but different event should be treated as unseen
	eventID2, err := message.EventId(channels.Channel("0"), []byte("event-2"))
	require.NoError(r.T(), err)
	assert.True(r.Suite.T(), r.c.Add(eventID2))
	assert.False(r.Suite.T(), r.c.Add(eventID2))

	// same event but different channels should be treated as unseen
	eventID3, err := message.EventId(channels.Channel("1"), []byte("event-2"))
	require.NoError(r.T(), err)
	assert.True(r.Suite.T(), r.c.Add(eventID3))
	assert.False(r.Suite.T(), r.c.Add(eventID3))
}

// TestNoneExistence evaluates the correctness of cache operation against non-existing element
func (r *ReceiveCacheTestSuite) TestNoneExistence() {
	eventID, err := message.EventId(channels.Channel("1"), []byte("non-existing event"))
	require.NoError(r.T(), err)

	// adding new event to cache should return true
	assert.True(r.Suite.T(), r.c.Add(eventID))
}

// TestMultipleElementAdd adds several eventIDs to th cache and evaluates their existence
func (r *ReceiveCacheTestSuite) TestMultipleElementAdd() {
	// creates and populates slice of 10 events
	eventIDs := make([]hash.Hash, 0)
	for i := 0; i < r.size; i++ {
		eventID, err := message.EventId(channels.Channel("1"), []byte(fmt.Sprintf("event-%d", i)))
		require.NoError(r.T(), err)

		eventIDs = append(eventIDs, eventID)
	}

	// adding non-existing even id must return true
	wg := sync.WaitGroup{}
	wg.Add(r.size)
	for i := 0; i < r.size; i++ {
		go func(i int) {
			assert.True(r.Suite.T(), r.c.Add(eventIDs[i]))

			wg.Done()
		}(i)

	}

	unittest.RequireReturnsBefore(r.T(), wg.Wait, 100*time.Millisecond, "cannot add events to cache on time")

	// adding duplicate event id must return false.
	wg.Add(r.size)
	for i := 0; i < r.size; i++ {
		go func(i int) {
			assert.False(r.Suite.T(), r.c.Add(eventIDs[i]))

			wg.Done()
		}(i)
	}

	unittest.RequireReturnsBefore(r.T(), wg.Wait, 100*time.Millisecond, "cannot add duplicate events to cache on time")
}

// TestLRU makes sure that received cache is configured in LRU mode.
func (r *ReceiveCacheTestSuite) TestLRU() {
	eventIDs := make([]hash.Hash, 0)
	total := r.size + 1
	for i := 0; i < total; i++ {
		eventID, err := message.EventId(channels.Channel("1"), []byte(fmt.Sprintf("event-%d", i)))
		require.NoError(r.T(), err)

		eventIDs = append(eventIDs, eventID)
	}

	// adding non-existing even id must return true
	for i := 0; i < total; i++ {
		assert.True(r.Suite.T(), r.c.Add(eventIDs[i]))
	}

	// when adding 11th element, cache goes beyond its size,
	// and 1st element (i.e., index 0) is ejected.
	// Now when trying to add 1st element again, it seems
	// new to cache and replaces 2nd element (at index 1) with it.
	require.True(r.T(), r.c.Add(eventIDs[0]))

}
