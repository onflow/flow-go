package pacemaker

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	startRepTimeout        float64 = 400.0 // Milliseconds
	minRepTimeout          float64 = 100.0 // Milliseconds
	voteTimeoutFraction    float64 = 0.5   // multiplicative factor
	multiplicativeIncrease float64 = 1.5   // multiplicative factor
	multiplicativeDecrease float64 = 0.85  // multiplicative factor
)

func expectedTimerInfo(view uint64, mode model.TimeoutMode) interface{} {
	return mock.MatchedBy(
		func(timerInfo *model.TimerInfo) bool {
			return timerInfo.View == view && timerInfo.Mode == mode
		})
}

func expectedTimeoutInfo(view uint64, mode model.TimeoutMode) interface{} {
	return mock.MatchedBy(
		func(timerInfo *model.TimerInfo) bool {
			return timerInfo.View == view && timerInfo.Mode == mode
		})
}

func TestActivePaceMaker(t *testing.T) {
	suite.Run(t, new(ActivePaceMakerTestSuite))
}

type ActivePaceMakerTestSuite struct {
	suite.Suite

	livenessData *hotstuff.LivenessData
	notifier     *mocks.Consumer
	persist      *mocks.Persister
	paceMaker    *ActivePaceMaker
}

func (s *ActivePaceMakerTestSuite) SetupTest() {
	s.notifier = &mocks.Consumer{}
	s.persist = &mocks.Persister{}

	tc, err := timeout.NewConfig(
		time.Duration(startRepTimeout*1e6),
		time.Duration(minRepTimeout*1e6),
		voteTimeoutFraction,
		multiplicativeIncrease,
		multiplicativeDecrease,
		0)
	require.NoError(s.T(), err)

	s.livenessData = &hotstuff.LivenessData{
		CurrentView: 3,
		LastViewTC:  nil,
		HighestQC:   helper.MakeQC(helper.WithQCView(2)),
	}

	s.paceMaker, err = New(s.livenessData, timeout.NewController(tc), s.notifier, s.persist)
	require.NoError(s.T(), err)

	s.notifier.On("OnStartingTimeout", expectedTimerInfo(s.livenessData.CurrentView, model.ReplicaTimeout)).Return().Once()

	s.paceMaker.Start()
}

func QC(view uint64) *flow.QuorumCertificate {
	return &flow.QuorumCertificate{View: view}
}

func LivenessData(qc *flow.QuorumCertificate) *hotstuff.LivenessData {
	return &hotstuff.LivenessData{
		CurrentView: qc.View + 1,
		LastViewTC:  nil,
		HighestQC:   qc,
	}
}

//func makeBlock(qcView, blockView uint64) *model.Block {
//	return &model.Block{View: blockView, QC: QC(qcView)}
//}

// Test_SkipIncreaseViewThroughQC tests that PaceMaker increases View when receiving QC,
// if applicable, by skipping views
func (s *ActivePaceMakerTestSuite) Test_SkipIncreaseViewThroughQC() {

	qc := QC(3)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, uint64(4)).Return().Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(4), s.paceMaker.CurView())
	require.True(s.T(), nve.View == 4)

	qc = QC(12)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(13, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, uint64(13)).Return().Once()
	nve, err = s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.True(s.T(), nve.View == 13)

	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(13), s.paceMaker.CurView())
}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
func (s *ActivePaceMakerTestSuite) Test_IgnoreOldQC() {

	nve, err := s.paceMaker.ProcessQC(QC(2))
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(3), s.paceMaker.CurView())
}

// Test_SkipViewThroughBlock tests that PaceMaker skips View when receiving Block containing QC with larger View Number
//func (s *ActivePaceMakerTestSuite) Test_SkipViewThroughBlock() {
//	pm, notifier := initPaceMaker(s.T(), 3)
//
//	block := makeBlock(5, 9)
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(6, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(6)).Return().Once()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(block, true)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(6), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 6)
//
//	block = makeBlock(22, 25)
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(23, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(23)).Return().Once()
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, false)
//	require.True(s.T(), err && nve.View == 23)
//
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(23), s.paceMaker.CurView())
//}

// Test_HandlesSkipViewAttack verifies that PaceMaker skips views based on QC.view
// but NOT based on block.View to avoid vulnerability against Fast-Forward Attack
//func (s *ActivePaceMakerTestSuite) Test_HandlesSkipViewAttack() {
//	pm, notifier := initPaceMaker(s.T(), 3)
//
//	block := makeBlock(5, 9)
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(6, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(6)).Return().Once()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(block, true)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(6), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 6)
//
//	block = makeBlock(14, 23)
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(15, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(15)).Return().Once()
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, false)
//	require.True(s.T(), err && nve.View == 15)
//
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(15), s.paceMaker.CurView())
//}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
//func (s *ActivePaceMakerTestSuite) Test_IgnoreOldBlocks() {
//	pm, notifier := initPaceMaker(s.T(), 3)
//	s.paceMaker.UpdateCurViewWithBlock(makeBlock(1, 2), false)
//	s.paceMaker.UpdateCurViewWithBlock(makeBlock(1, 2), true)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(3), s.paceMaker.CurView())
//}

// Test_ProcessBlockForCurrentView tests that PaceMaker processes the block for the current view correctly
//func (s *ActivePaceMakerTestSuite) Test_ProcessBlockForCurrentView() {
//	pm, notifier := initPaceMaker(s.T(), 3)
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(3, model.VoteCollectionTimeout)).Return().Once()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(makeBlock(1, 3), true)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(3), s.paceMaker.CurView())
//	require.True(s.T(), !err && nve == nil)
//
//	pm, notifier = initPaceMaker(s.T(), 3)
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(makeBlock(1, 3), false)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(4), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 4)
//}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view = curView
//    block.view = curView +1
// correctly. Specifically, it should induce a view change to the block.view, which
// enables processing the block right away, i.e. switch to block.view + 1
//func (s *ActivePaceMakerTestSuite) Test_FutureBlockWithQcForCurrentView() {
//	block := makeBlock(3, 4)
//
//	// NOT Primary for the Block's view
//	pm, notifier := initPaceMaker(s.T(), 3)
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(5, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(4)).Return().Once()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(block, false)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(5), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 5)
//
//	// PRIMARY for the Block's view
//	pm, notifier = initPaceMaker(s.T(), 3)
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.VoteCollectionTimeout)).Return().Once()
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(4)).Return().Once()
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, true)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(4), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 4)
//}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view > curView
//    block.view = block.qc.view +1
// correctly. Specifically, it should induce a view change to block.view, which
// enables processing the block right away, i.e. switch to block.view + 1
//func (s *ActivePaceMakerTestSuite) Test_FutureBlockWithQcForFutureView() {
//	block := makeBlock(13, 14)
//
//	pm, notifier := initPaceMaker(s.T(), 3)
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(14)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(15, model.ReplicaTimeout)).Return().Once()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(block, false)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(15), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 15)
//
//	pm, notifier = initPaceMaker(s.T(), 3)
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(14)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.VoteCollectionTimeout)).Return().Once()
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, true)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(14), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 14)
//}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view > curView
//    block.view > block.qc.view +1
// correctly. Specifically, it should induce a view change to the block.qc.view +1,
// which is not sufficient to process the block. I.e. we expect view change to block.qc.view +1
// enables processing the block right away.
//func (s *ActivePaceMakerTestSuite) Test_FutureBlockWithQcForFutureFutureView() {
//	block := makeBlock(13, 17)
//
//	pm, notifier := initPaceMaker(s.T(), 3)
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(14)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.ReplicaTimeout)).Return().Once()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(block, false)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(14), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 14)
//
//	pm, notifier = initPaceMaker(s.T(), 3)
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(14)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(14, model.ReplicaTimeout)).Return().Once()
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, true)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(14), s.paceMaker.CurView())
//	require.True(s.T(), err && nve.View == 14)
//}

// Test_IgnoreBlockDuplicates tests that PaceMaker ignores duplicate blocks
//func (s *ActivePaceMakerTestSuite) Test_IgnoreBlockDuplicates() {
//	block := makeBlock(3, 4)
//
//	// NOT Primary for the Block's view
//	pm, notifier := initPaceMaker(s.T(), 3)
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(4)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(5, model.ReplicaTimeout)).Return().Once()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(block, false)
//	require.True(s.T(), err && nve.View == 5)
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, false)
//	require.True(s.T(), !err && nve == nil)
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, false)
//	require.True(s.T(), !err && nve == nil)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(5), s.paceMaker.CurView())
//
//	// PRIMARY for the Block's view
//	pm, notifier = initPaceMaker(s.T(), 3)
//	s.notifier.On("OnQcTriggeredViewChange", block.QC, uint64(4)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.VoteCollectionTimeout)).Return().Once()
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, true)
//	require.True(s.T(), err && nve.View == 4)
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, true)
//	require.True(s.T(), !err && nve == nil)
//	nve, err = s.paceMaker.UpdateCurViewWithBlock(block, true)
//	require.True(s.T(), !err && nve == nil)
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(4), s.paceMaker.CurView())
//}

// Test_ReplicaTimeout tests that replica timeout fires as expected
func (s *ActivePaceMakerTestSuite) Test_ReplicaTimeout() {
	unittest.SkipUnless(s.T(), unittest.TEST_TODO, "active-pacemaker")
	start := time.Now()

	select {
	case <-s.paceMaker.TimeoutChannel():
		break // testing path: corresponds to EventLoop picking up timeout from channel
	case <-time.After(time.Duration(2) * time.Duration(startRepTimeout) * time.Millisecond):
		s.T().Fail() // to prevent test from hanging
	}
	actualTimeout := float64(time.Since(start).Milliseconds()) // in millisecond
	fmt.Println(actualTimeout)
	require.GreaterOrEqual(s.T(), actualTimeout, startRepTimeout, "the actual timeout should be greater or equal to the init timeout")
	// While the timeout event has been put in the channel,
	// PaceMaker should NOT react on it without the timeout event being processed by the EventHandler
	require.Equal(s.T(), uint64(3), s.paceMaker.CurView())

	// here the, the Event loop would now call EventHandler.OnTimeout() -> PaceMaker.OnTimeout()
	s.notifier.On("OnReachedTimeout", expectedTimeoutInfo(3, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	//viewChange := s.paceMaker.OnTimeout()
	//if viewChange == nil {
	//	require.Fail(s.T(), "Expecting ViewChange event as result of timeout")
	//}

	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(4), s.paceMaker.CurView())
}

// Test_ViewChangeWithProgress tests that the PaceMaker respects the definition of Progress:
// Progress is defined as entering view V for which the replica knows a QC with V = QC.view + 1
// For this test, we feed it with a block for the current view containing a QC from the last view.
// Hence, the PaceMaker should decrease the timeout (because the committee is synchronized).
//func (s *ActivePaceMakerTestSuite) Test_ViewChangeWithProgress() {
//	pm, notifier := initPaceMaker(s.T(), 5) // initPaceMaker also calls Start() on PaceMaker
//
//	// The pace maker should transition into view 6
//	// and decrease its timeout value by the following pacemaker call, as progress is made
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(6, model.ReplicaTimeout)).Return().Once()
//	start := time.Now()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(makeBlock(4, 5), false)
//	require.True(s.T(), err && nve.View == 6)
//	s.notifier.AssertExpectations(s.T())
//
//	select {
//	case <-s.paceMaker.TimeoutChannel():
//		break // testing path: corresponds to EventLoop picking up timeout from channel
//	case <-time.After(time.Duration(2) * time.Duration(startRepTimeout) * time.Millisecond):
//		s.T().Fail() // to prevent test from hanging
//	}
//
//	actualTimeout := float64(time.Since(start).Milliseconds()) // in millisecond
//	expectedVoteCollectionTimeout := startRepTimeout * multiplicativeDecrease
//	require.True(s.T(), math.Abs(actualTimeout-expectedVoteCollectionTimeout) < 0.1*expectedVoteCollectionTimeout)
//	require.Equal(s.T(), uint64(6), s.paceMaker.CurView())
//}

// Test_ViewChangeWithoutProgress tests that the PaceMaker respects the definition of Progress:
// Progress is defined as entering view V for which the replica knows a QC with V = QC.view + 1
// For this test, we feed it with a block for the current view, but containing an old QC.
// Hence, the PaceMaker should NOT decrease the timeout (because the committee is still not synchronized)
//func (s *ActivePaceMakerTestSuite) Test_ViewChangeWithoutProgress() {
//	pm, notifier := initPaceMaker(s.T(), 5) // initPaceMaker also calls Start() on PaceMaker
//
//	// while the pace maker should transition into view 6
//	// we are not expecting a change of timeout value by the following pacemaker call, because no progress is made
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(6, model.ReplicaTimeout)).Return().Once()
//	start := time.Now()
//	nve, err := s.paceMaker.UpdateCurViewWithBlock(makeBlock(3, 5), false)
//	require.True(s.T(), err && nve.View == 6)
//	s.notifier.AssertExpectations(s.T())
//
//	select {
//	case <-s.paceMaker.TimeoutChannel():
//		break // testing path: corresponds to EventLoop picking up timeout from channel
//	case <-time.After(time.Duration(2) * time.Duration(startRepTimeout) * time.Millisecond):
//		s.T().Fail() // to prevent test from hanging
//	}
//
//	actualTimeout := float64(time.Since(start).Milliseconds()) // in millisecond
//	expectedVoteCollectionTimeout := startRepTimeout
//	require.True(s.T(), math.Abs(actualTimeout-expectedVoteCollectionTimeout) < 0.1*expectedVoteCollectionTimeout)
//	require.Equal(s.T(), uint64(6), s.paceMaker.CurView())
//}

//func (s *ActivePaceMakerTestSuite) Test_ReplicaTimeoutAgain() {
//	start := time.Now()
//	pm, notifier := initPaceMaker(s.T(), 3) // initPaceMaker also calls Start() on PaceMaker
//	s.notifier.On("OnReachedTimeout", mock.Anything)
//	s.notifier.On("OnStartingTimeout", mock.Anything)
//
//	// wait until the timeout is hit for the first time
//	select {
//	case <-s.paceMaker.TimeoutChannel():
//	case <-time.After(time.Duration(2*startRepTimeout) * time.Millisecond):
//	}
//
//	// calculate the actual timeout duration that has been waited.
//	actualTimeout := float64(time.Since(start).Milliseconds()) // in millisecond
//	require.GreaterOrEqual(s.T(), actualTimeout, startRepTimeout, "the actual timeout should be greater or equal to the init timeout")
//	require.Less(s.T(), actualTimeout, 2*startRepTimeout, "the actual timeout was too long")
//
//	// reset timer
//	start = time.Now()
//
//	// trigger view change and restart the pacemaker timer. The next timeout should take 1.5 longer
//	_ = s.paceMaker.OnTimeout()
//
//	// wait until the timeout is hit again
//	select {
//	case <-s.paceMaker.TimeoutChannel():
//	case <-time.After(time.Duration(3) * time.Duration(startRepTimeout) * time.Millisecond):
//	}
//
//	nv := s.paceMaker.OnTimeout()
//
//	// calculate the actual timeout duration that has been waited again
//	actualTimeout = float64(time.Since(start).Milliseconds()) // in millisecond
//	require.GreaterOrEqual(s.T(), actualTimeout, 1.5*startRepTimeout, "the actual timeout should be greater or equal to the timeout")
//	require.Less(s.T(), actualTimeout, 2*startRepTimeout, "the actual timeout was too long")
//
//	var changed bool
//	// timeout reduce linearly
//	select {
//	case <-s.paceMaker.TimeoutChannel():
//		break
//	case <-time.After(time.Duration(0.1 * startRepTimeout)):
//		nv, changed = s.paceMaker.UpdateCurViewWithBlock(makeBlock(nv.View-1, nv.View), false)
//		break
//	}
//
//	require.True(s.T(), changed)
//
//	// timeout reduce linearly
//	select {
//	case <-s.paceMaker.TimeoutChannel():
//		break
//	case <-time.After(time.Duration(0.1 * startRepTimeout)):
//		// start the timer before the UpdateCurViewWithBlock is called, which starts the internal timer.
//		// This is required to make the tests more accurate, because when it's running on CI,
//		// there might be some delay for time.Now() gets called, which impact the accuracy of the actual timeout
//		// measurement
//
//		start = time.Now()
//		_, changed = s.paceMaker.UpdateCurViewWithBlock(makeBlock(nv.View-1, nv.View), false)
//		break
//	}
//
//	require.True(s.T(), changed)
//
//	// wait until the timeout is hit again
//	select {
//	case <-s.paceMaker.TimeoutChannel():
//	case <-time.After(time.Duration(3) * time.Duration(startRepTimeout) * time.Millisecond):
//	}
//
//	actualTimeout = float64(time.Since(start).Microseconds()) * 0.001 // in millisecond
//	// the actual timeout should be startRepTimeout * 1.5 *1.5 * multiplicativeDecrease * multiplicativeDecrease
//	// because it hits timeout twice and then received blocks for current view twice
//	require.GreaterOrEqual(s.T(), actualTimeout, 1.5*1.5*startRepTimeout*multiplicativeDecrease*multiplicativeDecrease, "the actual timeout should be greater or equal to the timeout")
//	require.Less(s.T(), actualTimeout, 1.5*1.5*startRepTimeout*multiplicativeDecrease, "the actual timeout is too long")
//}

// Test_VoteTimeout tests that vote timeout fires as expected
//func (s *ActivePaceMakerTestSuite) Test_VoteTimeout() {
//	pm, notifier := initPaceMaker(s.T(), 3) // initPaceMaker also calls Start() on PaceMaker
//	start := time.Now()
//
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(3, model.VoteCollectionTimeout)).Return().Once()
//	// we are not expecting a change of timeout value by the following pacemaker call, because no view change happens
//	s.paceMaker.UpdateCurViewWithBlock(makeBlock(2, 3), true)
//	s.notifier.AssertExpectations(s.T())
//
//	// the pacemaker should now be running the vote collection timeout:
//	expectedVoteCollectionTimeout := startRepTimeout * voteTimeoutFraction
//	select {
//	case <-s.paceMaker.TimeoutChannel():
//		break // testing path: corresponds to EventLoop picking up timeout from channel
//	case <-time.After(2 * time.Duration(expectedVoteCollectionTimeout) * time.Millisecond):
//		s.T().Fail() // to prevent test from hanging
//	}
//	duration := float64(time.Since(start).Milliseconds()) // in millisecond
//	require.True(s.T(), math.Abs(duration-expectedVoteCollectionTimeout) < 0.1*expectedVoteCollectionTimeout)
//	// While the timeout event has been put in the channel,
//	// PaceMaker should NOT react on it without the timeout event being processed by the EventHandler
//	require.Equal(s.T(), uint64(3), s.paceMaker.CurView())
//
//	// here the, the Event loop would now call EventHandler.OnTimeout() -> PaceMaker.OnTimeout()
//	s.notifier.On("OnReachedTimeout", expectedTimeoutInfo(3, model.VoteCollectionTimeout)).Return().Once()
//	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
//	viewChange := s.paceMaker.OnTimeout()
//	if viewChange == nil {
//		require.Fail(s.T(), "Expecting ViewChange event as result of timeout")
//	}
//	s.notifier.AssertExpectations(s.T())
//	require.Equal(s.T(), uint64(4), s.paceMaker.CurView())
//}
