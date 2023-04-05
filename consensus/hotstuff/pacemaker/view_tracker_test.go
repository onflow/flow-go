package pacemaker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
)

func TestViewTracker(t *testing.T) {
	suite.Run(t, new(ViewTrackerTestSuite))
}

type ViewTrackerTestSuite struct {
	suite.Suite

	initialView uint64
	initialQC   *flow.QuorumCertificate
	initialTC   *flow.TimeoutCertificate

	livenessData *hotstuff.LivenessData // Caution: we hand the memory address to viewTracker, which could modify this
	persist      *mocks.Persister
	tracker      viewTracker
}

func (s *ViewTrackerTestSuite) SetupTest() {
	s.initialView = 5
	s.initialQC = helper.MakeQC(helper.WithQCView(4))
	s.initialTC = nil

	s.livenessData = &hotstuff.LivenessData{
		NewestQC:    s.initialQC,
		LastViewTC:  s.initialTC,
		CurrentView: s.initialView, // we entered view 5 by observing a QC for view 4
	}
	s.persist = mocks.NewPersister(s.T())
	s.persist.On("GetLivenessData").Return(s.livenessData, nil).Once()

	var err error
	s.tracker, err = newViewTracker(s.persist)
	require.NoError(s.T(), err)
}

// confirmResultingState asserts that the view tracker's stored LivenessData reflects the provided
// current view, newest QC, and last view TC.
func (s *ViewTrackerTestSuite) confirmResultingState(curView uint64, qc *flow.QuorumCertificate, tc *flow.TimeoutCertificate) {
	require.Equal(s.T(), curView, s.tracker.CurView())
	require.Equal(s.T(), qc, s.tracker.NewestQC())
	if tc == nil {
		require.Nil(s.T(), s.tracker.LastViewTC())
	} else {
		require.Equal(s.T(), tc, s.tracker.LastViewTC())
	}
}

// TestProcessQC_SkipIncreaseViewThroughQC tests that viewTracker increases view when receiving QC,
// if applicable, by skipping views
func (s *ViewTrackerTestSuite) TestProcessQC_SkipIncreaseViewThroughQC() {
	// seeing a QC for the current view should advance the view by one
	qc := QC(s.initialView)
	expectedResultingView := s.initialView + 1
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	resultingCurView, err := s.tracker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedResultingView, resultingCurView)
	s.confirmResultingState(expectedResultingView, qc, nil)

	// seeing a QC for 10 views in the future should advance to view +11
	curView := s.tracker.CurView()
	qc = QC(curView + 10)
	expectedResultingView = curView + 11
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	resultingCurView, err = s.tracker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedResultingView, resultingCurView)
	s.confirmResultingState(expectedResultingView, qc, nil)
}

// TestProcessTC_SkipIncreaseViewThroughTC tests that viewTracker increases view when receiving TC,
// if applicable, by skipping views
func (s *ViewTrackerTestSuite) TestProcessTC_SkipIncreaseViewThroughTC() {
	// seeing a TC for the current view should advance the view by one
	qc := s.initialQC
	tc := helper.MakeTC(helper.WithTCView(s.initialView), helper.WithTCNewestQC(qc))
	expectedResultingView := s.initialView + 1
	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: expectedResultingView,
		LastViewTC:  tc,
		NewestQC:    qc,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	resultingCurView, err := s.tracker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedResultingView, resultingCurView)
	s.confirmResultingState(expectedResultingView, qc, tc)

	// seeing a TC for 10 views in the future should advance to view +11
	curView := s.tracker.CurView()
	tc = helper.MakeTC(helper.WithTCView(curView+10), helper.WithTCNewestQC(qc))
	expectedResultingView = curView + 11
	expectedLivenessData = &hotstuff.LivenessData{
		CurrentView: expectedResultingView,
		LastViewTC:  tc,
		NewestQC:    qc,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	resultingCurView, err = s.tracker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedResultingView, resultingCurView)
	s.confirmResultingState(expectedResultingView, qc, tc)
}

// TestProcessTC_IgnoreOldTC tests that viewTracker ignores old TC and doesn't advance round.
func (s *ViewTrackerTestSuite) TestProcessTC_IgnoreOldTC() {
	curView := s.tracker.CurView()
	tc := helper.MakeTC(
		helper.WithTCView(curView-1),
		helper.WithTCNewestQC(QC(curView-2)))
	resultingCurView, err := s.tracker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), curView, resultingCurView)
	s.confirmResultingState(curView, s.initialQC, s.initialTC)
}

// TestProcessTC_IgnoreNilTC tests that viewTracker accepts nil TC as allowed input but doesn't trigger a new view event
func (s *ViewTrackerTestSuite) TestProcessTC_IgnoreNilTC() {
	curView := s.tracker.CurView()
	resultingCurView, err := s.tracker.ProcessTC(nil)
	require.NoError(s.T(), err)
	require.Equal(s.T(), curView, resultingCurView)
	s.confirmResultingState(curView, s.initialQC, s.initialTC)
}

// TestProcessQC_PersistException tests that viewTracker propagates exception
// when processing QC
func (s *ViewTrackerTestSuite) TestProcessQC_PersistException() {
	qc := QC(s.initialView)
	exception := errors.New("persist-exception")
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()

	_, err := s.tracker.ProcessQC(qc)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcessTC_PersistException tests that viewTracker propagates exception
// when processing TC
func (s *ViewTrackerTestSuite) TestProcessTC_PersistException() {
	tc := helper.MakeTC(helper.WithTCView(s.initialView))
	exception := errors.New("persist-exception")
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()

	_, err := s.tracker.ProcessTC(tc)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcessQC_InvalidatesLastViewTC verifies that viewTracker does not retain any old
// TC if the last view change was triggered by observing a QC from the previous view.
func (s *ViewTrackerTestSuite) TestProcessQC_InvalidatesLastViewTC() {
	initialView := s.tracker.CurView()
	tc := helper.MakeTC(helper.WithTCView(initialView),
		helper.WithTCNewestQC(s.initialQC))
	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Twice()
	resultingCurView, err := s.tracker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), initialView+1, resultingCurView)
	require.NotNil(s.T(), s.tracker.LastViewTC())

	qc := QC(initialView + 1)
	resultingCurView, err = s.tracker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), initialView+2, resultingCurView)
	require.Nil(s.T(), s.tracker.LastViewTC())
}

// TestProcessQC_IgnoreOldQC tests that viewTracker ignores old QC and doesn't advance round
func (s *ViewTrackerTestSuite) TestProcessQC_IgnoreOldQC() {
	qc := QC(s.initialView - 1)
	resultingCurView, err := s.tracker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), s.initialView, resultingCurView)
	s.confirmResultingState(s.initialView, s.initialQC, s.initialTC)
}

// TestProcessQC_UpdateNewestQC tests that viewTracker tracks the newest QC even if it has advanced past this view.
// The only one scenario, where it is possible to receive a QC for a view that we already has passed, yet this QC
// being newer than any known one is:
//   - We advance views via TC.
//   - A QC for a passed view that is newer than any known one can arrive in 3 ways:
//     1. A QC (e.g. from the vote aggregator)
//     2. A QC embedded into a TC, where the TC is for a passed view
//     3. A QC embedded into a TC, where the TC is for the current or newer view
func (s *ViewTrackerTestSuite) TestProcessQC_UpdateNewestQC() {
	// Setup
	// * we start in view 5
	// * newest known QC is for view 4
	// * we receive a TC for view 55, which results in entering view 56
	initialView := s.tracker.CurView() //
	tc := helper.MakeTC(helper.WithTCView(initialView+50), helper.WithTCNewestQC(s.initialQC))
	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Once()
	expectedView := uint64(56) // processing the TC should results in entering view 56
	resultingCurView, err := s.tracker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedView, resultingCurView)
	s.confirmResultingState(expectedView, s.initialQC, tc)

	// Test 1: add QC for view 9, which is newer than our initial QC - it should become our newest QC
	qc := QC(s.tracker.NewestQC().View + 2)
	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: expectedView,
		LastViewTC:  tc,
		NewestQC:    qc,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	resultingCurView, err = s.tracker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedView, resultingCurView)
	s.confirmResultingState(expectedView, qc, tc)

	// Test 2: receiving a TC for a passed view, but the embedded QC is newer than the one we know
	qc2 := QC(s.tracker.NewestQC().View + 4)
	olderTC := helper.MakeTC(helper.WithTCView(qc2.View+3), helper.WithTCNewestQC(qc2))
	expectedLivenessData = &hotstuff.LivenessData{
		CurrentView: expectedView,
		LastViewTC:  tc,
		NewestQC:    qc2,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	resultingCurView, err = s.tracker.ProcessTC(olderTC)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedView, resultingCurView)
	s.confirmResultingState(expectedView, qc2, tc)

	// Test 3: receiving a TC for a newer view, the embedded QC is newer than the one we know, but still for a passed view
	qc3 := QC(s.tracker.NewestQC().View + 7)
	finalView := expectedView + 1
	newestTC := helper.MakeTC(helper.WithTCView(expectedView), helper.WithTCNewestQC(qc3))
	expectedLivenessData = &hotstuff.LivenessData{
		CurrentView: finalView,
		LastViewTC:  newestTC,
		NewestQC:    qc3,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	resultingCurView, err = s.tracker.ProcessTC(newestTC)
	require.NoError(s.T(), err)
	require.Equal(s.T(), finalView, resultingCurView)
	s.confirmResultingState(finalView, qc3, newestTC)
}
