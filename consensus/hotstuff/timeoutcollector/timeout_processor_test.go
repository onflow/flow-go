package timeoutcollector

import (
	"errors"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
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
	sigWeight               uint64
	totalWeight             uint64
	onTCCreatedState        mock.Mock
	onPartialTCCreatedState mock.Mock
	processor               *TimeoutProcessor
}

func (s *TimeoutProcessorTestSuite) SetupTest() {
	var err error
	s.sigWeight = 1000
	s.committee = &mocks.Replicas{}
	s.validator = &mocks.Validator{}
	s.sigAggregator = &mocks.TimeoutSignatureAggregator{}
	s.participants = unittest.IdentityListFixture(11, unittest.WithWeight(s.sigWeight))
	s.view = (uint64)(rand.Uint32() + 100)
	s.totalWeight = 0

	s.committee.On("WeightThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(s.participants.TotalWeight()), nil)
	s.sigAggregator.On("View").Return(s.view).Maybe()
	s.sigAggregator.On("VerifyAndAdd", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.totalWeight += s.sigWeight
	}).Return(func(signerID flow.Identifier, sig crypto.Signature, highestQCView uint64) uint64 {
		return s.totalWeight
	}, func(signerID flow.Identifier, sig crypto.Signature, highestQCView uint64) error {
		return nil
	}).Maybe()
	s.sigAggregator.On("TotalWeight").Return(func() uint64 {
		return s.totalWeight
	}).Maybe()

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
		helper.WithTimeoutLastViewTC(helper.MakeTC(helper.WithTCView(s.view)))))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewHighestQCInvalidView tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a proof that sender legitimately entered timeout.View but included HighestQC has older view
// than QC included in TC. For honest nodes this shouldn't happen.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewHighestQCInvalidView() {
	err := s.processor.Process(helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view-10))),
		helper.WithTimeoutLastViewTC(
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
		helper.WithTimeoutLastViewTC(nil)))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_IncludedQCInvalid tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout is well-formed but included QC is invalid
func (s *TimeoutProcessorTestSuite) TestProcess_IncludedQCInvalid() {
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(s.view-1))),
		helper.WithTimeoutLastViewTC(
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
		helper.WithTimeoutLastViewTC(
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
		helper.WithTimeoutLastViewTC(
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

// TestProcess_CreatingTC is a test for happy path single threaded signature aggregation and TC creation
// Each replica commits unique timeout object, this object gets processed by TimeoutProcessor. After collecting
// enough weight we expect a TC to be created. All further operations should be no-op, only one TC should be created.
func (s *TimeoutProcessorTestSuite) TestProcess_CreatingTC() {
	// each signer will commit a different QC
	var highQCViews []uint64
	signers := s.participants[1:]
	for range signers {
		highQCViews = append(highQCViews, s.view-1)
	}
	highestQC := helper.MakeQC(helper.WithQCView(highQCViews[0]))

	// change tracker to require all except one signer to create TC
	s.processor.tcTracker.minRequiredWeight = s.sigWeight * uint64(len(highQCViews))

	expectedNodeIDs := signers.NodeIDs()
	expectedSig := crypto.Signature(unittest.RandomBytes(128))
	s.validator.On("ValidateQC", mock.Anything).Return(nil)
	s.onPartialTCCreatedState.On("onPartialTCCreated", s.view).Return(nil).Once()
	s.onTCCreatedState.On("onTCCreated", mock.Anything).Run(func(args mock.Arguments) {
		tc := args.Get(0).(*flow.TimeoutCertificate)
		// ensure that TC contains correct fields
		expectedTC := &flow.TimeoutCertificate{
			View:          s.view,
			TOHighQCViews: highQCViews,
			TOHighestQC:   highestQC,
			SignerIDs:     expectedNodeIDs,
			SigData:       expectedSig,
		}
		require.Equal(s.T(), expectedTC, tc)
	}).Return(nil).Once()

	s.sigAggregator.On("Aggregate").Return(expectedNodeIDs, highQCViews, expectedSig, nil)

	for _, signer := range expectedNodeIDs {
		timeout := helper.TimeoutObjectFixture(
			helper.WithTimeoutObjectView(s.view),
			helper.WithTimeoutHighestQC(highestQC),
			helper.WithTimeoutObjectSignerID(signer),
			helper.WithTimeoutLastViewTC(nil),
		)
		err := s.processor.Process(timeout)
		require.NoError(s.T(), err)
	}
	s.onTCCreatedState.AssertExpectations(s.T())
	s.sigAggregator.AssertExpectations(s.T())

	// add extra timeout, make sure we don't create another TC
	// should be no-op
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(highestQC),
		helper.WithTimeoutObjectSignerID(s.participants[0].NodeID),
		helper.WithTimeoutLastViewTC(nil),
	)
	err := s.processor.Process(timeout)
	require.NoError(s.T(), err)

	s.onTCCreatedState.AssertNumberOfCalls(s.T(), "onTCCreated", 1)
	s.validator.AssertExpectations(s.T())
}

func (s *TimeoutProcessorTestSuite) TestProcess_ConcurrentCreatingTC() {

}
