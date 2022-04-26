package jobs

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataReaderSuite struct {
	suite.Suite

	maxCachedEntries uint64

	jobs   []module.Job
	head   func() uint64
	read   ReadExecutionData
	reader *ExecutionDataReader
}

func TestExecutionDataReaderSuite(t *testing.T) {
	t.Parallel()
	rand.Seed(time.Now().UnixMilli())
	suite.Run(t, new(ExecutionDataReaderSuite))
}

func (suite *ExecutionDataReaderSuite) SetupTest() {
	suite.maxCachedEntries = uint64(10)

	jobCount := 10
	suite.jobs = make([]module.Job, jobCount)
	for i := 0; i < jobCount; i++ {
		blockID := unittest.IdentifierFixture()
		suite.jobs[i] = BlockEntryToJob(&BlockEntry{
			BlockID:       blockID,
			Height:        uint64(i),
			ExecutionData: ExecutionDataFixture(blockID),
		})
	}

	// defaults
	suite.head = func() uint64 { return uint64(jobCount - 1) }
	suite.read = func(ctx irrecoverable.SignalerContext, index uint64) (*state_synchronization.ExecutionData, error) {
		if int(index) >= len(suite.jobs) {
			return nil, storage.ErrNotFound
		}

		entry, err := JobToBlockEntry(suite.jobs[int(index)])
		require.NoError(suite.T(), err)
		return entry.ExecutionData, nil
	}

	suite.reset()
}

func (suite *ExecutionDataReaderSuite) reset() {
	head := func() uint64 {
		return suite.head()
	}

	read := func(ctx irrecoverable.SignalerContext, index uint64) (*state_synchronization.ExecutionData, error) {
		return suite.read(ctx, index)
	}

	suite.reader = NewExecutionDataReader(suite.maxCachedEntries, head, read)
}

func (suite *ExecutionDataReaderSuite) TestAtIndex() {
	suite.Run("returns not found when not yet ready", func() {
		suite.reset()
		// runTest not called, so reader is never started
		job, err := suite.reader.AtIndex(1)
		assert.Nil(suite.T(), job, "job should be nil")
		assert.Equal(suite.T(), storage.ErrNotFound, err, "expected not found error")
	})

	suite.Run("returns not found when shutting down", func() {
		suite.reset()
		suite.runTest(func() {})
		// runTest called and returns first, so reader has shutdown
		job, err := suite.reader.AtIndex(1)
		assert.Nil(suite.T(), job, "job should be nil")
		assert.Equal(suite.T(), storage.ErrNotFound, err, "expected not found error")
	})

	suite.Run("returns not found when index out of range", func() {
		suite.reset()
		suite.runTest(func() {
			job, err := suite.reader.AtIndex(suite.head() + 1)
			assert.Nil(suite.T(), job, "job should be nil")
			assert.Equal(suite.T(), storage.ErrNotFound, err, "expected not found error")
		})
	})

	suite.Run("serves from cache", func() {
		suite.reset()
		suite.runTest(func() {
			ed := ExecutionDataFixture(unittest.IdentifierFixture())

			index := uint64(1)
			suite.reader.cache[index] = ed

			job, err := suite.reader.AtIndex(index)
			require.NoError(suite.T(), err)

			entry, err := JobToBlockEntry(job)
			assert.NoError(suite.T(), err)

			assert.Equal(suite.T(), entry.ExecutionData, ed)
		})
	})

	suite.Run("calls read when not cached", func() {
		suite.reset()
		suite.runTest(func() {
			ed := ExecutionDataFixture(unittest.IdentifierFixture())

			suite.read = func(irrecoverable.SignalerContext, uint64) (*state_synchronization.ExecutionData, error) {
				return ed, nil
			}

			job, err := suite.reader.AtIndex(1)
			require.NoError(suite.T(), err)

			entry, err := JobToBlockEntry(job)
			assert.NoError(suite.T(), err)

			assert.Equal(suite.T(), entry.ExecutionData, ed)
		})
	})

	suite.Run("returns error from read", func() {
		suite.reset()
		suite.runTest(func() {
			expecteErr := errors.New("expected error: read failed")
			suite.read = func(irrecoverable.SignalerContext, uint64) (*state_synchronization.ExecutionData, error) {
				return nil, expecteErr
			}

			job, err := suite.reader.AtIndex(1)
			assert.Nil(suite.T(), job, "job should be nil")
			assert.Equal(suite.T(), expecteErr, err, "expected not found error")
		})
	})

	suite.Run("handles concurrent calls", func() {
		suite.reset()
		suite.runTest(func() {
			ed := ExecutionDataFixture(unittest.IdentifierFixture())
			jobID := JobID(ed.BlockID)

			index := uint64(1)
			suite.reader.cache[index] = ed

			wg := sync.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					job, err := suite.reader.AtIndex(index)
					assert.NoError(suite.T(), err)
					assert.Equal(suite.T(), jobID, job.ID())
				}()
			}
			wg.Wait()
		})
	})
}

func (suite *ExecutionDataReaderSuite) TestHead() {
	suite.runTest(func() {
		expectedIndex := uint64(15)
		suite.head = func() uint64 {
			return expectedIndex
		}
		index, err := suite.reader.Head()
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), expectedIndex, index)
	})
}

func (suite *ExecutionDataReaderSuite) TestCaching() {
	suite.Run("add index out of range doesn't cache", func() {
		suite.runTest(func() {
			ed := ExecutionDataFixture(unittest.IdentifierFixture())
			head := suite.reader.head()
			suite.reader.Add(head, ed)
			suite.reader.Add(head-1, ed)

			assert.Len(suite.T(), suite.reader.cache, 0, "expected 0 entries in cache")
		})
	})

	suite.Run("add index outside of cacheable range doesn't cache", func() {
		suite.reset()
		suite.runTest(func() {
			head := suite.reader.head()
			cacheData := map[uint64]*state_synchronization.ExecutionData{}
			for i := head; i < head+suite.maxCachedEntries+5; i++ {
				cacheData[i] = ExecutionDataFixture(unittest.IdentifierFixture())
				suite.reader.Add(i, cacheData[i])
			}

			// Should have exactly maxCachedEntries entries in cache
			assert.Len(suite.T(), suite.reader.cache, int(suite.maxCachedEntries), "expected %d entries in cache", suite.maxCachedEntries)

			// Should have cached entries (head, head + maxCachedEntries]
			for i := head + 1; i <= head+suite.maxCachedEntries; i++ {
				assert.Equal(suite.T(), cacheData[i], suite.reader.cache[i])
			}
		})
	})

	suite.Run("add/remove happy path", func() {
		suite.reset()
		suite.runTest(func() {
			head := suite.reader.head()
			for i := head + 1; i <= head+suite.maxCachedEntries; i++ {
				ed := ExecutionDataFixture(unittest.IdentifierFixture())

				suite.reader.Add(i, ed)
				assert.Len(suite.T(), suite.reader.cache, 1, "expected 1 entry in cache")

				assert.Equal(suite.T(), ed, suite.reader.cache[i])

				suite.reader.Remove(i)
				assert.Len(suite.T(), suite.reader.cache, 0, "expected 0 entries in cache")
			}

			assert.Len(suite.T(), suite.reader.cache, 0, "expected 0 entries in cache")
		})
	})

	suite.Run("add/remove concurrent", func() {
		suite.maxCachedEntries = 500
		suite.reset()
		suite.runTest(func() {
			workers := sync.WaitGroup{}
			head := suite.reader.head()
			for i := head + 1; i <= head+suite.maxCachedEntries; i++ {
				workers.Add(1)
				go func(index uint64) {
					defer workers.Done()

					ed := ExecutionDataFixture(unittest.IdentifierFixture())

					suite.reader.Add(index, ed)
					actual, has := suite.getCached(index)
					assert.True(suite.T(), has, "expected entry to be in cache")
					assert.Equal(suite.T(), ed, actual)

					suite.reader.Remove(index)
					_, has = suite.getCached(index)
					assert.False(suite.T(), has, "expected entry to be removed from cache")
				}(i)
			}

			workers.Wait()

			assert.Len(suite.T(), suite.reader.cache, 0, "expected 0 entries in cache")
		})
	})
}

func (suite *ExecutionDataReaderSuite) getCached(index uint64) (*state_synchronization.ExecutionData, bool) {
	suite.reader.mu.RLock()
	defer suite.reader.mu.RUnlock()

	ed, has := suite.reader.cache[index]
	return ed, has
}

//
//
//
//

func (suite *ExecutionDataReaderSuite) runTest(fn func()) {
	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	ctx, cancel := context.WithCancel(testCtx)

	signalCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(suite.T(), testCtx, errChan)

	suite.reader.Start(signalCtx)
	unittest.RequireCloseBefore(suite.T(), suite.reader.Ready(), 5*time.Millisecond, "timed out waiting for reader to be ready")

	fn()

	cancel()
	unittest.RequireCloseBefore(suite.T(), suite.reader.Done(), 5*time.Millisecond, "timed out waiting for reader to be done")
}

func irrecoverableNotExpected(t *testing.T, ctx context.Context, errChan <-chan error) {
	select {
	case <-ctx.Done():
		return
	case err := <-errChan:
		require.NoError(t, err, "unexpected irrecoverable error")
	}
}

func JobIDAtIndex(index uint64) module.JobID {
	return module.JobID(fmt.Sprintf("%v", index))
}

func JobIDToIndex(id module.JobID) (uint64, error) {
	return strconv.ParseUint(string(id), 10, 64)
}

func ExecutionDataFixture(blockID flow.Identifier) *state_synchronization.ExecutionData {
	return &state_synchronization.ExecutionData{
		BlockID:     blockID,
		Collections: []*flow.Collection{},
		Events:      []flow.EventsList{},
		TrieUpdates: []*ledger.TrieUpdate{},
	}
}
