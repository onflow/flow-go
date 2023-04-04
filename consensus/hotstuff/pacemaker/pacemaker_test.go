package pacemaker

import (
	"context"
	"errors"
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
)

const (
	minRepTimeout             float64 = 100.0 // Milliseconds
	maxRepTimeout             float64 = 600.0 // Milliseconds
	multiplicativeIncrease    float64 = 1.5   // multiplicative factor
	happyPathMaxRoundFailures uint64  = 6     // number of failed rounds before first timeout increase
)

func expectedTimerInfo(view uint64) interface{} {
	return mock.MatchedBy(
		func(timerInfo model.TimerInfo) bool {
			return timerInfo.View == view
		})
}

func TestActivePaceMaker(t *testing.T) {
	suite.Run(t, new(ActivePaceMakerTestSuite))
}

type ActivePaceMakerTestSuite struct {
	suite.Suite

	initialView uint64
	initialQC   *flow.QuorumCertificate
	initialTC   *flow.TimeoutCertificate

	notifier  *mocks.Consumer
	persist   *mocks.Persister
	paceMaker *ActivePaceMaker
	stop      context.CancelFunc
}

func (s *ActivePaceMakerTestSuite) SetupTest() {
	s.initialView = 3
	s.initialQC = QC(2)
	s.initialTC = nil

	tc, err := timeout.NewConfig(
		time.Duration(minRepTimeout*1e6),
		time.Duration(maxRepTimeout*1e6),
		multiplicativeIncrease,
		happyPathMaxRoundFailures,
		0,
		time.Duration(maxRepTimeout*1e6))
	require.NoError(s.T(), err)

	// init consumer for notifications emitted by PaceMaker
	s.notifier = mocks.NewConsumer(s.T())
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(s.initialView)).Return().Once()

	// init Persister dependency for PaceMaker
	// CAUTION: The Persister hands a pointer to `livenessData` to the PaceMaker, which means the PaceMaker
	// could modify our struct in-place. `livenessData` is a local variable, which is not accessible by the
	// tests. Thereby, we avoid any possibility of tests deriving any expected values from  `livenessData`.
	s.persist = mocks.NewPersister(s.T())
	livenessData := &hotstuff.LivenessData{
		CurrentView: 3,
		LastViewTC:  nil,
		NewestQC:    s.initialQC,
	}
	s.persist.On("GetLivenessData").Return(livenessData, nil).Once()

	// init PaceMaker and start
	s.paceMaker, err = New(timeout.NewController(tc), s.notifier, s.persist)
	require.NoError(s.T(), err)

	var ctx context.Context
	ctx, s.stop = context.WithCancel(context.Background())
	s.paceMaker.Start(ctx)
}

func (s *ActivePaceMakerTestSuite) TearDownTest() {
	s.stop()
}

func QC(view uint64) *flow.QuorumCertificate {
	return helper.MakeQC(helper.WithQCView(view))
}

func LivenessData(qc *flow.QuorumCertificate) *hotstuff.LivenessData {
	return &hotstuff.LivenessData{
		CurrentView: qc.View + 1,
		LastViewTC:  nil,
		NewestQC:    qc,
	}
}

// TestProcessQC_SkipIncreaseViewThroughQC tests that ActivePaceMaker increases view when receiving QC,
// if applicable, by skipping views
func (s *ActivePaceMakerTestSuite) TestProcessQC_SkipIncreaseViewThroughQC() {
	// seeing a QC for the current view should advance the view by one
	qc := QC(s.initialView)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", s.initialView, uint64(4), qc).Return().Once()
	s.notifier.On("OnViewChange", s.initialView, qc.View+1).Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), qc.View+1, s.paceMaker.CurView())
	require.True(s.T(), nve.View == qc.View+1)
	require.Equal(s.T(), qc, s.paceMaker.NewestQC())
	require.Nil(s.T(), s.paceMaker.LastViewTC())

	// seeing a QC for 10 views in the future should advance to view +11
	curView := s.paceMaker.CurView()
	qc = QC(curView + 10)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(qc.View+1)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", curView, qc.View+1, qc).Return().Once()
	s.notifier.On("OnViewChange", curView, qc.View+1).Once()
	nve, err = s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.True(s.T(), nve.View == qc.View+1)
	require.Equal(s.T(), qc, s.paceMaker.NewestQC())
	require.Nil(s.T(), s.paceMaker.LastViewTC())

	require.Equal(s.T(), qc.View+1, s.paceMaker.CurView())
}

// TestProcessTC_SkipIncreaseViewThroughTC tests that ActivePaceMaker increases view when receiving TC,
// if applicable, by skipping views
func (s *ActivePaceMakerTestSuite) TestProcessTC_SkipIncreaseViewThroughTC() {
	// seeing a TC for the current view should advance the view by one
	tc := helper.MakeTC(helper.WithTCView(s.initialView), helper.WithTCNewestQC(s.initialQC))
	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: tc.View + 1,
		LastViewTC:  tc,
		NewestQC:    s.initialQC,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(tc.View+1)).Return().Once()
	s.notifier.On("OnTcTriggeredViewChange", s.initialView, tc.View+1, tc).Return().Once()
	s.notifier.On("OnViewChange", s.initialView, tc.View+1).Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), tc.View+1, s.paceMaker.CurView())
	require.True(s.T(), nve.View == tc.View+1)
	require.Equal(s.T(), tc, s.paceMaker.LastViewTC())

	// seeing a TC for 10 views in the future should advance to view +11
	curView := s.paceMaker.CurView()
	tc = helper.MakeTC(helper.WithTCView(curView+10), helper.WithTCNewestQC(s.initialQC))
	expectedLivenessData = &hotstuff.LivenessData{
		CurrentView: tc.View + 1,
		LastViewTC:  tc,
		NewestQC:    s.initialQC,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(tc.View+1)).Return().Once()
	s.notifier.On("OnTcTriggeredViewChange", curView, tc.View+1, tc).Return().Once()
	s.notifier.On("OnViewChange", curView, tc.View+1).Once()
	nve, err = s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.True(s.T(), nve.View == tc.View+1)
	require.Equal(s.T(), tc, s.paceMaker.LastViewTC())
	require.Equal(s.T(), tc.NewestQC, s.paceMaker.NewestQC())

	require.Equal(s.T(), tc.View+1, s.paceMaker.CurView())
}

// TestProcessTC_IgnoreOldTC tests that ActivePaceMaker ignores old TC and doesn't advance round.
func (s *ActivePaceMakerTestSuite) TestProcessTC_IgnoreOldTC() {
	nve, err := s.paceMaker.ProcessTC(helper.MakeTC(helper.WithTCView(s.initialView-1),
		helper.WithTCNewestQC(s.initialQC)))
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), s.initialView, s.paceMaker.CurView())
}

// TestProcessTC_IgnoreNilTC tests that ActivePaceMaker accepts nil TC as allowed input but doesn't trigger a new view event
func (s *ActivePaceMakerTestSuite) TestProcessTC_IgnoreNilTC() {
	nve, err := s.paceMaker.ProcessTC(nil)
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), s.initialView, s.paceMaker.CurView())
}

// TestProcessQC_PersistException tests that ActivePaceMaker propagates exception
// when processing QC
func (s *ActivePaceMakerTestSuite) TestProcessQC_PersistException() {
	exception := errors.New("persist-exception")
	qc := QC(s.initialView)
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.Nil(s.T(), nve)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcessTC_PersistException tests that ActivePaceMaker propagates exception
// when processing TC
func (s *ActivePaceMakerTestSuite) TestProcessTC_PersistException() {
	exception := errors.New("persist-exception")
	tc := helper.MakeTC(helper.WithTCView(s.initialView))
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.Nil(s.T(), nve)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcessQC_InvalidatesLastViewTC verifies that PaceMaker does not retain any old
// TC if the last view change was triggered by observing a QC from the previous view.
func (s *ActivePaceMakerTestSuite) TestProcessQC_InvalidatesLastViewTC() {
	tc := helper.MakeTC(helper.WithTCView(s.initialView+1), helper.WithTCNewestQC(s.initialQC))
	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Times(2)
	s.notifier.On("OnStartingTimeout", mock.Anything).Return().Times(2)
	s.notifier.On("OnTcTriggeredViewChange", mock.Anything, mock.Anything, mock.Anything).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", mock.Anything, mock.Anything, mock.Anything).Return().Once()
	s.notifier.On("OnViewChange", s.initialView, tc.View+1).Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NotNil(s.T(), nve)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), s.paceMaker.LastViewTC())

	qc := QC(tc.View + 1)
	s.notifier.On("OnViewChange", tc.View+1, qc.View+1).Once()
	nve, err = s.paceMaker.ProcessQC(qc)
	require.NotNil(s.T(), nve)
	require.NoError(s.T(), err)
	require.Nil(s.T(), s.paceMaker.LastViewTC())
}

// TestProcessQC_IgnoreOldQC tests that ActivePaceMaker ignores old QC and doesn't advance round
func (s *ActivePaceMakerTestSuite) TestProcessQC_IgnoreOldQC() {
	qc := QC(s.initialView - 1)
	nve, err := s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), s.initialView, s.paceMaker.CurView())
	require.NotEqual(s.T(), qc, s.paceMaker.NewestQC())
}

// TestProcessQC_UpdateNewestQC tests that ActivePaceMaker tracks the newest QC even if it has advanced past this view.
// In this test, we feed a newer QC as part of a TC into the PaceMaker.
func (s *ActivePaceMakerTestSuite) TestProcessQC_UpdateNewestQC() {
	tc := helper.MakeTC(helper.WithTCView(s.initialView+10), helper.WithTCNewestQC(s.initialQC))
	expectedView := tc.View + 1
	s.notifier.On("OnTcTriggeredViewChange", mock.Anything, mock.Anything, mock.Anything).Return().Once()
	s.notifier.On("OnViewChange", s.initialView, expectedView).Once()
	s.notifier.On("OnStartingTimeout", mock.Anything).Return().Once()
	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), nve)

	qc := QC(s.initialView + 5)
	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: expectedView,
		LastViewTC:  tc,
		NewestQC:    qc,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()

	nve, err = s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), qc, s.paceMaker.NewestQC())
}

// TestProcessTC_UpdateNewestQC tests that ActivePaceMaker tracks the newest QC included in TC even if it has advanced past this view.
func (s *ActivePaceMakerTestSuite) TestProcessTC_UpdateNewestQC() {
	tc := helper.MakeTC(helper.WithTCView(s.initialView+10), helper.WithTCNewestQC(s.initialQC))
	expectedView := tc.View + 1
	s.notifier.On("OnTcTriggeredViewChange", mock.Anything, mock.Anything, mock.Anything).Return().Once()
	s.notifier.On("OnViewChange", s.initialView, expectedView).Once()
	s.notifier.On("OnStartingTimeout", mock.Anything).Return().Once()
	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), nve)

	qc := QC(s.initialView + 5)
	olderTC := helper.MakeTC(helper.WithTCView(s.paceMaker.CurView()-1), helper.WithTCNewestQC(qc))
	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: expectedView,
		LastViewTC:  tc,
		NewestQC:    qc,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()

	nve, err = s.paceMaker.ProcessTC(olderTC)
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), qc, s.paceMaker.NewestQC())
}
