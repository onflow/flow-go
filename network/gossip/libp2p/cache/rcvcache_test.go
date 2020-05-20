package cache

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RcvCacheTestSuite struct {
	suite.Suite
	c    *RcvCache
	size int
}

func TestRcvCacheTestSuite(t *testing.T) {
	suite.Run(t, new(RcvCacheTestSuite))
}

// SetupTest creates a new cache
func (r *RcvCacheTestSuite) SetupTest() {
	const size = 10

	c, err := NewRcvCache(size)
	require.NoError(r.Suite.T(), err)

	r.c = c
	r.size = size
}

// TestSingleElementAdd adds a single element to the cache and verifies its existence
func (r *RcvCacheTestSuite) TestSingleElementAdd() {
	eventID := []byte("event-1")
	channelID := uint32(0)
	assert.False(r.Suite.T(), r.c.Add(eventID, channelID))

	assert.True(r.Suite.T(), r.c.Add(eventID, channelID))
}

// TestNoneExistence evaluates the correctness of cache operation against non-existing element
func (r *RcvCacheTestSuite) TestNoneExistence() {
	eventID := []byte("non-existing event")
	channelID := uint32(1)
	assert.False(r.Suite.T(), r.c.Add(eventID, channelID))
}

// TestMultipleElementAdd adds several eventIDs to th cache and evaluates their xistence
func (r *RcvCacheTestSuite) TestMultipleElementAdd() {
	// creates and populates slice of 10 events

	events := make([][]byte, 0)
	for i := 0; i < 10; i++ {
		events = append(events, []byte(fmt.Sprintf("event-%d", i)))
	}

	// adds all events to the cache
	for i := range events {
		assert.False(r.Suite.T(), r.c.Add(events[i], uint32(i)))
	}

	// checks for the existence of the added events
	for i := range events {
		assert.True(r.Suite.T(), r.c.Add(events[i], uint32(i)))
	}
}
