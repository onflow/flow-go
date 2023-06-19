package eventloop

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEventLoop performs unit testing of event loop, checks if submitted events are propagated
// to event handler as well as handling of timeouts.
func TestEventLoop(t *testing.T) {
	suite.Run(t, new(EventLoopTestSuite))
}

type EventLoopTestSuite struct {
	suite.Suite

	eh     *mocks.EventHandler
	cancel context.CancelFunc

	eventLoop *EventLoop
}

func (s *EventLoopTestSuite) SetupTest() {
	s.eh = mocks.NewEventHandler(s.T())
	s.eh.On("Start", mock.Anything).Return(nil).Maybe()
	s.eh.On("TimeoutChannel").Return(make(<-chan time.Time, 1)).Maybe()
	s.eh.On("OnLocalTimeout").Return(nil).Maybe()

	log := zerolog.New(io.Discard)

	eventLoop, err := NewEventLoop(log, metrics.NewNoopCollector(), metrics.NewNoopCollector(), s.eh, time.Time{})
	require.NoError(s.T(), err)
	s.eventLoop = eventLoop

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	s.eventLoop.Start(signalerCtx)
	unittest.RequireCloseBefore(s.T(), s.eventLoop.Ready(), 100*time.Millisecond, "event loop not started")
}

func (s *EventLoopTestSuite) TearDownTest() {
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.eventLoop.Done(), 100*time.Millisecond, "event loop not stopped")
}

// TestReadyDone tests if event loop stops internal worker thread
func (s *EventLoopTestSuite) TestReadyDone() {
	time.Sleep(1 * time.Second)
	go func() {
		s.cancel()
	}()

	unittest.RequireCloseBefore(s.T(), s.eventLoop.Done(), 100*time.Millisecond, "event loop not stopped")
}

// Test_SubmitQC tests that submitted proposal is eventually sent to event handler for processing
func (s *EventLoopTestSuite) Test_SubmitProposal() {
	proposal := helper.MakeProposal()
	processed := atomic.NewBool(false)
	s.eh.On("OnReceiveProposal", proposal).Run(func(args mock.Arguments) {
		processed.Store(true)
	}).Return(nil).Once()
	s.eventLoop.SubmitProposal(proposal)
	require.Eventually(s.T(), processed.Load, time.Millisecond*100, time.Millisecond*10)
}

// Test_SubmitQC tests that submitted QC is eventually sent to `EventHandler.OnReceiveQc` for processing
func (s *EventLoopTestSuite) Test_SubmitQC() {
	// qcIngestionFunction is the archetype for EventLoop.OnQcConstructedFromVotes and EventLoop.OnNewQcDiscovered
	type qcIngestionFunction func(*flow.QuorumCertificate)

	testQCIngestionFunction := func(f qcIngestionFunction, qcView uint64) {
		qc := helper.MakeQC(helper.WithQCView(qcView))
		processed := atomic.NewBool(false)
		s.eh.On("OnReceiveQc", qc).Run(func(args mock.Arguments) {
			processed.Store(true)
		}).Return(nil).Once()
		f(qc)
		require.Eventually(s.T(), processed.Load, time.Millisecond*100, time.Millisecond*10)
	}

	s.Run("QCs handed to EventLoop.OnQcConstructedFromVotes are forwarded to EventHandler", func() {
		testQCIngestionFunction(s.eventLoop.OnQcConstructedFromVotes, 100)
	})

	s.Run("QCs handed to EventLoop.OnNewQcDiscovered are forwarded to EventHandler", func() {
		testQCIngestionFunction(s.eventLoop.OnNewQcDiscovered, 101)
	})
}

// Test_SubmitTC tests that submitted TC is eventually sent to `EventHandler.OnReceiveTc` for processing
func (s *EventLoopTestSuite) Test_SubmitTC() {
	// tcIngestionFunction is the archetype for EventLoop.OnTcConstructedFromTimeouts and EventLoop.OnNewTcDiscovered
	type tcIngestionFunction func(*flow.TimeoutCertificate)

	testTCIngestionFunction := func(f tcIngestionFunction, tcView uint64) {
		tc := helper.MakeTC(helper.WithTCView(tcView))
		processed := atomic.NewBool(false)
		s.eh.On("OnReceiveTc", tc).Run(func(args mock.Arguments) {
			processed.Store(true)
		}).Return(nil).Once()
		f(tc)
		require.Eventually(s.T(), processed.Load, time.Millisecond*100, time.Millisecond*10)
	}

	s.Run("TCs handed to EventLoop.OnTcConstructedFromTimeouts are forwarded to EventHandler", func() {
		testTCIngestionFunction(s.eventLoop.OnTcConstructedFromTimeouts, 100)
	})

	s.Run("TCs handed to EventLoop.OnNewTcDiscovered are forwarded to EventHandler", func() {
		testTCIngestionFunction(s.eventLoop.OnNewTcDiscovered, 101)
	})
}

// Test_SubmitTC_IngestNewestQC tests that included QC in TC is eventually sent to `EventHandler.OnReceiveQc` for processing
func (s *EventLoopTestSuite) Test_SubmitTC_IngestNewestQC() {
	// tcIngestionFunction is the archetype for EventLoop.OnTcConstructedFromTimeouts and EventLoop.OnNewTcDiscovered
	type tcIngestionFunction func(*flow.TimeoutCertificate)

	testTCIngestionFunction := func(f tcIngestionFunction, tcView, qcView uint64) {
		tc := helper.MakeTC(helper.WithTCView(tcView),
			helper.WithTCNewestQC(helper.MakeQC(helper.WithQCView(qcView))))
		processed := atomic.NewBool(false)
		s.eh.On("OnReceiveQc", tc.NewestQC).Run(func(args mock.Arguments) {
			processed.Store(true)
		}).Return(nil).Once()
		f(tc)
		require.Eventually(s.T(), processed.Load, time.Millisecond*100, time.Millisecond*10)
	}

	// process initial TC, this will track the newest TC
	s.eh.On("OnReceiveTc", mock.Anything).Return(nil).Once()
	s.eventLoop.OnTcConstructedFromTimeouts(helper.MakeTC(
		helper.WithTCView(100),
		helper.WithTCNewestQC(
			helper.MakeQC(
				helper.WithQCView(80),
			),
		),
	))

	s.Run("QCs handed to EventLoop.OnTcConstructedFromTimeouts are forwarded to EventHandler", func() {
		testTCIngestionFunction(s.eventLoop.OnTcConstructedFromTimeouts, 100, 99)
	})

	s.Run("QCs handed to EventLoop.OnNewTcDiscovered are forwarded to EventHandler", func() {
		testTCIngestionFunction(s.eventLoop.OnNewTcDiscovered, 100, 100)
	})
}

// Test_OnPartialTcCreated tests that event loop delivers partialTcCreated events to event handler.
func (s *EventLoopTestSuite) Test_OnPartialTcCreated() {
	view := uint64(1000)
	newestQC := helper.MakeQC(helper.WithQCView(view - 10))
	lastViewTC := helper.MakeTC(helper.WithTCView(view-1), helper.WithTCNewestQC(newestQC))

	processed := atomic.NewBool(false)
	partialTcCreated := &hotstuff.PartialTcCreated{
		View:       view,
		NewestQC:   newestQC,
		LastViewTC: lastViewTC,
	}
	s.eh.On("OnPartialTcCreated", partialTcCreated).Run(func(args mock.Arguments) {
		processed.Store(true)
	}).Return(nil).Once()
	s.eventLoop.OnPartialTcCreated(view, newestQC, lastViewTC)
	require.Eventually(s.T(), processed.Load, time.Millisecond*100, time.Millisecond*10)
}

// TestEventLoop_Timeout tests that event loop delivers timeout events to event handler under pressure
func TestEventLoop_Timeout(t *testing.T) {
	eh := &mocks.EventHandler{}
	processed := atomic.NewBool(false)
	eh.On("Start", mock.Anything).Return(nil).Once()
	eh.On("OnReceiveQc", mock.Anything).Return(nil).Maybe()
	eh.On("OnReceiveProposal", mock.Anything).Return(nil).Maybe()
	eh.On("OnLocalTimeout").Run(func(args mock.Arguments) {
		processed.Store(true)
	}).Return(nil).Once()

	log := zerolog.New(io.Discard)

	metricsCollector := metrics.NewNoopCollector()
	eventLoop, err := NewEventLoop(log, metricsCollector, metricsCollector, eh, time.Time{})
	require.NoError(t, err)

	eh.On("TimeoutChannel").Return(time.After(100 * time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)
	eventLoop.Start(signalerCtx)

	unittest.RequireCloseBefore(t, eventLoop.Ready(), 100*time.Millisecond, "event loop not stopped")

	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)

	// spam with proposals and QCs
	go func() {
		defer wg.Done()
		for !processed.Load() {
			qc := unittest.QuorumCertificateFixture()
			eventLoop.OnQcConstructedFromVotes(qc)
		}
	}()

	go func() {
		defer wg.Done()
		for !processed.Load() {
			eventLoop.SubmitProposal(helper.MakeProposal())
		}
	}()

	require.Eventually(t, processed.Load, time.Millisecond*200, time.Millisecond*10)
	unittest.AssertReturnsBefore(t, func() { wg.Wait() }, time.Millisecond*200)

	cancel()
	unittest.RequireCloseBefore(t, eventLoop.Done(), 100*time.Millisecond, "event loop not stopped")
}

// TestReadyDoneWithStartTime tests that event loop correctly starts and schedules start of processing
// when startTime argument is used
func TestReadyDoneWithStartTime(t *testing.T) {
	eh := &mocks.EventHandler{}
	eh.On("Start", mock.Anything).Return(nil)
	eh.On("TimeoutChannel").Return(make(<-chan time.Time, 1))
	eh.On("OnLocalTimeout").Return(nil)

	metrics := metrics.NewNoopCollector()

	log := zerolog.New(io.Discard)

	startTimeDuration := 2 * time.Second
	startTime := time.Now().Add(startTimeDuration)
	eventLoop, err := NewEventLoop(log, metrics, metrics, eh, startTime)
	require.NoError(t, err)

	done := make(chan struct{})
	eh.On("OnReceiveProposal", mock.AnythingOfType("*model.Proposal")).Run(func(args mock.Arguments) {
		require.True(t, time.Now().After(startTime))
		close(done)
	}).Return(nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)
	eventLoop.Start(signalerCtx)

	unittest.RequireCloseBefore(t, eventLoop.Ready(), 100*time.Millisecond, "event loop not started")

	parentBlock := unittest.BlockHeaderFixture()
	block := unittest.BlockHeaderWithParentFixture(parentBlock)
	eventLoop.SubmitProposal(model.ProposalFromFlow(block))

	unittest.RequireCloseBefore(t, done, startTimeDuration+100*time.Millisecond, "proposal wasn't received")
	cancel()
	unittest.RequireCloseBefore(t, eventLoop.Done(), 100*time.Millisecond, "event loop not stopped")
}
