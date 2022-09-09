package pacemaker

import (
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
	minRepTimeout          float64 = 100.0 // Milliseconds
	maxRepTimeout          float64 = 600.0 // Milliseconds
	multiplicativeIncrease float64 = 1.5   // multiplicative factor
	happyPathRounds        uint64  = 6     // number of failed rounds before first timeout increase
)

func expectedTimerInfo(view uint64) interface{} {
	return mock.MatchedBy(
		func(timerInfo *model.TimerInfo) bool {
			return timerInfo.View == view
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
	s.notifier = mocks.NewConsumer(s.T())
	s.persist = mocks.NewPersister(s.T())

	tc, err := timeout.NewConfig(
		time.Duration(minRepTimeout*1e6),
		time.Duration(maxRepTimeout*1e6),
		multiplicativeIncrease,
		happyPathRounds,
		0)
	require.NoError(s.T(), err)

	s.livenessData = &hotstuff.LivenessData{
		CurrentView: 3,
		LastViewTC:  nil,
		NewestQC:    helper.MakeQC(helper.WithQCView(2)),
	}

	s.persist.On("GetLivenessData").Return(s.livenessData, nil).Once()

	s.paceMaker, err = New(timeout.NewController(tc), s.notifier, s.persist)
	require.NoError(s.T(), err)

	s.notifier.On("OnStartingTimeout", expectedTimerInfo(s.livenessData.CurrentView)).Return().Once()

	s.paceMaker.Start()
}

func QC(view uint64) *flow.QuorumCertificate {
	return &flow.QuorumCertificate{View: view}
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
	qc := QC(s.livenessData.CurrentView)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, uint64(4)).Return().Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), qc.View+1, s.paceMaker.CurView())
	require.True(s.T(), nve.View == qc.View+1)
	require.Equal(s.T(), qc, s.paceMaker.NewestQC())
	require.Nil(s.T(), s.paceMaker.LastViewTC())

	// skip 10 views
	qc = QC(s.livenessData.CurrentView + 10)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(qc.View+1)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, qc.View+1).Return().Once()
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
	tc := helper.MakeTC(helper.WithTCView(s.livenessData.CurrentView),
		helper.WithTCNewestQC(s.livenessData.NewestQC))
	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: tc.View + 1,
		LastViewTC:  tc,
		NewestQC:    tc.NewestQC,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(tc.View+1)).Return().Once()
	s.notifier.On("OnTcTriggeredViewChange", tc, tc.View+1).Return().Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), tc.View+1, s.paceMaker.CurView())
	require.True(s.T(), nve.View == tc.View+1)
	require.Equal(s.T(), tc, s.paceMaker.LastViewTC())

	// skip 10 views
	tc = helper.MakeTC(helper.WithTCView(tc.View+10),
		helper.WithTCNewestQC(s.livenessData.NewestQC),
		helper.WithTCNewestQC(QC(s.livenessData.CurrentView)))
	expectedLivenessData = &hotstuff.LivenessData{
		CurrentView: tc.View + 1,
		LastViewTC:  tc,
		NewestQC:    tc.NewestQC,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(tc.View+1)).Return().Once()
	s.notifier.On("OnTcTriggeredViewChange", tc, tc.View+1).Return().Once()
	nve, err = s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.True(s.T(), nve.View == tc.View+1)
	require.Equal(s.T(), tc, s.paceMaker.LastViewTC())
	require.Equal(s.T(), tc.NewestQC, s.paceMaker.NewestQC())

	require.Equal(s.T(), tc.View+1, s.paceMaker.CurView())
}

// TestProcessTC_IgnoreOldTC tests that ActivePaceMaker ignores old TC and doesn't advance round.
func (s *ActivePaceMakerTestSuite) TestProcessTC_IgnoreOldTC() {
	nve, err := s.paceMaker.ProcessTC(helper.MakeTC(helper.WithTCView(s.livenessData.CurrentView-1),
		helper.WithTCNewestQC(s.livenessData.NewestQC)))
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), s.livenessData.CurrentView, s.paceMaker.CurView())
}

// TestProcessTC_IgnoreNilTC tests that ActivePaceMaker accepts nil TC as allowed input but doesn't trigger a new view event
func (s *ActivePaceMakerTestSuite) TestProcessTC_IgnoreNilTC() {
	nve, err := s.paceMaker.ProcessTC(nil)
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), s.livenessData.CurrentView, s.paceMaker.CurView())
}

// TestProcessQC_PersistException tests that ActivePaceMaker propagates exception
// when processing QC
func (s *ActivePaceMakerTestSuite) TestProcessQC_PersistException() {
	exception := errors.New("persist-exception")
	qc := QC(s.livenessData.CurrentView)
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.Nil(s.T(), nve)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcessTC_PersistException tests that ActivePaceMaker propagates exception
// when processing TC
func (s *ActivePaceMakerTestSuite) TestProcessTC_PersistException() {
	exception := errors.New("persist-exception")
	tc := helper.MakeTC(helper.WithTCView(s.livenessData.CurrentView))
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.Nil(s.T(), nve)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcessQC_InvalidatesLastViewTC verifies that PaceMaker does not retain any old
// TC if the last view change was triggered by observing a QC from the previous view.
func (s *ActivePaceMakerTestSuite) TestProcessQC_InvalidatesLastViewTC() {
	tc := helper.MakeTC(helper.WithTCView(s.livenessData.CurrentView+1),
		helper.WithTCNewestQC(s.livenessData.NewestQC))
	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Times(2)
	s.notifier.On("OnStartingTimeout", mock.Anything).Return().Times(2)
	s.notifier.On("OnTcTriggeredViewChange", mock.Anything, mock.Anything).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", mock.Anything, mock.Anything).Return().Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NotNil(s.T(), nve)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), s.paceMaker.LastViewTC())

	qc := QC(tc.View + 1)
	nve, err = s.paceMaker.ProcessQC(qc)
	require.NotNil(s.T(), nve)
	require.NoError(s.T(), err)
	require.Nil(s.T(), s.paceMaker.LastViewTC())
}

// TestProcessQC_IgnoreOldQC tests that ActivePaceMaker ignores old QC and doesn't advance round
func (s *ActivePaceMakerTestSuite) TestProcessQC_IgnoreOldQC() {
	qc := QC(s.livenessData.CurrentView - 1)
	nve, err := s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), s.livenessData.CurrentView, s.paceMaker.CurView())
	require.NotEqual(s.T(), qc, s.paceMaker.NewestQC())
}

// TestProcessQC_UpdateNewestQC tests that ActivePaceMaker tracks the newest QC even if it has advanced past this view.
// In this test, we feed a newer QC as part of a TC into the PaceMaker.
func (s *ActivePaceMakerTestSuite) TestProcessQC_UpdateNewestQC() {
	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", mock.Anything).Return().Once()
	s.notifier.On("OnTcTriggeredViewChange", mock.Anything, mock.Anything).Return().Once()
	tc := helper.MakeTC(helper.WithTCView(s.livenessData.CurrentView+10),
		helper.WithTCNewestQC(s.livenessData.NewestQC))
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), nve)

	qc := QC(s.livenessData.NewestQC.View + 5)

	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: s.livenessData.CurrentView,
		LastViewTC:  s.livenessData.LastViewTC,
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
	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", mock.Anything).Return().Once()
	s.notifier.On("OnTcTriggeredViewChange", mock.Anything, mock.Anything).Return().Once()
	tc := helper.MakeTC(helper.WithTCView(s.livenessData.CurrentView+10),
		helper.WithTCNewestQC(s.livenessData.NewestQC))
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), nve)

	qc := QC(s.livenessData.NewestQC.View + 5)
	olderTC := helper.MakeTC(helper.WithTCView(s.paceMaker.CurView()-1),
		helper.WithTCNewestQC(qc))

	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: s.livenessData.CurrentView,
		LastViewTC:  s.livenessData.LastViewTC,
		NewestQC:    qc,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()

	nve, err = s.paceMaker.ProcessTC(olderTC)
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	require.Equal(s.T(), qc, s.paceMaker.NewestQC())
}
