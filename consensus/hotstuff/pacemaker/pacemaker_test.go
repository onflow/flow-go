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

// TestProcessQC_SkipIncreaseViewThroughQC tests that ActivePaceMaker increases view when receiving QC,
// if applicable, by skipping views
func (s *ActivePaceMakerTestSuite) TestProcessQC_SkipIncreaseViewThroughQC() {
	qc := QC(3)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, uint64(4)).Return().Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(4), s.paceMaker.CurView())
	require.True(s.T(), nve.View == 4)
	require.Equal(s.T(), qc, s.paceMaker.HighestQC())
	require.Nil(s.T(), s.paceMaker.LastViewTC())

	qc = QC(12)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(13, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, uint64(13)).Return().Once()
	nve, err = s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.True(s.T(), nve.View == 13)
	require.Equal(s.T(), qc, s.paceMaker.HighestQC())
	require.Nil(s.T(), s.paceMaker.LastViewTC())

	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(13), s.paceMaker.CurView())
}

// TestProcessTC_SkipIncreaseViewThroughTC tests that ActivePaceMaker increases view when receiving TC,
// if applicable, by skipping views
func (s *ActivePaceMakerTestSuite) TestProcessTC_SkipIncreaseViewThroughTC() {
	tc := helper.MakeTC(helper.WithTCView(3),
		helper.WithTCHighestQC(s.livenessData.HighestQC))
	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: tc.View + 1,
		LastViewTC:  tc,
		HighestQC:   tc.TOHighestQC,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	//s.notifier.On("OnQcTriggeredViewChange", qc, uint64(4)).Return().Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(4), s.paceMaker.CurView())
	require.True(s.T(), nve.View == 4)
	require.Equal(s.T(), tc, s.paceMaker.LastViewTC())

	tc = helper.MakeTC(helper.WithTCView(12),
		helper.WithTCHighestQC(s.livenessData.HighestQC),
		helper.WithTCHighestQC(QC(3)))
	expectedLivenessData = &hotstuff.LivenessData{
		CurrentView: tc.View + 1,
		LastViewTC:  tc,
		HighestQC:   tc.TOHighestQC,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(13, model.ReplicaTimeout)).Return().Once()
	//s.notifier.On("OnQcTriggeredViewChange", qc, uint64(13)).Return().Once()
	nve, err = s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.True(s.T(), nve.View == 13)
	require.Equal(s.T(), tc, s.paceMaker.LastViewTC())
	require.Equal(s.T(), tc.TOHighestQC, s.paceMaker.HighestQC())

	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(13), s.paceMaker.CurView())
}

// TestProcessQC_PersistException tests that ActivePaceMaker propagates exception
// when processing QC
func (s *ActivePaceMakerTestSuite) TestProcessQC_PersistException() {
	exception := errors.New("persist-exception")
	qc := QC(3)
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.Nil(s.T(), nve)
	require.ErrorAs(s.T(), err, &exception)
}

// TestProcessTC_PersistException tests that ActivePaceMaker propagates exception
// when processing TC
func (s *ActivePaceMakerTestSuite) TestProcessTC_PersistException() {
	exception := errors.New("persist-exception")
	tc := helper.MakeTC(helper.WithTCView(3))
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.Nil(s.T(), nve)
	require.ErrorAs(s.T(), err, &exception)
}

// TestProcessQC_IgnoreOldQC tests that ActivePaceMaker ignores old QCs
func (s *ActivePaceMakerTestSuite) TestProcessQC_IgnoreOldQC() {
	nve, err := s.paceMaker.ProcessQC(QC(2))
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(3), s.paceMaker.CurView())
}
