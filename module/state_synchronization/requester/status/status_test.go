package status_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/state_synchronization/requester/status"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type StatusSuite struct {
	suite.Suite

	rootHeight       uint64
	maxCachedEntries uint64

	ds       datastore.Batching
	status   *status.Status
	progress *storagemock.ConsumerProgress

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

	suite.progress = new(storagemock.ConsumerProgress)
	suite.progress.On("ProcessedIndex").Return(suite.rootHeight, nil)
	suite.progress.On("Halted").Return(nil, nil)

	suite.ds = dssync.MutexWrap(datastore.NewMapDatastore())
	suite.status = status.New(zerolog.Nop(), suite.maxCachedEntries, suite.progress)

	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 30*time.Second)

	err := suite.status.Load()
	assert.NoError(suite.T(), err)
}

func (suite *StatusSuite) TearDownTest() {
	suite.cancel()
}

func (suite *StatusSuite) reset(flushDB bool) {
	if flushDB {
		suite.ds = dssync.MutexWrap(datastore.NewMapDatastore())
		suite.status = status.New(zerolog.Nop(), suite.maxCachedEntries, suite.progress)
	}

	err := suite.status.Load()
	assert.NoError(suite.T(), err)
}

// TestLoadFromEmptyState tests that the status is initialized correctly when starting with an
// empty state
func (suite *StatusSuite) TestLoad() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		progress := bstorage.NewConsumerProgress(db, module.ConsumeProgressExecutionDataRequesterNotification)
		err := progress.InitProcessedIndex(suite.rootHeight)
		assert.NoError(suite.T(), err)

		suite.Run("Load from an empty state", func() {
			s := status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
			err = s.Load()
			assert.NoError(suite.T(), err)

			assert.Equal(suite.T(), suite.rootHeight, s.LastNotified())
			assert.Nil(suite.T(), s.Halted())
		})

		suite.Run("Load from a non-empty state", func() {
			// reuses progress and db
			s := status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
			err = s.Load()
			assert.NoError(suite.T(), err)

			assert.Equal(suite.T(), suite.rootHeight, s.LastNotified())
			assert.Nil(suite.T(), s.Halted())
		})

		suite.Run("Load from a non-empty state", func() {
			// set new value for processed and halted
			haltErr := &status.RequesterHaltedError{
				ExecutionDataID: unittest.IdentifierFixture(),
				BlockID:         unittest.IdentifierFixture(),
				Height:          suite.rootHeight + 1,
				Err:             err,
			}
			err := progress.SetProcessedIndex(suite.rootHeight + 1)
			assert.NoError(suite.T(), err)
			err = progress.SetHalted(haltErr)
			assert.NoError(suite.T(), err)

			s := status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
			err = s.Load()
			assert.NoError(suite.T(), err)

			assert.Equal(suite.T(), suite.rootHeight+1, s.LastNotified())
			require.NotNil(suite.T(), s.Halted())
			assert.Equal(suite.T(), haltErr.Error(), s.Halted().Error())
		})
	})

	suite.Run("Load returns error from progress", func() {
		expectedErr := errors.New("load error")
		progress := new(storagemock.ConsumerProgress)
		progress.On("ProcessedIndex").Return(uint64(0), expectedErr).Once()

		s := status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
		err := s.Load()
		assert.ErrorIs(suite.T(), err, expectedErr)
	})

	suite.Run("Load returns error from halted", func() {
		expectedErr := errors.New("halted error")
		progress := new(storagemock.ConsumerProgress)
		progress.On("ProcessedIndex").Return(suite.rootHeight, nil).Once()
		progress.On("Halted").Return(nil, expectedErr).Once()

		s := status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
		err := s.Load()
		assert.ErrorIs(suite.T(), err, expectedErr)
	})

	suite.Run("Load initializes halted state", func() {
		progress := new(storagemock.ConsumerProgress)
		progress.On("ProcessedIndex").Return(suite.rootHeight, nil).Once()
		progress.On("Halted").Return(nil, storage.ErrNotFound).Once()
		progress.On("InitHalted").Return(nil).Once()

		s := status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
		err := s.Load()
		assert.NoError(suite.T(), err)
	})

	suite.Run("Load returns error from initializing halted state", func() {
		expectedErr := errors.New("initialization error")
		progress := new(storagemock.ConsumerProgress)
		progress.On("ProcessedIndex").Return(suite.rootHeight, nil).Once()
		progress.On("Halted").Return(nil, storage.ErrNotFound).Once()
		progress.On("InitHalted").Return(expectedErr).Once()

		s := status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
		err := s.Load()
		assert.ErrorIs(suite.T(), err, expectedErr)
	})
}

// TestFetched tests that the status is updated correctly when a new entry is fetched
func (suite *StatusSuite) TestFetched() {
	suite.Run("fetched adds all entries when in order", func() {
		testLength := uint64(100)
		for i := uint64(1); i <= testLength; i++ {
			entry := synctest.BlockEntryFixture(suite.rootHeight + i)
			suite.status.Fetched(entry)

			assert.Equal(suite.T(), int(i), suite.status.OutstandingNotifications())
		}
	})

	suite.Run("fetched adds all entries when out of order", func() {
		suite.reset(true)

		// Fetch in random order by iterating a map of heights
		testLength := uint64(100)
		heights := make(map[uint64]struct{}, testLength)
		for i := uint64(1); i <= testLength; i++ {
			heights[suite.rootHeight+i] = struct{}{}
		}

		count := 0
		max := uint64(0)
		for height := range heights {
			if height > max {
				max = height
			}

			entry := synctest.BlockEntryFixture(height)
			suite.status.Fetched(entry)
			count++

			assert.Equal(suite.T(), count, suite.status.OutstandingNotifications())
		}
	})

	suite.Run("fetch ignored when height already notified", func() {
		suite.reset(true)

		entry := synctest.BlockEntryFixture(suite.rootHeight)
		suite.status.Fetched(entry)

		assert.Equal(suite.T(), 0, suite.status.OutstandingNotifications())
	})
}

// TestCache tests that ExecutionData objects are cached correctly
func (suite *StatusSuite) TestCache() {

	testLength := suite.maxCachedEntries * 2
	heights := make([]uint64, testLength)
	heightsMap := make(map[uint64]struct{}, testLength)
	for i := uint64(0); i < testLength; i++ {
		h := suite.rootHeight + 1 + i
		heights[int(i)] = h
		heightsMap[h] = struct{}{}
	}

	check := func(height uint64) {
		notified := make([]uint64, 0, testLength)
		cached := 0
		for {
			job, err := suite.status.AtIndex(height)
			if errors.Is(err, storage.ErrNotFound) {
				break
			}
			assert.NoError(suite.T(), err)

			entry, err := status.JobToBlockEntry(job)
			assert.NoError(suite.T(), err)

			suite.status.Notified(entry.Height)
			notified = append(notified, entry.Height)
			if entry.ExecutionData != nil {
				cached++
			}
			height++
		}

		assert.Len(suite.T(), notified, int(testLength), "missing notifications")
		assert.LessOrEqual(suite.T(), cached, int(suite.maxCachedEntries), "too many cached entries")
		assert.Equal(suite.T(), heights, notified, "missing/out of order notifications")
	}

	suite.Run("maxCachedEntries enforced properly when fetched in order", func() {
		for _, height := range heights {
			suite.status.Fetched(synctest.BlockEntryFixture(height))
		}
		check(suite.rootHeight + 1)
	})

	suite.Run("maxCachedEntries enforced properly when fetched in order", func() {
		suite.reset(true)

		// Fetch in random order by iterating a map of heights
		for height := range heightsMap {
			suite.status.Fetched(synctest.BlockEntryFixture(height))
		}
		check(suite.rootHeight + 1)
	})
}

func (suite *StatusSuite) TestAtIndex() {
	entry1 := synctest.BlockEntryFixture(suite.rootHeight + 1)
	entry2 := synctest.BlockEntryFixture(suite.rootHeight + 2)
	entry3 := synctest.BlockEntryFixture(suite.rootHeight + 3)
	entry4 := synctest.BlockEntryFixture(suite.rootHeight + 4)

	suite.Run("returns NotFound when no pending notifications", func() {
		job, err := suite.status.AtIndex(entry1.Height)
		assert.ErrorIs(suite.T(), err, storage.ErrNotFound)
		assert.Nil(suite.T(), job)
	})

	suite.Run("returns NotFound when next height is not ready", func() {
		suite.status.Fetched(entry2)

		job, err := suite.status.AtIndex(entry1.Height)
		assert.ErrorIs(suite.T(), err, storage.ErrNotFound)
		assert.Nil(suite.T(), job)
	})

	suite.Run("returns entry when next height is ready", func() {
		suite.reset(true)

		suite.status.Fetched(entry1)
		suite.status.Fetched(entry2)
		suite.status.Fetched(entry4) // leave a gap so the heap's not empty

		// Notify for entry1
		job, err := suite.status.AtIndex(entry1.Height)
		assert.NoErrorf(suite.T(), err, "error returned for job at index %d", entry1.Height)
		assertJobIsEntry(suite.T(), entry1, job)

		suite.status.Notified(entry1.Height)

		// Notify for entry2
		job, err = suite.status.AtIndex(entry2.Height)
		assert.NoErrorf(suite.T(), err, "error returned for job at index %d", entry2.Height)
		assertJobIsEntry(suite.T(), entry2, job)

		suite.status.Notified(entry2.Height)

		// Next entry isn't available
		job, err = suite.status.AtIndex(entry3.Height)
		assert.ErrorIs(suite.T(), err, storage.ErrNotFound)
		assert.Nil(suite.T(), job)
	})

	rand.Seed(time.Now().UnixNano())
	blockCount := 100
	expected := make([]uint64, blockCount)
	blocks := make(map[flow.Identifier]*status.BlockEntry, blockCount)
	for i := 0; i < blockCount; i++ {
		expected[i] = suite.rootHeight + 1 + uint64(i)
		for j := 0; j <= rand.Intn(3); j++ {
			entry := synctest.BlockEntryFixture(expected[i])
			blocks[entry.BlockID] = entry
		}
	}

	check := func(height uint64) {
		heights := []uint64{}
		for {
			job, err := suite.status.AtIndex(height)
			if errors.Is(err, storage.ErrNotFound) {
				break
			}
			assert.NoError(suite.T(), err)

			entry, err := status.JobToBlockEntry(job)
			assert.NoError(suite.T(), err)

			suite.status.Notified(entry.Height)

			heights = append(heights, entry.Height)
			height++
		}

		assert.Equal(suite.T(), expected, heights)
	}

	suite.Run("drops duplicate entries fetched in order", func() {
		suite.reset(true)

		for _, height := range expected {
			for j := 0; j <= rand.Intn(3); j++ {
				suite.status.Fetched(synctest.BlockEntryFixture(height))
			}
		}

		check(expected[0])
	})

	suite.Run("drops duplicate entries fetched in random order", func() {
		suite.reset(true)

		for _, entry := range blocks {
			suite.status.Fetched(entry)
		}

		check(expected[0])
	})
}

func (suite *StatusSuite) TestHead() {

	check := func(expected uint64) {
		height, err := suite.status.Head()
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), expected, height)
	}

	checkNotFound := func() {
		job, err := suite.status.Head()
		assert.ErrorIs(suite.T(), err, storage.ErrNotFound)
		assert.Zero(suite.T(), job)
	}

	suite.Run("returns NotFound when no pending notifications", func() {
		checkNotFound()

		// entry is not the next height
		entry := synctest.BlockEntryFixture(suite.rootHeight + 2)
		suite.status.Fetched(entry)

		checkNotFound()
	})

	suite.Run("returns height for pending notification", func() {
		entry1 := synctest.BlockEntryFixture(suite.rootHeight + 1)
		suite.status.Fetched(entry1)

		check(entry1.Height)

		entry2 := synctest.BlockEntryFixture(suite.rootHeight + 2)
		suite.status.Fetched(entry2)

		// Head always returns the next available height
		check(entry1.Height)

		_, err := suite.status.AtIndex(entry1.Height)
		assert.NoError(suite.T(), err)

		suite.status.Notified(entry1.Height)

		// Next available should be incremented
		check(entry2.Height)
	})
}

func (suite *StatusSuite) TestHalted() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		progress := bstorage.NewConsumerProgress(db, module.ConsumeProgressExecutionDataRequesterNotification)
		err := progress.InitProcessedIndex(suite.rootHeight)
		assert.NoError(suite.T(), err)

		s := status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
		err = s.Load()
		assert.NoError(suite.T(), err)

		haltErr := &status.RequesterHaltedError{
			ExecutionDataID: unittest.IdentifierFixture(),
			BlockID:         unittest.IdentifierFixture(),
			Height:          suite.rootHeight + 1,
			Err:             err,
		}

		suite.Run("halted defaults to nil", func() {
			assert.Nil(suite.T(), suite.status.Halted())
		})

		suite.Run("calling Halt sets halted to true", func() {
			s.Halt(haltErr)

			// saved error matches original error
			assert.Equal(suite.T(), haltErr, s.Halted())
		})

		suite.Run("halted persists across loads", func() {
			s = status.New(zerolog.Nop(), suite.maxCachedEntries, progress)
			err = s.Load()
			assert.NoError(suite.T(), err)

			// loaded error message matches original error message
			require.NotNil(suite.T(), s.Halted())
			assert.Equal(suite.T(), haltErr.Error(), s.Halted().Error())
		})
	})
}

func (suite *StatusSuite) TestNextNotificationHeight() {

	suite.Run("next height returns correct height for startblock", func() {
		assert.Equal(suite.T(), suite.rootHeight+1, suite.status.NextNotificationHeight())
	})

	suite.Run("next height returns correct height", func() {
		height := suite.rootHeight + 1
		expected := synctest.BlockEntryFixture(height)
		suite.status.Fetched(expected)
		suite.status.Fetched(synctest.BlockEntryFixture(height + 1))

		job, err := suite.status.AtIndex(height)
		assert.NoError(suite.T(), err)
		assertJobIsEntry(suite.T(), expected, job)

		suite.status.Notified(expected.Height)

		assert.Equal(suite.T(), expected.Height+1, suite.status.NextNotificationHeight())
	})

}

func assertJobIsEntry(t *testing.T, expected *status.BlockEntry, job module.Job) {
	require.NotNil(t, job)

	actual, err := status.JobToBlockEntry(job)
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}
