package timeoutcollector

import (
	"errors"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestTimeoutCollector(t *testing.T) {
	suite.Run(t, new(TimeoutCollectorTestSuite))
}

type TimeoutCollectorTestSuite struct {
	suite.Suite

	view                   uint64
	notifier               *mocks.Consumer
	processor              *mocks.TimeoutProcessor
	onNewQCDiscoveredState mock.Mock
	onNewTCDiscoveredState mock.Mock
	collector              *TimeoutCollector
}

func (s *TimeoutCollectorTestSuite) SetupTest() {
	s.view = 1000
	s.notifier = &mocks.Consumer{}
	s.processor = &mocks.TimeoutProcessor{}

	s.onNewQCDiscoveredState.On("onNewQCDiscovered", mock.Anything).Maybe()
	s.onNewTCDiscoveredState.On("onNewTCDiscovered", mock.Anything).Maybe()

	s.collector = NewTimeoutCollector(s.view, s.notifier, s.processor, s.onNewQCDiscovered, s.onNewTCDiscovered)

}

// onQCCreated is a special function that registers call in mocked state.
// ATTENTION: don't change name of this function since the same name is used in:
// s.onNewQCDiscoveredState.On("onNewQCDiscovered") statements
func (s *TimeoutCollectorTestSuite) onNewQCDiscovered(qc *flow.QuorumCertificate) {
	s.onNewQCDiscoveredState.Called(qc)
}

// onNewTCDiscovered is a special function that registers call in mocked state.
// ATTENTION: don't change name of this function since the same name is used in:
// s.onNewTCDiscoveredState.On("onNewTCDiscovered") statements
func (s *TimeoutCollectorTestSuite) onNewTCDiscovered(tc *flow.TimeoutCertificate) {
	s.onNewTCDiscoveredState.Called(tc)
}

func (s *TimeoutCollectorTestSuite) TestView() {
	require.Equal(s.T(), s.view, s.collector.View())
}

// Test_AddTimeoutHappyPath tests that process in happy path executed by multiple workers deliver expected results
// all operations should be successful, no errors expected
func (s *TimeoutCollectorTestSuite) TestAddTimeout_HappyPath() {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view))
			s.processor.On("Process", timeout).Return(nil).Once()
			err := s.collector.AddTimeout(timeout)
			require.NoError(s.T(), err)
		}()
	}

	wg.Wait()
	s.processor.AssertExpectations(s.T())
}

func (s *TimeoutCollectorTestSuite) TestAddTimeout_DoubleTimeout() {
	timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view))
	s.processor.On("Process", timeout).Return(nil).Once()
	err := s.collector.AddTimeout(timeout)
	require.NoError(s.T(), err)

	otherTimeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutObjectSignerID(timeout.SignerID))

	s.notifier.On("OnDoubleTimeoutDetected", timeout, otherTimeout).Once()

	err = s.collector.AddTimeout(otherTimeout)
	require.NoError(s.T(), err)
	s.notifier.AssertCalled(s.T(), "OnDoubleTimeoutDetected", timeout, otherTimeout)
	s.processor.AssertNumberOfCalls(s.T(), "Process", 1)
}

func (s *TimeoutCollectorTestSuite) TestAddTimeout_RepeatedTimeout() {
	timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view))
	s.processor.On("Process", timeout).Return(nil).Once()
	err := s.collector.AddTimeout(timeout)
	require.NoError(s.T(), err)
	err = s.collector.AddTimeout(timeout)
	require.NoError(s.T(), err)
	s.processor.AssertNumberOfCalls(s.T(), "Process", 1)
}

func (s *TimeoutCollectorTestSuite) TestAddTimeout_TimeoutCacheException() {
	// incompatible view is an exception and not handled by timeout collector
	timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view + 1))
	err := s.collector.AddTimeout(timeout)
	require.ErrorAs(s.T(), err, &ErrTimeoutForIncompatibleView)
	s.processor.AssertNotCalled(s.T(), "Process")
}

func (s *TimeoutCollectorTestSuite) TestAddTimeout_InvalidTimeout() {
	s.Run("invalid-timeout", func() {
		timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view))
		s.processor.On("Process", timeout).Return(model.NewInvalidTimeoutErrorf(timeout, "")).Once()
		s.notifier.On("OnInvalidTimeoutDetected", timeout).Once()
		err := s.collector.AddTimeout(timeout)
		require.NoError(s.T(), err)

		s.notifier.AssertCalled(s.T(), "OnInvalidTimeoutDetected", timeout)
	})
	s.Run("process-exception", func() {
		exception := errors.New("invalid-signature")
		timeout := helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view))
		s.processor.On("Process", timeout).Return(exception).Once()
		err := s.collector.AddTimeout(timeout)
		require.ErrorAs(s.T(), err, &exception)
	})
}

func (s *TimeoutCollectorTestSuite) TestAddTimeout_TONotifications() {
	qcCount := 100
	// generate QCs with increasing view numbers
	if s.view < uint64(qcCount) {
		s.T().Fatal("invalid test configuration")
	}

	s.onNewQCDiscoveredState = mock.Mock{}
	s.onNewTCDiscoveredState = mock.Mock{}

	var highestReportedQC *flow.QuorumCertificate
	s.onNewQCDiscoveredState.On("onNewQCDiscovered", mock.Anything).Run(func(args mock.Arguments) {
		qc := args.Get(0).(*flow.QuorumCertificate)
		if highestReportedQC == nil || highestReportedQC.View < qc.View {
			highestReportedQC = qc
		}
	})

	lastViewTC := helper.MakeTC(helper.WithTCView(s.view - 1))
	s.onNewTCDiscoveredState.On("onNewTCDiscovered", lastViewTC).Once()

	timeouts := make([]*model.TimeoutObject, 0, qcCount)
	for i := 0; i < qcCount; i++ {
		qc := helper.MakeQC(helper.WithQCView(uint64(i)))
		timeout := helper.TimeoutObjectFixture(func(timeout *model.TimeoutObject) {
			timeout.View = s.view
			timeout.HighestQC = qc
			timeout.LastViewTC = lastViewTC
		})
		timeouts = append(timeouts, timeout)
		s.processor.On("Process", timeout).Return(nil).Once()
	}

	expectedHighestQC := timeouts[len(timeouts)-1].HighestQC

	// shuffle timeouts in random order
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(timeouts), func(i, j int) {
		timeouts[i], timeouts[j] = timeouts[j], timeouts[i]
	})

	var wg sync.WaitGroup
	wg.Add(len(timeouts))
	for _, timeout := range timeouts {
		go func(timeout *model.TimeoutObject) {
			defer wg.Done()
			err := s.collector.AddTimeout(timeout)
			require.NoError(s.T(), err)
		}(timeout)
	}
	wg.Wait()

	s.onNewTCDiscoveredState.AssertCalled(s.T(), "onNewTCDiscovered", lastViewTC)
	require.Equal(s.T(), expectedHighestQC, highestReportedQC)
}
