package timeoutcollector

import (
	"errors"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/unittest"
	"golang.org/x/exp/rand"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
)

func TestTimeoutProcessor(t *testing.T) {
	suite.Run(t, new(TimeoutProcessorTestSuite))
}

// StakingVoteProcessorTestSuite is a test suite that holds mocked state for isolated testing of StakingVoteProcessor.
type TimeoutProcessorTestSuite struct {
	suite.Suite

	participants            flow.IdentityList
	view                    uint64
	committee               *mocks.Replicas
	validator               *mocks.Validator
	sigAggregator           *mocks.TimeoutSignatureAggregator
	onTCCreatedState        mock.Mock
	onPartialTCCreatedState mock.Mock
	processor               *TimeoutProcessor
}

func (s *TimeoutProcessorTestSuite) SetupTest() {
	var err error
	s.committee = &mocks.Replicas{}
	s.validator = &mocks.Validator{}
	s.sigAggregator = &mocks.TimeoutSignatureAggregator{}
	s.participants = unittest.IdentityListFixture(11)
	s.view = (uint64)(rand.Uint32() + 100)

	s.committee.On("WeightThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(s.participants.TotalWeight()), nil)
	s.sigAggregator.On("View").Return(s.view).Maybe()

	s.processor, err = NewTimeoutProcessor(s.committee,
		s.validator,
		s.sigAggregator,
		s.onPartialTCCreated,
		s.onTCCreated,
	)
	require.NoError(s.T(), err)
}

// onQCCreated is a special function that registers call in mocked state.
// ATTENTION: don't change name of this function since the same name is used in:
// s.onTCCreatedState.On("onTCCreated") statements
func (s *TimeoutProcessorTestSuite) onTCCreated(tc *flow.TimeoutCertificate) {
	s.onTCCreatedState.Called(tc)
}

// onQCCreated is a special function that registers call in mocked state.
// ATTENTION: don't change name of this function since the same name is used in:
// s.onPartialTCCreatedState.On("onPartialTCCreated") statements
func (s *TimeoutProcessorTestSuite) onPartialTCCreated(view uint64) {
	s.onPartialTCCreatedState.Called(view)
}

// TestProcess_TimeoutNotForView tests that TimeoutProcessor accepts only timeouts for the view it was initialized with
// We expect dedicated sentinel errors for timeouts for different views (`ErrTimeoutForIncompatibleView`).
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutNotForView() {
	err := s.processor.Process(helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view + 1)))
	require.ErrorAs(s.T(), err, &ErrTimeoutForIncompatibleView)
	require.False(s.T(), model.IsInvalidTimeoutError(err))

	s.sigAggregator.AssertNotCalled(s.T(), "Verify")
}

// TestProcess_TimeoutWithoutQC tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout doesn't contain QC.
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutWithoutQC() {
	err := s.processor.Process(helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(nil)))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_TimeoutNewerHighestQC tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a QC with QC.View > timeout.View, QC can be only with lower view than timeout.
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutNewerHighestQC() {
	err := s.processor.Process(helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view)))))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewTCRequiredButNotPresent tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a proof that sender legitimately entered timeout.View but it has wrong view meaning he used TC from previous rounds.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewTCWrongView() {
	// if TC is included it must have timeout.View == timeout.LastViewTC.View+1
	err := s.processor.Process(helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view-10))),
		helper.WIthTimeoutLastViewTC(helper.MakeTC(helper.WithTCView(s.view)))))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewHighestQCInvalidView tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a proof that sender legitimately entered timeout.View but included HighestQC has older view
// than QC included in TC. For honest nodes this shouldn't happen.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewHighestQCInvalidView() {
	err := s.processor.Process(helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view-10))),
		helper.WIthTimeoutLastViewTC(
			helper.MakeTC(
				helper.WithTCView(s.view-1),
				helper.WithTCHighestQC(helper.MakeQC(helper.WithQCView(s.view-5)))))))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewTCRequiredButNotPresent tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout must contain a proof that sender legitimately entered timeout.View but doesn't have it.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewTCRequiredButNotPresent() {
	// if last view is not successful(timeout.View != timeout.HighestQC.View+1) then this
	// timeout must contain valid timeout.LastViewTC
	err := s.processor.Process(helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view-10))),
		helper.WIthTimeoutLastViewTC(nil)))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_IncludedQCInvalid tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout is well-formed but included QC is invalid
func (s *TimeoutProcessorTestSuite) TestProcess_IncludedQCInvalid() {
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view-1))),
		helper.WIthTimeoutLastViewTC(
			helper.MakeTC(helper.WithTCView(s.view-1),
				helper.WithTCHighestQC(helper.MakeQC(helper.WithQCView(s.view-1))))),
	)

	exception := errors.New("validate-qc-failed")
	s.validator.On("ValidateQC", timeout.HighestQC).Return(exception)

	err := s.processor.Process(timeout)
	require.True(s.T(), model.IsInvalidTimeoutError(err))
	require.ErrorAs(s.T(), err, &exception)
}

// TestProcess_IncludedTCInvalid tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout is well-formed but included TC is invalid
func (s *TimeoutProcessorTestSuite) TestProcess_IncludedTCInvalid() {
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view-1))),
		helper.WIthTimeoutLastViewTC(
			helper.MakeTC(helper.WithTCView(s.view-1),
				helper.WithTCHighestQC(helper.MakeQC(helper.WithQCView(s.view-1))))),
	)

	exception := errors.New("validate-qc-failed")
	s.validator.On("ValidateQC", timeout.HighestQC).Return(nil)
	s.validator.On("ValidateTC", timeout.LastViewTC).Return(exception)

	err := s.processor.Process(timeout)
	require.True(s.T(), model.IsInvalidTimeoutError(err))
	require.ErrorAs(s.T(), err, &exception)
}

// TestProcess_ValidTimeout tests that processing a valid timeout succeeds without error
func (s *TimeoutProcessorTestSuite) TestProcess_ValidTimeout() {
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view-1))),
		helper.WIthTimeoutLastViewTC(
			helper.MakeTC(helper.WithTCView(s.view-1),
				helper.WithTCHighestQC(helper.MakeQC(helper.WithQCView(s.view-1))))),
	)

	s.validator.On("ValidateQC", timeout.HighestQC).Return(nil)
	s.validator.On("ValidateTC", timeout.LastViewTC).Return(nil)
	s.sigAggregator.On("VerifyAndAdd", timeout.SignerID, timeout.SigData, timeout.HighestQC.View).Return(uint64(0), nil)

	err := s.processor.Process(timeout)
	require.NoError(s.T(), err)
	s.validator.AssertCalled(s.T(), "ValidateQC", timeout.HighestQC)
	s.validator.AssertCalled(s.T(), "ValidateTC", timeout.LastViewTC)
	s.sigAggregator.AssertCalled(s.T(), "VerifyAndAdd", timeout.SignerID, timeout.SigData, timeout.HighestQC.View)
}
