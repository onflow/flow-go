package pacemaker

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	notifier                 *mocks.Consumer
	proposalDurationProvider hotstuff.ProposalDurationProvider
	persist                  *mocks.Persister
	paceMaker                *ActivePaceMaker
	stop                     context.CancelFunc
	timeoutConf              timeout.Config
}

func (s *ActivePaceMakerTestSuite) SetupTest() {
	s.initialView = 3
	s.initialQC = QC(2)
	s.initialTC = nil
	var err error

	s.timeoutConf, err = timeout.NewConfig(time.Duration(minRepTimeout*1e6), time.Duration(maxRepTimeout*1e6), multiplicativeIncrease, happyPathMaxRoundFailures, time.Duration(maxRepTimeout*1e6))
	require.NoError(s.T(), err)

	// init consumer for notifications emitted by PaceMaker
	s.notifier = mocks.NewConsumer(s.T())
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(s.initialView)).Return().Once()

	// init Persister dependency for PaceMaker
	// CAUTION: The Persister hands a pointer to `livenessData` to the PaceMaker, which means the PaceMaker
	// could modify our struct in-place. `livenessData` should not be used by tests to determine expected values!
	s.persist = mocks.NewPersister(s.T())
	livenessData := &hotstuff.LivenessData{
		CurrentView: 3,
		LastViewTC:  nil,
		NewestQC:    s.initialQC,
	}
	s.persist.On("GetLivenessData").Return(livenessData, nil)

	// init PaceMaker and start
	s.paceMaker, err = New(timeout.NewController(s.timeoutConf), NoProposalDelay(), s.notifier, s.persist)
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

// Test_Initialization tests QCs and TCs provided as optional constructor arguments.
// We want to test that nil, old and duplicate TCs & QCs are accepted in arbitrary order.
// The constructed PaceMaker should be in the state:
//   - in view V+1, where V is the _largest view of _any_ of the ingested QCs and TCs
//   - method `NewestQC` should report the QC with the highest View in _any_ of the inputs
func (s *ActivePaceMakerTestSuite) Test_Initialization() {
	highestView := uint64(0) // highest View of any QC or TC constructed below

	// Randomly create 80 TCs:
	//  * their view is randomly sampled from the range [3, 103)
	//  * as we sample 80 times, probability of creating 2 TCs for the same
	//    view is practically 1 (-> birthday problem)
	//  * we place the TCs in a slice of length 110, i.e. some elements are guaranteed to be nil
	//  * Note: we specifically allow for the TC to have the same view as the highest QC.
	//    This is useful as a fallback, because it allows replicas other than the designated
	//     leader to also collect votes and generate a QC.
	tcs := make([]*flow.TimeoutCertificate, 110)
	for i := 0; i < 80; i++ {
		tcView := s.initialView + uint64(rand.Intn(100))
		qcView := 1 + uint64(rand.Intn(int(tcView)))
		tcs[i] = helper.MakeTC(helper.WithTCView(tcView), helper.WithTCNewestQC(QC(qcView)))
		highestView = max(highestView, tcView, qcView)
	}
	rand.Shuffle(len(tcs), func(i, j int) {
		tcs[i], tcs[j] = tcs[j], tcs[i]
	})

	// randomly create 80 QCs (same logic as above)
	qcs := make([]*flow.QuorumCertificate, 110)
	for i := 0; i < 80; i++ {
		qcs[i] = QC(s.initialView + uint64(rand.Intn(100)))
		highestView = max(highestView, qcs[i].View)
	}
	rand.Shuffle(len(qcs), func(i, j int) {
		qcs[i], qcs[j] = qcs[j], qcs[i]
	})

	// set up mocks
	s.persist.On("PutLivenessData", mock.Anything).Return(nil)

	// test that the constructor finds the newest QC and TC
	s.Run("Random TCs and QCs combined", func() {
		pm, err := New(
			timeout.NewController(s.timeoutConf), NoProposalDelay(), s.notifier, s.persist,
			WithQCs(qcs...), WithTCs(tcs...),
		)
		require.NoError(s.T(), err)

		require.Equal(s.T(), highestView+1, pm.CurView())
		if tc := pm.LastViewTC(); tc != nil {
			require.Equal(s.T(), highestView, tc.View)
		} else {
			require.Equal(s.T(), highestView, pm.NewestQC().View)
		}
	})

	// We specifically test an edge case: an outdated TC can still contain a QC that
	// is newer than the newest QC the pacemaker knows so far.
	s.Run("Newest QC in older TC", func() {
		tcs[17] = helper.MakeTC(helper.WithTCView(highestView+20), helper.WithTCNewestQC(QC(highestView+5)))
		tcs[45] = helper.MakeTC(helper.WithTCView(highestView+15), helper.WithTCNewestQC(QC(highestView+12)))

		pm, err := New(
			timeout.NewController(s.timeoutConf), NoProposalDelay(), s.notifier, s.persist,
			WithTCs(tcs...), WithQCs(qcs...),
		)
		require.NoError(s.T(), err)

		// * when observing tcs[17], which is newer than any other QC or TC, the pacemaker should enter view tcs[17].View + 1
		// * when observing tcs[45], which is older than tcs[17], the PaceMaker should notice that the QC in tcs[45]
		//   is newer than its local QC and update it
		require.Equal(s.T(), tcs[17].View+1, pm.CurView())
		require.Equal(s.T(), tcs[17], pm.LastViewTC())
		require.Equal(s.T(), tcs[45].NewestQC, pm.NewestQC())
	})

	// Another edge case: a TC from a past view contains QC for the same view.
	// While is TC is outdated, the contained QC is still newer that the QC the pacemaker knows so far.
	s.Run("Newest QC in older TC", func() {
		tcs[17] = helper.MakeTC(helper.WithTCView(highestView+20), helper.WithTCNewestQC(QC(highestView+5)))
		tcs[45] = helper.MakeTC(helper.WithTCView(highestView+15), helper.WithTCNewestQC(QC(highestView+15)))

		pm, err := New(
			timeout.NewController(s.timeoutConf), NoProposalDelay(), s.notifier, s.persist,
			WithTCs(tcs...), WithQCs(qcs...),
		)
		require.NoError(s.T(), err)

		// * when observing tcs[17], which is newer than any other QC or TC, the pacemaker should enter view tcs[17].View + 1
		// * when observing tcs[45], which is older than tcs[17], the PaceMaker should notice that the QC in tcs[45]
		//   is newer than its local QC and update it
		require.Equal(s.T(), tcs[17].View+1, pm.CurView())
		require.Equal(s.T(), tcs[17], pm.LastViewTC())
		require.Equal(s.T(), tcs[45].NewestQC, pm.NewestQC())
	})

	// Verify that WithTCs still works correctly if no TCs are given:
	// the list of TCs is empty or all contained TCs are nil
	s.Run("Only nil TCs", func() {
		pm, err := New(timeout.NewController(s.timeoutConf), NoProposalDelay(), s.notifier, s.persist, WithTCs())
		require.NoError(s.T(), err)
		require.Equal(s.T(), s.initialView, pm.CurView())

		pm, err = New(timeout.NewController(s.timeoutConf), NoProposalDelay(), s.notifier, s.persist, WithTCs(nil, nil, nil))
		require.NoError(s.T(), err)
		require.Equal(s.T(), s.initialView, pm.CurView())
	})

	// Verify that WithQCs still works correctly if no QCs are given:
	// the list of QCs is empty or all contained QCs are nil
	s.Run("Only nil QCs", func() {
		pm, err := New(timeout.NewController(s.timeoutConf), NoProposalDelay(), s.notifier, s.persist, WithQCs())
		require.NoError(s.T(), err)
		require.Equal(s.T(), s.initialView, pm.CurView())

		pm, err = New(timeout.NewController(s.timeoutConf), NoProposalDelay(), s.notifier, s.persist, WithQCs(nil, nil, nil))
		require.NoError(s.T(), err)
		require.Equal(s.T(), s.initialView, pm.CurView())
	})

}

// TestProposalDuration tests that the active pacemaker forwards proposal duration values from the provider.
func (s *ActivePaceMakerTestSuite) TestProposalDuration() {
	proposalDurationProvider := NewStaticProposalDurationProvider(time.Millisecond * 500)
	pm, err := New(timeout.NewController(s.timeoutConf), &proposalDurationProvider, s.notifier, s.persist)
	require.NoError(s.T(), err)

	now := time.Now().UTC()
	assert.Equal(s.T(), now.Add(time.Millisecond*500), pm.TargetPublicationTime(117, now, unittest.IdentifierFixture()))
	proposalDurationProvider.dur = time.Second
	assert.Equal(s.T(), now.Add(time.Second), pm.TargetPublicationTime(117, now, unittest.IdentifierFixture()))
}

func max(a uint64, values ...uint64) uint64 {
	for _, v := range values {
		if v > a {
			a = v
		}
	}
	return a
}
