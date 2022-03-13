package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/requester/status"
	"github.com/onflow/flow-go/utils/unittest"
)

type StatusSuite struct {
	suite.Suite

	rootHeight       uint64
	maxCachedEntries uint64
	maxSearchAhead   uint64

	ds     datastore.Batching
	status *status.Status

	ctx    context.Context
	cancel context.CancelFunc
}

func TestStatusSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(StatusSuite))
}

func (suite *StatusSuite) SetupTest() {
	suite.rootHeight = uint64(100)
	suite.maxCachedEntries = uint64(10)
	suite.maxSearchAhead = uint64(20)

	suite.ds = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.status = status.New(suite.ds, zerolog.Nop(), suite.rootHeight, suite.maxCachedEntries, suite.maxSearchAhead)

	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 30*time.Second)

	err := suite.status.Load(suite.ctx)
	assert.NoError(suite.T(), err)
}

func (suite *StatusSuite) TearDownTest() {
	suite.cancel()
}

func (suite *StatusSuite) TestLoadFromEmptyState() {
	assert.Equal(suite.T(), suite.rootHeight, suite.status.LastReceived())
	assert.Equal(suite.T(), suite.rootHeight, suite.status.LastProcessed())
	assert.Equal(suite.T(), suite.rootHeight, suite.status.LastNotified())
}

func (suite *StatusSuite) reset(flushDB bool) {
	if flushDB {
		suite.ds = dssync.MutexWrap(datastore.NewMapDatastore())
		suite.status = status.New(suite.ds, zerolog.Nop(), suite.rootHeight, suite.maxCachedEntries, suite.maxSearchAhead)
	}

	err := suite.status.Load(suite.ctx)
	assert.NoError(suite.T(), err)
}

func EntryFixture(height uint64) *status.BlockEntry {
	blockID := unittest.IdentifierFixture()
	return &status.BlockEntry{
		BlockID: blockID,
		Height:  height,
		ExecutionData: &state_synchronization.ExecutionData{
			BlockID: blockID,
		},
	}
}

func (suite *StatusSuite) TestFetched() {
	suite.Run("LastReceived is set correctly when fetched in order", func() {
		testLength := uint64(50)
		for i := uint64(0); i < testLength; i++ {
			entry := EntryFixture(suite.rootHeight + i)
			suite.status.Fetched(entry)
			assert.Equal(suite.T(), entry.Height, suite.status.LastReceived())
		}
		assert.Equal(suite.T(), int(testLength), suite.status.OutstandingNotifications())
	})

	suite.Run("LastReceived is set correctly when fetched out of order", func() {
		suite.reset(true)

		// Fetch in random order by iterating a map of heights
		testLength := uint64(50)
		heights := make(map[uint64]struct{}, testLength)
		for i := uint64(0); i < testLength; i++ {
			heights[suite.rootHeight+i] = struct{}{}
		}

		max := uint64(0)
		for height := range heights {
			if height > max {
				max = height
			}

			entry := EntryFixture(height)
			suite.status.Fetched(entry)
			assert.Equal(suite.T(), max, suite.status.LastReceived())
		}
		assert.Equal(suite.T(), int(testLength), suite.status.OutstandingNotifications())
	})

	suite.Run("fetch ignored when height already notified", func() {
		suite.reset(true)

		before := suite.status.LastReceived()
		entry := EntryFixture(before - 1)

		suite.status.Fetched(entry)
		assert.Equal(suite.T(), before, suite.status.LastReceived())
		assert.Equal(suite.T(), 0, suite.status.OutstandingNotifications())
	})
}

func (suite *StatusSuite) TestCache() {
	testLength := suite.maxCachedEntries * 2
	heights := make([]uint64, testLength)
	heightsMap := make(map[uint64]struct{}, testLength)
	for i := uint64(0); i < testLength; i++ {
		h := suite.rootHeight + i
		heights[int(i)] = h
		heightsMap[h] = struct{}{}
	}

	check := func() {
		notified := make([]uint64, 0, testLength)
		cached := 0
		for {
			entry, ok := suite.status.NextNotification(suite.ctx)
			if !ok {
				break
			}

			notified = append(notified, entry.Height)
			if entry.ExecutionData != nil {
				cached++
			}
		}

		assert.Len(suite.T(), notified, int(testLength))
		assert.LessOrEqual(suite.T(), int(suite.maxCachedEntries), cached)
		assert.Equal(suite.T(), notified, heights)
	}

	suite.Run("max cached enforced properly when fetched in order", func() {
		for _, height := range heights {
			suite.status.Fetched(EntryFixture(height))
		}
		check()
	})

	suite.Run("max cached enforced properly when fetched in order", func() {
		suite.reset(true)

		// Fetch in random order by iterating a map of heights
		for height := range heightsMap {
			suite.status.Fetched(EntryFixture(height))
		}
		check()
	})
}

func (suite *StatusSuite) TestNotifications() {
	entry0 := EntryFixture(suite.rootHeight)
	entry1 := EntryFixture(suite.rootHeight + 1)
	entry2 := EntryFixture(suite.rootHeight + 2)
	entry3 := EntryFixture(suite.rootHeight + 3)

	suite.Run("returns false when no pending notifications", func() {
		entry, ok := suite.status.NextNotification(suite.ctx)
		assert.False(suite.T(), ok)
		assert.Nil(suite.T(), entry)
	})

	suite.Run("returns false when next height is not ready", func() {
		suite.status.Fetched(entry1)

		entry, ok := suite.status.NextNotification(suite.ctx)
		assert.False(suite.T(), ok)
		assert.Nil(suite.T(), entry)
	})

	suite.Run("returns entry when next height is ready until ", func() {
		suite.reset(true)

		suite.status.Fetched(entry0)
		assert.Equal(suite.T(), entry0.Height, suite.status.LastReceived())
		suite.status.Fetched(entry1)
		assert.Equal(suite.T(), entry1.Height, suite.status.LastReceived())
		suite.status.Fetched(entry3) //  leave a gap to the heap's not empty
		assert.Equal(suite.T(), entry3.Height, suite.status.LastReceived())

		// Notify for entry0
		entry, ok := suite.status.NextNotification(suite.ctx)
		assert.True(suite.T(), ok)
		assert.Equal(suite.T(), entry0, entry)

		// Notify for entry1
		entry, ok = suite.status.NextNotification(suite.ctx)
		assert.True(suite.T(), ok)
		assert.Equal(suite.T(), entry1, entry)

		// Next entry isn't available
		entry, ok = suite.status.NextNotification(suite.ctx)
		assert.False(suite.T(), ok)
		assert.Nil(suite.T(), entry)
	})

	suite.Run("drops duplicate entries", func() {
		suite.reset(true)

		suite.status.Fetched(EntryFixture(entry3.Height))
		suite.status.Fetched(EntryFixture(entry3.Height))
		suite.status.Fetched(entry3)
		suite.status.Fetched(entry3)
		suite.status.Fetched(EntryFixture(entry2.Height))
		suite.status.Fetched(entry2)
		suite.status.Fetched(entry2)
		suite.status.Fetched(entry1)
		suite.status.Fetched(entry1)
		suite.status.Fetched(EntryFixture(entry1.Height))
		suite.status.Fetched(EntryFixture(entry1.Height))
		suite.status.Fetched(EntryFixture(entry1.Height))
		suite.status.Fetched(EntryFixture(entry1.Height))
		suite.status.Fetched(EntryFixture(entry1.Height))
		suite.status.Fetched(entry0)

		heights := []uint64{}
		for {
			entry, ok := suite.status.NextNotification(suite.ctx)
			if !ok {
				break
			}
			heights = append(heights, entry.Height)
		}

		assert.Equal(suite.T(), []uint64{
			entry0.Height,
			entry1.Height,
			entry2.Height,
			entry3.Height,
		}, heights)
	})

	suite.Run("lastNotified loaded from db", func() {
		suite.reset(true)

		entry := EntryFixture(suite.rootHeight)
		suite.status.Fetched(entry)

		actual, ok := suite.status.NextNotification(suite.ctx)
		assert.True(suite.T(), ok)
		assert.Equal(suite.T(), entry, actual)

		suite.reset(false) // reset using existing db

		assert.Equal(suite.T(), entry.Height, suite.status.LastNotified())
	})
}

func (suite *StatusSuite) TestHalted() {
	suite.Run("halted defaults to false", func() {
		assert.False(suite.T(), suite.status.Halted())
	})

	suite.Run("calling Halt sets halted to true", func() {
		suite.status.Halt(suite.ctx)

		assert.True(suite.T(), suite.status.Halted())
	})

	suite.Run("halted persists across loads", func() {
		suite.reset(true)

		suite.status.Halt(suite.ctx)
		suite.reset(false) // reset using existing db

		assert.True(suite.T(), suite.status.Halted())
	})
}

func (suite *StatusSuite) TestProcessed() {
	before := suite.status.LastProcessed()
	expected := uint64(500)
	suite.status.Processed(expected)
	after := suite.status.LastProcessed()

	assert.NotEqual(suite.T(), before, after)
	assert.Equal(suite.T(), expected, after)
}

func (suite *StatusSuite) TestIsPaused() {
	suite.Run("fresh status is not paused", func() {
		assert.False(suite.T(), suite.status.IsPaused())
	})

	suite.Run("status is not paused when under search ahead limit", func() {
		lastNotified := suite.status.LastNotified()
		suite.status.Processed(lastNotified + suite.maxSearchAhead)
		assert.False(suite.T(), suite.status.IsPaused())
	})

	suite.Run("status is paused when over search ahead limit", func() {
		lastNotified := suite.status.LastNotified()
		suite.status.Processed(lastNotified + suite.maxSearchAhead + 1)
		assert.True(suite.T(), suite.status.IsPaused())
	})
}

func (suite *StatusSuite) TestNextNotificationHeight() {
	suite.Run("next height returns correct height for startblock", func() {
		assert.Equal(suite.T(), suite.rootHeight, suite.status.NextNotificationHeight())
	})

	suite.Run("next height returns correct height", func() {
		entry := EntryFixture(suite.rootHeight)
		suite.status.Fetched(entry)

		actual, ok := suite.status.NextNotification(suite.ctx)
		assert.True(suite.T(), ok) // Not time to notify yet
		assert.Equal(suite.T(), entry, actual)
		assert.Equal(suite.T(), entry.Height+1, suite.status.NextNotificationHeight())
	})

	suite.Run("next height handles block zero", func() {
		entry0 := EntryFixture(0)
		entry1 := EntryFixture(1)

		suite.rootHeight = 0
		suite.reset(true)

		// 1. Next should be 0 from an empty state with start height == 0
		assert.Equal(suite.T(), uint64(0), suite.status.NextNotificationHeight())

		// Process block 1 out of order
		suite.status.Fetched(entry1)

		entry, ok := suite.status.NextNotification(suite.ctx)
		assert.False(suite.T(), ok) // Not time to notify yet
		assert.Nil(suite.T(), entry)

		// 2. Next should still be 0 since block 0 still hasn't been notified
		assert.Equal(suite.T(), uint64(0), suite.status.NextNotificationHeight())

		// Process block 0
		suite.status.Fetched(entry0)

		entry, ok = suite.status.NextNotification(suite.ctx)
		assert.True(suite.T(), ok)
		assert.Equal(suite.T(), entry0, entry)

		// 3. Next should be 1 after processing the notification for block 0
		assert.Equal(suite.T(), uint64(1), suite.status.NextNotificationHeight())
	})
}
