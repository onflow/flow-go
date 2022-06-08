package approvals

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRequestTracker performs isolated testing of RequestTracker.
// RequestTracker has to lazy initialize items.
// RequestTracker has to properly update items when they are in non-blackout period.
// RequestTracker has to properly prune outdated items by height
func TestRequestTracker(t *testing.T) {
	suite.Run(t, new(RequestTrackerTestSuite))
}

type RequestTrackerTestSuite struct {
	suite.Suite
	headers *mockstorage.Headers

	tracker *RequestTracker
}

func (s *RequestTrackerTestSuite) SetupTest() {
	s.headers = &mockstorage.Headers{}
	s.tracker = NewRequestTracker(s.headers, 1, 3)
}

// TestTryUpdate_CreateAndUpdate tests that tracker item is lazy initialized and successfully
// updated when blackout period has passed.
func (s *RequestTrackerTestSuite) TestTryUpdate_CreateAndUpdate() {
	executedBlock := unittest.BlockFixture()
	s.headers.On("ByBlockID", executedBlock.ID()).Return(executedBlock.Header, nil)
	result := unittest.ExecutionResultFixture(unittest.WithBlock(&executedBlock))
	chunks := 5
	for i := 0; i < chunks; i++ {
		_, updated, err := s.tracker.TryUpdate(result, executedBlock.ID(), uint64(i))
		require.NoError(s.T(), err)
		require.False(s.T(), updated)
	}

	// wait for maximum blackout period
	time.Sleep(time.Second * 3)

	for i := 0; i < chunks; i++ {
		item, updated, err := s.tracker.TryUpdate(result, executedBlock.ID(), uint64(i))
		require.NoError(s.T(), err)
		require.True(s.T(), updated)
		require.Equal(s.T(), uint(1), item.Requests)
	}
}

// TestTryUpdate_ConcurrentTracking tests that TryUpdate behaves correctly under concurrent updates
func (s *RequestTrackerTestSuite) TestTryUpdate_ConcurrentTracking() {
	s.tracker.blackoutPeriodMax = 0
	s.tracker.blackoutPeriodMin = 0

	executedBlock := unittest.BlockFixture()
	s.headers.On("ByBlockID", executedBlock.ID()).Return(executedBlock.Header, nil)
	result := unittest.ExecutionResultFixture(unittest.WithBlock(&executedBlock))
	chunks := 5
	var wg sync.WaitGroup
	for times := 0; times < 10; times++ {
		wg.Add(1)
		go func() {
			for i := 0; i < chunks; i++ {
				_, updated, err := s.tracker.TryUpdate(result, executedBlock.ID(), uint64(i))
				require.NoError(s.T(), err)
				require.True(s.T(), updated)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	for i := 0; i < chunks; i++ {
		tracker, ok := s.tracker.index[result.ID()][executedBlock.ID()][uint64(i)]
		require.True(s.T(), ok)
		require.Equal(s.T(), uint(10), tracker.Requests)
	}
}

// TestTryUpdate_UpdateForInvalidResult tests that submitting ER which is referencing invalid block
// results in error.
func (s *RequestTrackerTestSuite) TestTryUpdate_UpdateForInvalidResult() {
	executedBlock := unittest.BlockFixture()
	s.headers.On("ByBlockID", executedBlock.ID()).Return(nil, storage.ErrNotFound)
	result := unittest.ExecutionResultFixture(unittest.WithBlock(&executedBlock))
	_, updated, err := s.tracker.TryUpdate(result, executedBlock.ID(), uint64(0))
	require.Error(s.T(), err)
	require.False(s.T(), updated)
}

// TestTryUpdate_UpdateForPrunedHeight tests that request tracker doesn't accept items for execution results
// that are lower than our lowest height.
func (s *RequestTrackerTestSuite) TestTryUpdate_UpdateForPrunedHeight() {
	executedBlock := unittest.BlockFixture()
	s.headers.On("ByBlockID", executedBlock.ID()).Return(executedBlock.Header, nil)
	err := s.tracker.PruneUpToHeight(executedBlock.Header.Height + 1)
	require.NoError(s.T(), err)
	result := unittest.ExecutionResultFixture(unittest.WithBlock(&executedBlock))
	_, updated, err := s.tracker.TryUpdate(result, executedBlock.ID(), uint64(0))
	require.Error(s.T(), err)
	require.True(s.T(), mempool.IsDecreasingPruningHeightError(err))
	require.False(s.T(), updated)
}

// TestPruneUpToHeight_Pruning tests that pruning up to height some height correctly removes needed items
func (s *RequestTrackerTestSuite) TestPruneUpToHeight_Pruning() {
	executedBlock := unittest.BlockFixture()
	nextExecutedBlock := unittest.BlockWithParentFixture(executedBlock.Header)
	s.headers.On("ByBlockID", executedBlock.ID()).Return(executedBlock.Header, nil)
	s.headers.On("ByBlockID", nextExecutedBlock.ID()).Return(nextExecutedBlock.Header, nil)

	result := unittest.ExecutionResultFixture(unittest.WithBlock(&executedBlock))
	nextResult := unittest.ExecutionResultFixture(unittest.WithBlock(nextExecutedBlock))

	for _, r := range []*flow.ExecutionResult{result, nextResult} {
		_, updated, err := s.tracker.TryUpdate(r, executedBlock.ID(), uint64(0))
		require.NoError(s.T(), err)
		require.False(s.T(), updated)
	}

	err := s.tracker.PruneUpToHeight(nextExecutedBlock.Header.Height)
	require.NoError(s.T(), err)

	_, ok := s.tracker.index[result.ID()]
	require.False(s.T(), ok)
	_, ok = s.tracker.index[nextResult.ID()]
	require.True(s.T(), ok)
}

// TestPruneUpToHeight_PruningWithDecreasingHeight tests that pruning with decreasing height results in error
func (s *RequestTrackerTestSuite) TestPruneUpToHeight_PruningWithDecreasingHeight() {
	height := uint64(100)
	err := s.tracker.PruneUpToHeight(height + 1)
	require.NoError(s.T(), err)
	err = s.tracker.PruneUpToHeight(height)
	require.Error(s.T(), err)
	require.True(s.T(), mempool.IsDecreasingPruningHeightError(err))
}
