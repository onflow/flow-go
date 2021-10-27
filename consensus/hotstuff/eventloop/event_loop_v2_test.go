package eventloop

import (
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEventLoopV2 performs unit testing of event loop, checks if submitted events are propagated
// to event handler as well as handling of timeouts.
func TestEventLoopV2(t *testing.T) {
	suite.Run(t, new(EventLoopV2TestSuite))
}

type EventLoopV2TestSuite struct {
	suite.Suite

	eh *mocks.EventHandlerV2

	eventLoop *EventLoopV2
}

func (s *EventLoopV2TestSuite) SetupTest() {
	s.eh = &mocks.EventHandlerV2{}
	s.eh.On("Start").Return(nil).Maybe()
	s.eh.On("TimeoutChannel").Return(time.NewTimer(10 * time.Second).C).Maybe()
	s.eh.On("OnLocalTimeout").Return(nil).Maybe()

	log := zerolog.New(ioutil.Discard)

	eventLoop, err := NewEventLoopV2(log, metrics.NewNoopCollector(), s.eh)
	require.NoError(s.T(), err)
	s.eventLoop = eventLoop
	<-eventLoop.Ready()
}

func (s *EventLoopV2TestSuite) TearDownTest() {
	<-s.eventLoop.Done()
}

// TestReadyDone tests if event loop stops internal worker thread
func (s *EventLoopV2TestSuite) TestReadyDone() {
	time.Sleep(1 * time.Second)
	done := atomic.NewBool(false)
	go func() {
		<-s.eventLoop.Done()
		done.Store(true)
	}()

	require.Eventually(s.T(), done.Load, time.Millisecond*100, time.Millisecond*10)
}

// Test_SubmitQC tests that submitted proposal is eventually sent to event handler for processing
func (s *EventLoopV2TestSuite) Test_SubmitProposal() {
	proposal := unittest.BlockHeaderFixture()
	expectedProposal := model.ProposalFromFlow(&proposal, proposal.View-1)
	processed := atomic.NewBool(false)
	s.eh.On("OnReceiveProposal", expectedProposal).Run(func(args mock.Arguments) {
		processed.Store(true)
	}).Return(nil).Once()
	s.eventLoop.SubmitProposal(&proposal, proposal.View-1)
	require.Eventually(s.T(), processed.Load, time.Millisecond*100, time.Millisecond*10)
	s.eh.AssertExpectations(s.T())
}

// Test_SubmitQC tests that submitted QC is eventually sent to event handler for processing
func (s *EventLoopV2TestSuite) Test_SubmitQC() {
	qc := unittest.QuorumCertificateFixture()
	processed := atomic.NewBool(false)
	s.eh.On("OnQCConstructed", qc).Run(func(args mock.Arguments) {
		processed.Store(true)
	}).Return(nil).Once()
	s.eventLoop.SubmitTrustedQC(qc)
	require.Eventually(s.T(), processed.Load, time.Millisecond*100, time.Millisecond*10)
	s.eh.AssertExpectations(s.T())
}

// TestEventLoopV2_Timeout tests that event loop delivers timeout events to event handler under pressure
func TestEventLoopV2_Timeout(t *testing.T) {
	eh := &mocks.EventHandlerV2{}
	processed := atomic.NewBool(false)
	eh.On("Start").Return(nil).Once()
	eh.On("TimeoutChannel").Return(time.NewTimer(100 * time.Millisecond).C)
	eh.On("OnQCConstructed", mock.Anything).Return(nil).Maybe()
	eh.On("OnReceiveProposal", mock.Anything).Return(nil).Maybe()
	eh.On("OnLocalTimeout").Run(func(args mock.Arguments) {
		processed.Store(true)
	}).Return(nil).Once()

	log := zerolog.New(ioutil.Discard)

	eventLoop, err := NewEventLoopV2(log, metrics.NewNoopCollector(), eh)
	require.NoError(t, err)

	<-eventLoop.Ready()

	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)

	// spam with proposals and QCs
	go func() {
		defer wg.Done()
		for !processed.Load() {
			qc := unittest.QuorumCertificateFixture()
			eventLoop.SubmitTrustedQC(qc)
		}
	}()

	go func() {
		defer wg.Done()
		for !processed.Load() {
			proposal := unittest.BlockHeaderFixture()
			eventLoop.SubmitProposal(&proposal, proposal.View-1)
		}
	}()

	require.Eventually(t, processed.Load, time.Millisecond*200, time.Millisecond*10)
	wg.Wait()
	<-eventLoop.Done()
}
