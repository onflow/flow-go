package timeoutaggregator

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/gammazero/workerpool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/utils/unittest"
)

var factoryError = errors.New("factory error")

func TestTimeoutCollectors(t *testing.T) {
	suite.Run(t, new(TimeoutCollectorsTestSuite))
}

// TimeoutCollectorsTestSuite is a test suite for isolated testing of TimeoutCollectors.
// Contains helper methods and mocked state which is used to verify correct behavior of TimeoutCollectors.
type TimeoutCollectorsTestSuite struct {
	suite.Suite

	mockedCollectors map[uint64]*mocks.TimeoutCollector
	factoryMethod    NewCollectorFactoryMethod
	collectors       *TimeoutCollectors
	lowestView      uint64
	workerPool       *workerpool.WorkerPool
}

func (s *TimeoutCollectorsTestSuite) SetupTest() {
	s.lowestLevel = 1000
	s.mockedCollectors = make(map[uint64]*mocks.TimeoutCollector)
	s.workerPool = workerpool.New(2)
	s.factoryMethod = func(view uint64) (hotstuff.TimeoutCollector, error) {
		if collector, found := s.mockedCollectors[view]; found {
			return collector, nil
		}
		return nil, fmt.Errorf("mocked collector %v not found: %w", view, factoryError)
	}
	s.collectors = NewTimeoutCollectors(unittest.Logger(), s.lowestLevel, s.factoryMethod)
}

func (s *TimeoutCollectorsTestSuite) TearDownTest() {
	s.workerPool.StopWait()
}

// prepareMockedCollector prepares a mocked collector and stores it in map, later it will be used
// to mock behavior of timeout collectors.
func (s *TimeoutCollectorsTestSuite) prepareMockedCollector(view uint64) *mocks.TimeoutCollector {
	collector := mocks.NewTimeoutCollector(s.T())
	collector.On("View").Return(view).Maybe()
	s.mockedCollectors[view] = collector
	return collector
}

// TestGetOrCreatorCollector_ViewLowerThanLowest tests a scenario where caller tries to create a collector with view
// lower than already pruned one. This should result in sentinel error `DecreasingPruningHeightError`
func (s *TimeoutCollectorsTestSuite) TestGetOrCreatorCollector_ViewLowerThanLowest() {
	collector, created, err := s.collectors.GetOrCreateCollector(s.lowestLevel - 10)
	require.Nil(s.T(), collector)
	require.False(s.T(), created)
	require.Error(s.T(), err)
	require.True(s.T(), mempool.IsDecreasingPruningHeightError(err))
}

// TestGetOrCreateCollector_ValidCollector tests a happy path scenario where we try first to create and then retrieve cached collector.
func (s *TimeoutCollectorsTestSuite) TestGetOrCreateCollector_ValidCollector() {
	view := s.lowestLevel + 10
	s.prepareMockedCollector(view)
	collector, created, err := s.collectors.GetOrCreateCollector(view)
	require.NoError(s.T(), err)
	require.True(s.T(), created)
	require.Equal(s.T(), view, collector.View())

	cached, cachedCreated, err := s.collectors.GetOrCreateCollector(view)
	require.NoError(s.T(), err)
	require.False(s.T(), cachedCreated)
	require.Equal(s.T(), collector, cached)
}

// TestGetOrCreateCollector_FactoryError tests that error from factory method is propagated to caller.
func (s *TimeoutCollectorsTestSuite) TestGetOrCreateCollector_FactoryError() {
	// creating collector without calling prepareMockedCollector will yield factoryError.
	collector, created, err := s.collectors.GetOrCreateCollector(s.lowestLevel + 10)
	require.Nil(s.T(), collector)
	require.False(s.T(), created)
	require.ErrorIs(s.T(), err, factoryError)
}

// TestGetOrCreateCollectors_ConcurrentAccess tests that concurrently accessing of GetOrCreateCollector creates
// only one collector and all other instances are retrieved from cache.
func (s *TimeoutCollectorsTestSuite) TestGetOrCreateCollectors_ConcurrentAccess() {
	createdTimes := atomic.NewUint64(0)
	view := s.lowestLevel + 10
	s.prepareMockedCollector(view)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			_, created, err := s.collectors.GetOrCreateCollector(view)
			require.NoError(s.T(), err)
			if created {
				createdTimes.Add(1)
			}
			wg.Done()
		}()
	}

	unittest.AssertReturnsBefore(s.T(), wg.Wait, time.Second)
	require.Equal(s.T(), uint64(1), createdTimes.Load())
}

// TestPruneUpToView tests pruning removes item below pruning height and leaves unmodified other items.
func (s *TimeoutCollectorsTestSuite) TestPruneUpToView() {
	numberOfCollectors := uint64(10)
	prunedViews := make([]uint64, 0)
	for i := uint64(0); i < numberOfCollectors; i++ {
		view := s.lowestLevel + i
		s.prepareMockedCollector(view)
		_, _, err := s.collectors.GetOrCreateCollector(view)
		require.NoError(s.T(), err)
		prunedViews = append(prunedViews, view)
	}

	pruningHeight := s.lowestLevel + numberOfCollectors

	expectedCollectors := make([]hotstuff.TimeoutCollector, 0)
	for i := uint64(0); i < numberOfCollectors; i++ {
		view := pruningHeight + i
		s.prepareMockedCollector(view)
		collector, _, err := s.collectors.GetOrCreateCollector(view)
		require.NoError(s.T(), err)
		expectedCollectors = append(expectedCollectors, collector)
	}

	// after this operation collectors below pruning height should be pruned and everything higher
	// should be left unmodified
	s.collectors.PruneUpToView(pruningHeight)

	for _, prunedView := range prunedViews {
		_, _, err := s.collectors.GetOrCreateCollector(prunedView)
		require.Error(s.T(), err)
		require.True(s.T(), mempool.IsDecreasingPruningHeightError(err))
	}

	for _, collector := range expectedCollectors {
		cached, _, _ := s.collectors.GetOrCreateCollector(collector.View())
		require.Equal(s.T(), collector, cached)
	}
}
