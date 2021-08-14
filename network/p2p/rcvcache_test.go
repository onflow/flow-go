package p2p

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/network"
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

	c, err := newRcvCache(size)
	require.NoError(r.Suite.T(), err)

	r.c = c
	r.size = size
}

// TestSingleElementAdd adds a single element to the cache and verifies its existence
func (r *RcvCacheTestSuite) TestSingleElementAdd() {
	eventID := []byte("event-1")
	channel := network.Channel("0")
	assert.False(r.Suite.T(), r.c.add(eventID, channel))

	assert.True(r.Suite.T(), r.c.add(eventID, channel))
}

// TestNoneExistence evaluates the correctness of cache operation against non-existing element
func (r *RcvCacheTestSuite) TestNoneExistence() {
	eventID := []byte("non-existing event")
	channel := network.Channel("1")
	assert.False(r.Suite.T(), r.c.add(eventID, channel))
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
		assert.False(r.Suite.T(), r.c.add(events[i], network.Channel(strconv.Itoa(i))))
	}

	// checks for the existence of the added events
	for i := range events {
		assert.True(r.Suite.T(), r.c.add(events[i], network.Channel(strconv.Itoa(i))))
	}
}
