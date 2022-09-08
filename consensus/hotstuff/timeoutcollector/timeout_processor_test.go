package timeoutcollector

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTimeoutProcessor(t *testing.T) {
	suite.Run(t, new(TimeoutProcessorTestSuite))
}

// TimeoutProcessorTestSuite is a test suite that holds mocked state for isolated testing of TimeoutProcessor.
type TimeoutProcessorTestSuite struct {
	suite.Suite

	participants  flow.IdentityList
	signer        *flow.Identity
	view          uint64
	sigWeight     uint64
	totalWeight   atomic.Uint64
	committee     *mocks.Replicas
	validator     *mocks.Validator
	sigAggregator *mocks.TimeoutSignatureAggregator
	notifier      *mocks.TimeoutCollectorConsumer
	processor     *TimeoutProcessor
}

func (s *TimeoutProcessorTestSuite) SetupTest() {
	var err error
	s.sigWeight = 1000
	s.committee = mocks.NewReplicas(s.T())
	s.validator = mocks.NewValidator(s.T())
	s.sigAggregator = mocks.NewTimeoutSignatureAggregator(s.T())
	s.notifier = mocks.NewTimeoutCollectorConsumer(s.T())
	s.participants = unittest.IdentityListFixture(11, unittest.WithWeight(s.sigWeight))
	s.signer = s.participants[0]
	s.view = (uint64)(rand.Uint32() + 100)
	s.totalWeight = *atomic.NewUint64(0)

	s.committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(s.participants.TotalWeight()), nil).Maybe()
	s.committee.On("TimeoutThresholdForView", mock.Anything).Return(committees.WeightThresholdToTimeout(s.participants.TotalWeight()), nil).Maybe()
	s.committee.On("IdentityByEpoch", mock.Anything, mock.Anything).Return(s.signer, nil).Maybe()
	s.sigAggregator.On("View").Return(s.view).Maybe()
	s.sigAggregator.On("VerifyAndAdd", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.totalWeight.Add(s.sigWeight)
	}).Return(func(signerID flow.Identifier, sig crypto.Signature, newestQCView uint64) uint64 {
		return s.totalWeight.Load()
	}, func(signerID flow.Identifier, sig crypto.Signature, newestQCView uint64) error {
		return nil
	}).Maybe()
	s.sigAggregator.On("TotalWeight").Return(func() uint64 {
		return s.totalWeight.Load()
	}).Maybe()

	s.processor, err = NewTimeoutProcessor(s.committee,
		s.validator,
		s.sigAggregator,
		s.notifier,
	)
	require.NoError(s.T(), err)
}

// TimeoutLastViewSuccessfulFixture creates a valid timeout if last view has ended with QC.
func (s *TimeoutProcessorTestSuite) TimeoutLastViewSuccessfulFixture(opts ...func(*model.TimeoutObject)) *model.TimeoutObject {
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(s.view-1))),
		helper.WithTimeoutLastViewTC(nil),
	)

	for _, opt := range opts {
		opt(timeout)
	}

	return timeout
}

// TimeoutLastViewFailedFixture creates a valid timeout if last view has ended with TC.
func (s *TimeoutProcessorTestSuite) TimeoutLastViewFailedFixture(opts ...func(*model.TimeoutObject)) *model.TimeoutObject {
	newestQC := helper.MakeQC(helper.WithQCView(s.view - 10))
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(newestQC),
		helper.WithTimeoutLastViewTC(helper.MakeTC(
			helper.WithTCView(s.view-1),
			helper.WithTCNewestQC(helper.MakeQC(helper.WithQCView(newestQC.View))))),
	)

	for _, opt := range opts {
		opt(timeout)
	}

	return timeout
}

// TestProcess_TimeoutNotForView tests that TimeoutProcessor accepts only timeouts for the view it was initialized with
// We expect dedicated sentinel errors for timeouts for different views (`ErrTimeoutForIncompatibleView`).
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutNotForView() {
	err := s.processor.Process(s.TimeoutLastViewSuccessfulFixture(func(t *model.TimeoutObject) {
		t.View++
	}))
	require.ErrorIs(s.T(), err, ErrTimeoutForIncompatibleView)
	require.False(s.T(), model.IsInvalidTimeoutError(err))

	s.sigAggregator.AssertNotCalled(s.T(), "Verify")
}

// TestProcess_TimeoutWithoutQC tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout doesn't contain QC.
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutWithoutQC() {
	err := s.processor.Process(s.TimeoutLastViewSuccessfulFixture(func(t *model.TimeoutObject) {
		t.NewestQC = nil
	}))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_TimeoutNewerHighestQC tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a QC with QC.View > timeout.View, QC can be only with lower view than timeout.
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutNewerHighestQC() {
	s.Run("t.View == t.NewestQC.View", func() {
		err := s.processor.Process(s.TimeoutLastViewSuccessfulFixture(func(t *model.TimeoutObject) {
			t.NewestQC.View = t.View
		}))
		require.True(s.T(), model.IsInvalidTimeoutError(err))
	})
	s.Run("t.View < t.NewestQC.View", func() {
		err := s.processor.Process(s.TimeoutLastViewSuccessfulFixture(func(t *model.TimeoutObject) {
			t.NewestQC.View = t.View + 1
		}))
		require.True(s.T(), model.IsInvalidTimeoutError(err))
	})
}

// TestProcess_LastViewTCWrongView tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a proof that sender legitimately entered timeout.View but it has wrong view meaning he used TC from previous rounds.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewTCWrongView() {
	// if TC is included it must have timeout.View == timeout.LastViewTC.View+1
	err := s.processor.Process(s.TimeoutLastViewFailedFixture(func(t *model.TimeoutObject) {
		t.LastViewTC.View = t.View - 10
	}))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewHighestQCInvalidView tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a proof that sender legitimately entered timeout.View but included HighestQC has older view
// than QC included in TC. For honest nodes this shouldn't happen.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewHighestQCInvalidView() {
	err := s.processor.Process(s.TimeoutLastViewFailedFixture(func(t *model.TimeoutObject) {
		t.LastViewTC.NewestQC.View = t.NewestQC.View + 1 // TC contains newer QC than Timeout Object
	}))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewTCRequiredButNotPresent tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout must contain a proof that sender legitimately entered timeout.View but doesn't have it.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewTCRequiredButNotPresent() {
	// if last view is not successful(timeout.View != timeout.HighestQC.View+1) then this
	// timeout must contain valid timeout.LastViewTC
	err := s.processor.Process(s.TimeoutLastViewFailedFixture(func(t *model.TimeoutObject) {
		t.LastViewTC = nil
	}))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_IncludedQCInvalid tests that TimeoutProcessor correctly handles validation errors if
// timeout is well-formed but included QC is invalid
func (s *TimeoutProcessorTestSuite) TestProcess_IncludedQCInvalid() {
	timeout := s.TimeoutLastViewSuccessfulFixture()

	s.Run("invalid-qc-sentinel", func() {
		*s.validator = *mocks.NewValidator(s.T())
		s.validator.On("ValidateQC", timeout.NewestQC).Return(model.InvalidQCError{}).Once()

		err := s.processor.Process(timeout)
		require.True(s.T(), model.IsInvalidTimeoutError(err))
		require.True(s.T(), model.IsInvalidQCError(err))
	})
	s.Run("invalid-qc-exception", func() {
		exception := errors.New("validate-qc-failed")
		*s.validator = *mocks.NewValidator(s.T())
		s.validator.On("ValidateQC", timeout.NewestQC).Return(exception).Once()

		err := s.processor.Process(timeout)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidTimeoutError(err))
	})
	s.Run("invalid-qc-err-view-for-unknown-epoch", func() {
		*s.validator = *mocks.NewValidator(s.T())
		s.validator.On("ValidateQC", timeout.NewestQC).Return(model.ErrViewForUnknownEpoch).Once()

		err := s.processor.Process(timeout)
		require.False(s.T(), model.IsInvalidTimeoutError(err))
		require.NotErrorIs(s.T(), err, model.ErrViewForUnknownEpoch)
	})
}

// TestProcess_IncludedTCInvalid tests that TimeoutProcessor correctly handles validation errors if
// timeout is well-formed but included TC is invalid
func (s *TimeoutProcessorTestSuite) TestProcess_IncludedTCInvalid() {
	timeout := s.TimeoutLastViewFailedFixture()

	s.Run("invalid-tc-sentinel", func() {
		*s.validator = *mocks.NewValidator(s.T())
		s.validator.On("ValidateQC", timeout.NewestQC).Return(nil)
		s.validator.On("ValidateTC", timeout.LastViewTC).Return(model.InvalidTCError{})

		err := s.processor.Process(timeout)
		require.True(s.T(), model.IsInvalidTimeoutError(err))
		require.True(s.T(), model.IsInvalidTCError(err))
	})
	s.Run("invalid-tc-exception", func() {
		exception := errors.New("validate-tc-failed")
		*s.validator = *mocks.NewValidator(s.T())
		s.validator.On("ValidateQC", timeout.NewestQC).Return(nil)
		s.validator.On("ValidateTC", timeout.LastViewTC).Return(exception).Once()

		err := s.processor.Process(timeout)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidTimeoutError(err))
	})
	s.Run("invalid-tc-err-view-for-unknown-epoch", func() {
		*s.validator = *mocks.NewValidator(s.T())
		s.validator.On("ValidateQC", timeout.NewestQC).Return(nil)
		s.validator.On("ValidateTC", timeout.LastViewTC).Return(model.ErrViewForUnknownEpoch).Once()

		err := s.processor.Process(timeout)
		require.False(s.T(), model.IsInvalidTimeoutError(err))
		require.NotErrorIs(s.T(), err, model.ErrViewForUnknownEpoch)
	})
}

// TestProcess_ValidTimeout tests that processing a valid timeout succeeds without error
func (s *TimeoutProcessorTestSuite) TestProcess_ValidTimeout() {
	s.Run("happy-path", func() {
		timeout := s.TimeoutLastViewSuccessfulFixture()
		s.validator.On("ValidateQC", timeout.NewestQC).Return(nil).Once()
		err := s.processor.Process(timeout)
		require.NoError(s.T(), err)
		s.sigAggregator.AssertCalled(s.T(), "VerifyAndAdd", timeout.SignerID, timeout.SigData, timeout.NewestQC.View)
	})
	s.Run("recovery-path", func() {
		timeout := s.TimeoutLastViewFailedFixture()
		s.validator.On("ValidateQC", timeout.NewestQC).Return(nil).Once()
		s.validator.On("ValidateTC", timeout.LastViewTC).Return(nil).Once()
		err := s.processor.Process(timeout)
		require.NoError(s.T(), err)
		s.sigAggregator.AssertCalled(s.T(), "VerifyAndAdd", timeout.SignerID, timeout.SigData, timeout.NewestQC.View)
	})
}

// TestProcess_VerifyAndAddFailed tests different scenarios when TimeoutSignatureAggregator fails with error.
// We check all sentinel errors and exceptions in this scenario.
func (s *TimeoutProcessorTestSuite) TestProcess_VerifyAndAddFailed() {
	timeout := s.TimeoutLastViewSuccessfulFixture()
	s.validator.On("ValidateQC", timeout.NewestQC).Return(nil)
	s.Run("invalid-signer", func() {
		*s.sigAggregator = *mocks.NewTimeoutSignatureAggregator(s.T())
		s.sigAggregator.On("VerifyAndAdd", mock.Anything, mock.Anything, mock.Anything).
			Return(uint64(0), model.NewInvalidSignerError(fmt.Errorf(""))).Once()
		err := s.processor.Process(timeout)
		require.True(s.T(), model.IsInvalidTimeoutError(err))
		require.True(s.T(), model.IsInvalidSignerError(err))
	})
	s.Run("invalid-signature", func() {
		*s.sigAggregator = *mocks.NewTimeoutSignatureAggregator(s.T())
		s.sigAggregator.On("VerifyAndAdd", mock.Anything, mock.Anything, mock.Anything).
			Return(uint64(0), model.ErrInvalidSignature).Once()
		err := s.processor.Process(timeout)
		require.True(s.T(), model.IsInvalidTimeoutError(err))
		require.ErrorIs(s.T(), err, model.ErrInvalidSignature)
	})
	s.Run("duplicated-signer", func() {
		*s.sigAggregator = *mocks.NewTimeoutSignatureAggregator(s.T())
		s.sigAggregator.On("VerifyAndAdd", mock.Anything, mock.Anything, mock.Anything).
			Return(uint64(0), model.NewDuplicatedSignerErrorf("")).Once()
		err := s.processor.Process(timeout)
		require.True(s.T(), model.IsDuplicatedSignerError(err))
		// this shouldn't be wrapped in invalid timeout
		require.False(s.T(), model.IsInvalidTimeoutError(err))
	})
	s.Run("verify-exception", func() {
		*s.sigAggregator = *mocks.NewTimeoutSignatureAggregator(s.T())
		exception := errors.New("verify-exception")
		s.sigAggregator.On("VerifyAndAdd", mock.Anything, mock.Anything, mock.Anything).
			Return(uint64(0), exception).Once()
		err := s.processor.Process(timeout)
		require.False(s.T(), model.IsInvalidTimeoutError(err))
		require.ErrorIs(s.T(), err, exception)
	})
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
		helper.WithTCNewestQC(lastSuccessfulQC))

	var highQCViews []uint64
	var timeouts []*model.TimeoutObject
	signers := s.participants[1:]
	for i, signer := range signers {
		qc := helper.MakeQC(helper.WithQCView(lastSuccessfulQC.View + uint64(i+1)))
		highQCViews = append(highQCViews, qc.View)

		timeout := helper.TimeoutObjectFixture(
			helper.WithTimeoutObjectView(s.view),
			helper.WithTimeoutNewestQC(qc),
			helper.WithTimeoutObjectSignerID(signer.NodeID),
			helper.WithTimeoutLastViewTC(lastViewTC),
		)
		timeouts = append(timeouts, timeout)
	}

	// change tracker to require all except one signer to create TC
	s.processor.tcTracker.minRequiredWeight = s.sigWeight * uint64(len(highQCViews))

	signerIndices, err := msig.EncodeSignersToIndices(s.participants.NodeIDs(), signers.NodeIDs())
	require.NoError(s.T(), err)
	expectedSig := crypto.Signature(unittest.RandomBytes(128))
	s.validator.On("ValidateQC", mock.Anything).Return(nil)
	s.validator.On("ValidateTC", mock.Anything).Return(nil)
	s.notifier.On("OnPartialTcCreated", s.view, mock.Anything, lastViewTC).Return(nil).Once()
	s.notifier.On("OnTcConstructedFromTimeouts", mock.Anything).Run(func(args mock.Arguments) {
		newestQC := timeouts[len(timeouts)-1].NewestQC
		tc := args.Get(0).(*flow.TimeoutCertificate)
		// ensure that TC contains correct fields
		expectedTC := &flow.TimeoutCertificate{
			View:          s.view,
			NewestQCViews: highQCViews,
			NewestQC:      newestQC,
			SignerIndices: signerIndices,
			SigData:       expectedSig,
		}
		require.Equal(s.T(), expectedTC, tc)
	}).Return(nil).Once()

	s.sigAggregator.On("Aggregate").Return([]flow.Identifier(signers.NodeIDs()), highQCViews, expectedSig, nil)
	s.committee.On("IdentitiesByEpoch", s.view).Return(s.participants, nil)

	for _, timeout := range timeouts {
		err := s.processor.Process(timeout)
		require.NoError(s.T(), err)
	}
	s.notifier.AssertExpectations(s.T())
	s.sigAggregator.AssertExpectations(s.T())

	// add extra timeout, make sure we don't create another TC
	// should be no-op
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(lastSuccessfulQC.View))),
		helper.WithTimeoutObjectSignerID(s.participants[0].NodeID),
		helper.WithTimeoutLastViewTC(nil),
	)
	err = s.processor.Process(timeout)
	require.NoError(s.T(), err)

	s.notifier.AssertExpectations(s.T())
	s.validator.AssertExpectations(s.T())
}

// TestProcess_ConcurrentCreatingTC tests a scenario where multiple goroutines process timeout at same time,
// we expect only one TC created in this scenario.
func (s *TimeoutProcessorTestSuite) TestProcess_ConcurrentCreatingTC() {
	s.validator.On("ValidateQC", mock.Anything).Return(nil)
	s.notifier.On("OnPartialTcCreated", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.notifier.On("OnTcConstructedFromTimeouts", mock.Anything).Return(nil).Once()
	s.sigAggregator.On("Aggregate").Return([]flow.Identifier(s.participants.NodeIDs()), []uint64{}, crypto.Signature{}, nil)
	s.committee.On("IdentitiesByEpoch", mock.Anything).Return(s.participants, nil)

	var startupWg, shutdownWg sync.WaitGroup

	newestQC := helper.MakeQC(helper.WithQCView(s.view - 1))

	startupWg.Add(1)
	// prepare goroutines, so they are ready to submit a timeout at roughly same time
	for i, signer := range s.participants {
		shutdownWg.Add(1)
		timeout := helper.TimeoutObjectFixture(
			helper.WithTimeoutObjectView(s.view),
			helper.WithTimeoutNewestQC(newestQC),
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
}

// TestTimeoutProcessor_BuildVerifyTC tests a complete path from creating timeouts to collecting timeouts and then
// building & verifying TC.
// We start with building valid newest QC that will be included in every TimeoutObject. We need to have a valid QC
// since TimeoutProcessor performs complete validation of TimeoutObject. Then we create a valid cryptographically signed
// timeout for each signer. Created timeouts are feed to TimeoutProcessor which eventually creates a TC after seeing processing
// enough objects. After we verify if TC was correctly constructed and if it doesn't violate protocol rules.
// After obtaining valid TC we will repeat this test case to make sure that TimeoutObject(and TC eventually) with LastViewTC is
// correctly built
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

	// utility function which generates a valid timeout for every signer
	createTimeouts := func(view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) []*model.TimeoutObject {
		timeouts := make([]*model.TimeoutObject, 0, len(stakingSigners))
		for _, signer := range stakingSigners {
			timeout, err := signers[signer.NodeID].CreateTimeout(view, newestQC, lastViewTC)
			require.NoError(t, err)
			timeouts = append(timeouts, timeout)
		}
		return timeouts
	}

	leader := stakingSigners[0]

	view := uint64(rand.Uint32() + 100)
	block := helper.MakeBlock(helper.WithBlockView(view-1),
		helper.WithBlockProposer(leader.NodeID))

	committee := mocks.NewDynamicCommittee(t)
	committee.On("IdentitiesByEpoch", mock.Anything).Return(stakingSigners, nil)
	committee.On("IdentitiesByBlock", mock.Anything).Return(stakingSigners, nil)
	committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(stakingSigners.TotalWeight()), nil)
	committee.On("TimeoutThresholdForView", mock.Anything).Return(committees.WeightThresholdToTimeout(stakingSigners.TotalWeight()), nil)

	proposal, err := signers[leader.NodeID].CreateProposal(block)
	require.NoError(t, err)

	var newestQC *flow.QuorumCertificate
	onQCCreated := func(qc *flow.QuorumCertificate) {
		newestQC = qc
	}

	voteProcessorFactory := votecollector.NewStakingVoteProcessorFactory(committee, onQCCreated)
	voteProcessor, err := voteProcessorFactory.Create(unittest.Logger(), proposal)
	require.NoError(t, err)

	for _, signer := range stakingSigners[1:] {
		vote, err := signers[signer.NodeID].CreateVote(block)
		require.NoError(t, err)
		err = voteProcessor.Process(vote)
		require.NoError(t, err)
	}

	require.NotNil(t, newestQC, "vote processor must create a valid QC at this point")

	// create verifier that will do crypto checks of created TC
	verifier := verification.NewStakingVerifier()
	// create validator which will do compliance and crypto checks of created TC
	validator := hotstuffvalidator.New(committee, verifier)

	var lastViewTC *flow.TimeoutCertificate
	onTCCreated := func(args mock.Arguments) {
		tc := args.Get(0).(*flow.TimeoutCertificate)
		// check if resulted TC is valid
		err := validator.ValidateTC(tc)
		require.NoError(t, err)
		lastViewTC = tc
	}

	aggregator, err := NewTimeoutSignatureAggregator(view, stakingSigners, msig.CollectorTimeoutTag)
	require.NoError(t, err)

	notifier := mocks.NewTimeoutCollectorConsumer(t)
	notifier.On("OnPartialTcCreated", view, newestQC, (*flow.TimeoutCertificate)(nil)).Return().Once()
	notifier.On("OnTcConstructedFromTimeouts", mock.Anything).Run(onTCCreated).Return().Once()
	processor, err := NewTimeoutProcessor(committee, validator, aggregator, notifier)
	require.NoError(t, err)

	// last view was successful, no lastViewTC in this case
	timeouts := createTimeouts(view, newestQC, nil)
	for _, timeout := range timeouts {
		err := processor.Process(timeout)
		require.NoError(t, err)
	}

	notifier.AssertExpectations(t)

	aggregator, err = NewTimeoutSignatureAggregator(view+1, stakingSigners, msig.CollectorTimeoutTag)
	require.NoError(t, err)

	notifier = mocks.NewTimeoutCollectorConsumer(t)
	notifier.On("OnPartialTcCreated", view+1, newestQC, lastViewTC).Return().Once()
	notifier.On("OnTcConstructedFromTimeouts", mock.Anything).Run(onTCCreated).Return().Once()
	processor, err = NewTimeoutProcessor(committee, validator, aggregator, notifier)
	require.NoError(t, err)

	// last view ended with TC, need to include lastViewTC
	timeouts = createTimeouts(view+1, newestQC, lastViewTC)
	for _, timeout := range timeouts {
		err := processor.Process(timeout)
		require.NoError(t, err)
	}

	notifier.AssertExpectations(t)
}
