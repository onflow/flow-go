package timeoutcollector

import (
	"errors"
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/module/local"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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
	// consider next situation:
	// last successful view was N, after this we weren't able to get a proposal with QC for
	// len(participants) views, but in each view QC was created(but not distributed).
	// In view N+len(participants) each replica contributes with unique highest QC.
	lastSuccessfulQC := helper.MakeQC(helper.WithQCView(s.view - uint64(len(s.participants))))
	lastViewTC := helper.MakeTC(helper.WithTCView(s.view-1),
		helper.WithTCHighestQC(lastSuccessfulQC))

	var highQCViews []uint64
	var timeouts []*model.TimeoutObject
	signers := s.participants[1:]
	for i, signer := range signers {
		qc := helper.MakeQC(helper.WithQCView(lastSuccessfulQC.View + uint64(i+1)))
		highQCViews = append(highQCViews, qc.View)

		timeout := helper.TimeoutObjectFixture(
			helper.WithTimeoutObjectView(s.view),
			helper.WithTimeoutHighestQC(qc),
			helper.WithTimeoutObjectSignerID(signer.NodeID),
			helper.WithTimeoutLastViewTC(lastViewTC),
		)
		timeouts = append(timeouts, timeout)
	}

	// change tracker to require all except one signer to create TC
	s.processor.tcTracker.minRequiredWeight = s.sigWeight * uint64(len(highQCViews))

	expectedNodeIDs := signers.NodeIDs()
	expectedSig := crypto.Signature(unittest.RandomBytes(128))
	s.validator.On("ValidateQC", mock.Anything).Return(nil)
	s.validator.On("ValidateTC", mock.Anything).Return(nil)
	s.onPartialTCCreatedState.On("onPartialTCCreated", s.view).Return(nil).Once()
	s.onTCCreatedState.On("onTCCreated", mock.Anything).Run(func(args mock.Arguments) {
		highestQC := timeouts[len(timeouts)-1].HighestQC
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

	for _, timeout := range timeouts {
		err := s.processor.Process(timeout)
		require.NoError(s.T(), err)
	}
	s.onTCCreatedState.AssertExpectations(s.T())
	s.sigAggregator.AssertExpectations(s.T())

	// add extra timeout, make sure we don't create another TC
	// should be no-op
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutHighestQC(helper.MakeQC(helper.WithQCView(lastSuccessfulQC.View))),
		helper.WithTimeoutObjectSignerID(s.participants[0].NodeID),
		helper.WithTimeoutLastViewTC(nil),
	)
	err := s.processor.Process(timeout)
	require.NoError(s.T(), err)

	s.onTCCreatedState.AssertExpectations(s.T())
	s.validator.AssertExpectations(s.T())
}

// TestProcess_ConcurrentCreatingTC tests a scenario where multiple goroutines process timeout at same time,
// we expect only one TC created in this scenario.
func (s *TimeoutProcessorTestSuite) TestProcess_ConcurrentCreatingTC() {
	s.validator.On("ValidateQC", mock.Anything).Return(nil)
	s.onPartialTCCreatedState.On("onPartialTCCreated", mock.Anything).Return(nil).Once()
	s.onTCCreatedState.On("onTCCreated", mock.Anything).Return(nil).Once()
	s.sigAggregator.On("Aggregate").Return(s.participants.NodeIDs(), []uint64{}, crypto.Signature{}, nil)

	var startupWg, shutdownWg sync.WaitGroup

	highestQC := helper.MakeQC(helper.WithQCView(s.view - 1))

	startupWg.Add(1)
	// prepare goroutines, so they are ready to submit a timeout at roughly same time
	for i, signer := range s.participants {
		shutdownWg.Add(1)
		timeout := helper.TimeoutObjectFixture(
			helper.WithTimeoutObjectView(s.view),
			helper.WithTimeoutHighestQC(highestQC),
			helper.WithTimeoutObjectSignerID(signer.NodeID),
			helper.WithTimeoutLastViewTC(nil),
		)
		go func(i int, timeout *model.TimeoutObject) {
			defer shutdownWg.Done()
			startupWg.Wait()
			err := s.processor.Process(timeout)
			require.NoError(s.T(), err)
		}(i, timeout)
	}

	startupWg.Done()

	// wait for all routines to finish
	shutdownWg.Wait()

	s.onTCCreatedState.AssertNumberOfCalls(s.T(), "onTCCreated", 1)
}

// TestTimeoutProcessor_BuildVerifyTC tests a complete path from creating timeouts to collecting timeouts and then
// building & verifying TC.
// We start with
func TestTimeoutProcessor_BuildVerifyTC(t *testing.T) {
	// signers hold objects that are created with private key and can sign votes and proposals
	signers := make(map[flow.Identifier]*verification.StakingSigner)
	// prepare staking signers, each signer has its own private/public key pair
	stakingSigners := unittest.IdentityListFixture(11, func(identity *flow.Identity) {
		stakingPriv := unittest.StakingPrivKeyFixture()
		identity.StakingPubKey = stakingPriv.PublicKey()

		me, err := local.New(identity, stakingPriv)
		require.NoError(t, err)

		signers[identity.NodeID] = verification.NewStakingSigner(me)
	})

	view := uint64(rand.Uint32() + 100)
	highestQC := helper.MakeQC(helper.WithQCView(view - 2))
	lastViewTC := helper.MakeTC(helper.WithTCView(view-1),
		helper.WithTCHighestQC(highestQC))

	committee := &mocks.Replicas{}
	committee.On("IdentitiesByEpoch", view, mock.Anything).Return(stakingSigners, nil)
	committee.On("WeightThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(stakingSigners.TotalWeight()), nil)

	timeouts := make([]*model.TimeoutObject, 0, len(stakingSigners))

	for _, signer := range stakingSigners {
		timeout, err := signers[signer.NodeID].CreateTimeout(view, highestQC, lastViewTC)
		require.NoError(t, err)
		timeouts = append(timeouts, timeout)
	}

	tcCreated := false
	onTCCreated := func(tc *flow.TimeoutCertificate) {
		verifier := verification.NewStakingVerifier()
		validator := hotstuffvalidator.New(committee. )
	}
}
