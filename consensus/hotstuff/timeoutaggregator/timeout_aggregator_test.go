package timeoutaggregator

import (
	"context"
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
	"github.com/onflow/flow-go/module/metrics"
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
	stopAggregator     context.CancelFunc
}

func (s *TimeoutAggregatorTestSuite) SetupTest() {
	var err error
	s.collectors = mocks.NewTimeoutCollectors(s.T())

	s.lowestRetainedView = 100

	metricsCollector := metrics.NewNoopCollector()

	s.aggregator, err = NewTimeoutAggregator(
		unittest.Logger(),
		metricsCollector,
		metricsCollector,
		metricsCollector,
		s.lowestRetainedView,
		s.collectors,
	)
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

	var start sync.WaitGroup
	start.Add(timeoutsCount)
	for i := 0; i < timeoutsCount; i++ {
		go func() {
			timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.lowestRetainedView))

			start.Done()
			// Wait for last worker routine to signal ready. Then,
			// feed all timeouts into cache
			start.Wait()

			s.aggregator.AddTimeout(timeout)
		}()
	}

	start.Wait()

	require.Eventually(s.T(), func() bool {
		return callCount.Load() == uint64(timeoutsCount)
	}, time.Second, time.Millisecond*20)
}

// TestAddTimeout_EpochUnknown tests if timeout objects targeting unknown epoch should be ignored
func (s *TimeoutAggregatorTestSuite) TestAddTimeout_EpochUnknown() {
	timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.lowestRetainedView))
	*s.collectors = *mocks.NewTimeoutCollectors(s.T())
	done := make(chan struct{})
	s.collectors.On("GetOrCreateCollector", timeout.View).Return(nil, false, model.ErrViewForUnknownEpoch).Run(func(args mock.Arguments) {
		close(done)
	}).Once()
	s.aggregator.AddTimeout(timeout)
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

// TestPruneUpToView tests that pruning removes collectors lower that retained view
func (s *TimeoutAggregatorTestSuite) TestPruneUpToView() {
	s.collectors.On("PruneUpToView", s.lowestRetainedView+1).Once()
	s.aggregator.PruneUpToView(s.lowestRetainedView + 1)
}

// TestOnQcTriggeredViewChange tests if entering view event gets processed when send through `TimeoutAggregator`.
// Tests the whole processing pipeline.
func (s *TimeoutAggregatorTestSuite) TestOnQcTriggeredViewChange() {
	done := make(chan struct{})
	s.collectors.On("PruneUpToView", s.lowestRetainedView+1).Run(func(args mock.Arguments) {
		close(done)
	}).Once()
	qc := helper.MakeQC(helper.WithQCView(s.lowestRetainedView))
	s.aggregator.OnViewChange(qc.View, qc.View+1)
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

// TestOnTcTriggeredViewChange tests if entering view event gets processed when send through `TimeoutAggregator`.
// Tests the whole processing pipeline.
func (s *TimeoutAggregatorTestSuite) TestOnTcTriggeredViewChange() {
	view := s.lowestRetainedView + 1
	done := make(chan struct{})
	s.collectors.On("PruneUpToView", view).Run(func(args mock.Arguments) {
		close(done)
	}).Once()
	tc := helper.MakeTC(helper.WithTCView(s.lowestRetainedView))
	s.aggregator.OnViewChange(tc.View, tc.View+1)
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}
