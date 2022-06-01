package timeoutaggregator

import (
	"context"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"
	"sync"
	"testing"
	"time"
)

func TestTimeoutAggregator(t *testing.T) {
	suite.Run(t, new(TimeoutAggregatorTestSuite))
}

// TimeoutAggregatorTestSuite is a test suite for isolated testing of TimeoutAggregator.
// Contains mocked state which is used to verify correct behavior of TimeoutAggregator.
// Automatically starts and stops module.Startable in SetupTest and TearDownTest respectively.
type TimeoutAggregatorTestSuite struct {
	suite.Suite

	lowestRetainedView uint64
	aggregator         *TimeoutAggregator
	collectors         *mocks.TimeoutCollectors
	consumer           *mocks.Consumer
	stopAggregator     context.CancelFunc
}

func (s *TimeoutAggregatorTestSuite) SetupTest() {
	var err error
	s.collectors = mocks.NewTimeoutCollectors(s.T())
	s.consumer = mocks.NewConsumer(s.T())

	s.lowestRetainedView = 100

	s.aggregator, err = NewTimeoutAggregator(unittest.Logger(), s.consumer, s.lowestRetainedView, s.collectors)
	require.NoError(s.T(), err)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)
	s.stopAggregator = cancel
	s.aggregator.Start(signalerCtx)
	unittest.RequireCloseBefore(s.T(), s.aggregator.Ready(), 100*time.Millisecond, "should close before timeout")
}

func (s *TimeoutAggregatorTestSuite) TearDownTest() {
	s.stopAggregator()
	unittest.RequireCloseBefore(s.T(), s.aggregator.Done(), time.Second, "should close before timeout")
}

// TestAddTimeout_HappyPath tests a happy path when multiple threads are adding timeouts for processing
// Eventually every timeout has to be processed by TimeoutCollector
func (s *TimeoutAggregatorTestSuite) TestAddTimeout_HappyPath() {
	timeoutsCount := 20
	collector := mocks.NewTimeoutCollector(s.T())
	callCount := atomic.NewUint64(0)
	collector.On("AddTimeout", mock.Anything).Run(func(mock.Arguments) {
		callCount.Add(1)
	}).Return(nil).Times(timeoutsCount)
	s.collectors.On("GetOrCreateCollector", s.lowestRetainedView).Return(collector, true, nil).Times(timeoutsCount)

	var wg, startWg sync.WaitGroup
	wg.Add(timeoutsCount)
	startWg.Add(1)
	for i := 0; i < timeoutsCount; i++ {
		go func() {
			defer wg.Done()
			timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.lowestRetainedView))
			s.aggregator.AddTimeout(timeout)
		}()
	}

	require.Eventually(s.T(), func() bool {
		return callCount.Load() == uint64(timeoutsCount)
	}, time.Second, time.Millisecond*20)
}

// TestAddTimeout_DoubleTimeout tests if double timeout is reported to notifier when returned by TimeoutCollector
func (s *TimeoutAggregatorTestSuite) TestAddTimeout_DoubleTimeout() {
	collector := mocks.NewTimeoutCollector(s.T())
	timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.lowestRetainedView))
	collector.On("AddTimeout", timeout).Return(model.NewDoubleTimeoutErrorf(timeout, timeout, "")).Once()
	s.collectors.On("GetOrCreateCollector", s.lowestRetainedView).Return(collector, true, nil).Once()
	s.consumer.On("OnDoubleTimeoutDetected", mock.Anything, mock.Anything).Once()
	s.aggregator.AddTimeout(timeout)

	require.Eventually(s.T(),
		func() bool {
			return s.consumer.AssertCalled(s.T(), "OnDoubleTimeoutDetected", timeout, timeout)
		}, time.Second, time.Millisecond*20)
}

// TestPruneUpToView tests that pruning removes collectors lower that retained view
func (s *TimeoutAggregatorTestSuite) TestPruneUpToView() {
	// try pruning with lower view than already set
	s.aggregator.PruneUpToView(s.lowestRetainedView)
	s.collectors.AssertNotCalled(s.T(), "PruneUpToView")

	s.collectors.On("PruneUpToView", s.lowestRetainedView+1).Once()
	s.aggregator.PruneUpToView(s.lowestRetainedView + 1)
}
