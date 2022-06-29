package timeoutaggregator

import (
	"context"
	"github.com/onflow/flow-go/model/flow"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
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
	highestKnownView   uint64
	aggregator         *TimeoutAggregator
	collectors         *mocks.TimeoutCollectors
	committee          *mocks.Replicas
	consumer           *mocks.Consumer
	stopAggregator     context.CancelFunc
}

func (s *TimeoutAggregatorTestSuite) SetupTest() {
	var err error
	s.collectors = mocks.NewTimeoutCollectors(s.T())
	s.consumer = mocks.NewConsumer(s.T())
	s.committee = mocks.NewReplicas(s.T())

	s.lowestRetainedView = 100

	s.committee.On("LeaderForView", mock.Anything).Return(flow.Identifier{}, nil).Maybe()

	s.aggregator, err = NewTimeoutAggregator(unittest.Logger(), s.consumer, s.lowestRetainedView, s.committee, s.collectors)
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

// TestAddTimeout_EpochUnknown tests if timeout objects targeting unknown epoch should be ignored
func (s *TimeoutAggregatorTestSuite) TestAddTimeout_EpochUnknown() {
	*s.committee = *mocks.NewReplicas(s.T())
	timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.lowestRetainedView))
	s.committee.On("LeaderForView", timeout.View).Return(nil, model.ErrViewForUnknownEpoch).Once()
	s.aggregator.AddTimeout(timeout)
	require.Eventually(s.T(),
		func() bool {
			return s.committee.AssertCalled(s.T(), "LeaderForView", timeout.View)
		}, time.Second, time.Millisecond*20)
	s.collectors.AssertNotCalled(s.T(), "GetOrCreateCollector")
}

// TestPruneUpToView tests that pruning removes collectors lower that retained view
func (s *TimeoutAggregatorTestSuite) TestPruneUpToView() {
	// try pruning with lower view than already set
	s.aggregator.PruneUpToView(s.lowestRetainedView)
	s.collectors.AssertNotCalled(s.T(), "PruneUpToView")

	s.collectors.On("PruneUpToView", s.lowestRetainedView+1).Once()
	s.aggregator.PruneUpToView(s.lowestRetainedView + 1)
}

// TestOnEnteringView tests if entering view event gets processed when send through `TimeoutAggregator`.
// Tests the whole processing pipeline.
func (s *TimeoutAggregatorTestSuite) TestOnEnteringView() {
	view := s.lowestRetainedView + 1
	s.collectors.On("PruneUpToView", view).Once()
	s.aggregator.OnEnteringView(view, unittest.IdentifierFixture())
	require.Eventually(s.T(),
		func() bool {
			return s.collectors.AssertCalled(s.T(), "PruneUpToView", view)
		}, time.Second, time.Millisecond*20)
}
