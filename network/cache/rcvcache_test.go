package netcache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/network"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

type RcvCacheTestSuite struct {
	suite.Suite
	c    *netcache.RcvCache
	size int
}

func TestRcvCacheTestSuite(t *testing.T) {
	suite.Run(t, new(RcvCacheTestSuite))
}

// SetupTest creates a new cache
func (r *RcvCacheTestSuite) SetupTest() {
	const size = 10

	c := netcache.NewRcvCache(size, unittest.Logger())

	r.c = c
	r.size = size
}

// TestSingleElementAdd adds a single element to the cache and verifies its existence.
func (r *RcvCacheTestSuite) TestSingleElementAdd() {
	eventID, err := p2p.EventId(network.Channel("0"), []byte("event-1"))
	require.NoError(r.T(), err)

	assert.True(r.Suite.T(), r.c.Add(eventID))
	assert.False(r.Suite.T(), r.c.Add(eventID))

	// same channel but different event should be treated as unseen
	eventID2, err := p2p.EventId(network.Channel("0"), []byte("event-2"))
	require.NoError(r.T(), err)
	assert.True(r.Suite.T(), r.c.Add(eventID2))
	assert.False(r.Suite.T(), r.c.Add(eventID2))

	// same event but different channels should be treated as unseen
	eventID3, err := p2p.EventId(network.Channel("1"), []byte("event-2"))
	require.NoError(r.T(), err)
	assert.True(r.Suite.T(), r.c.Add(eventID3))
	assert.False(r.Suite.T(), r.c.Add(eventID3))
}

// TestNoneExistence evaluates the correctness of cache operation against non-existing element
func (r *RcvCacheTestSuite) TestNoneExistence() {
	eventID, err := p2p.EventId(network.Channel("1"), []byte("non-existing event"))
	require.NoError(r.T(), err)

	// adding new event to cache should return true
	assert.True(r.Suite.T(), r.c.Add(eventID))
}

// TestMultipleElementAdd adds several eventIDs to th cache and evaluates their xistence
func (r *RcvCacheTestSuite) TestMultipleElementAdd() {
	// creates and populates slice of 10 events
	eventIDs := make([]hash.Hash, 0)
	for i := 0; i < 10; i++ {
		eventID, err := p2p.EventId(network.Channel("1"), []byte(fmt.Sprintf("event-%d", i)))
		require.NoError(r.T(), err)

		eventIDs = append(eventIDs, eventID)
	}

	// adding non-existing even id must return true
	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			assert.True(r.Suite.T(), r.c.Add(eventIDs[i]))

			wg.Done()
		}(i)

	}

	unittest.RequireReturnsBefore(r.T(), wg.Wait, 100*time.Millisecond, "cannot add events to cache on time")

	// adding duplicate event id must return false.
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			assert.False(r.Suite.T(), r.c.Add(eventIDs[i]))

			wg.Done()
		}(i)
	}

	unittest.RequireReturnsBefore(r.T(), wg.Wait, 100*time.Millisecond, "cannot add duplicate events to cache on time")
}
