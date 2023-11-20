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

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
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

	participants  flow.IdentitySkeletonList
	signer        *flow.IdentitySkeleton
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
	s.participants = unittest.IdentityListFixture(11, unittest.WithInitialWeight(s.sigWeight)).Sort(order.Canonical[flow.Identity]).ToSkeleton()
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

	s.processor, err = NewTimeoutProcessor(
		unittest.Logger(),
		s.committee,
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

	signersData := make([]hotstuff.TimeoutSignerInfo, 0)
	for i, signer := range signers.NodeIDs() {
		signersData = append(signersData, hotstuff.TimeoutSignerInfo{
			NewestQCView: highQCViews[i],
			Signer:       signer,
		})
	}
	s.sigAggregator.On("Aggregate").Return(signersData, expectedSig, nil)
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
	s.committee.On("IdentitiesByEpoch", mock.Anything).Return(s.participants, nil)

	signersData := make([]hotstuff.TimeoutSignerInfo, 0, len(s.participants))
	for _, signer := range s.participants.NodeIDs() {
		signersData = append(signersData, hotstuff.TimeoutSignerInfo{
			NewestQCView: 0,
			Signer:       signer,
		})
	}
	// don't care about actual data
	s.sigAggregator.On("Aggregate").Return(signersData, crypto.Signature{}, nil)

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
// This test emulates the most complex scenario where TC consists of TimeoutObjects that are structurally different.
// Let's consider a case where at some view N consensus committee generated both QC and TC, resulting in nodes differently entering view N+1.
// When constructing TC for view N+1 some replicas will contribute with TO{View:N+1, NewestQC.View: N, LastViewTC: nil}
// while others with TO{View:N+1, NewestQC.View: N-1, LastViewTC: TC{View: N, NewestQC.View: N-1}}.
// This results in multi-message BLS signature with messages picked from set M={N-1,N}.
// We have to be able to construct a valid TC for view N+1 and successfully validate it.
// We start by building a valid QC for view N-1, that will be included in every TimeoutObject at view N.
// Right after we create a valid QC for view N. We need to have valid QCs since TimeoutProcessor performs complete validation of TimeoutObject.
// Then we create a valid cryptographically signed timeout for each signer. Created timeouts are feed to TimeoutProcessor
// which eventually creates a TC after seeing processing enough objects. After we verify if TC was correctly constructed
// and if it doesn't violate protocol rules. At this point we have QC for view N-1, both QC and TC for view N.
// After constructing valid objects we will repeat TC creation process and create a TC for view N+1 where replicas contribute
// with structurally different TimeoutObjects to make sure that TC is correctly built and can be successfully validated.
func TestTimeoutProcessor_BuildVerifyTC(t *testing.T) {
	// signers hold objects that are created with private key and can sign votes and proposals
	signers := make(map[flow.Identifier]*verification.StakingSigner)
	// prepare staking signers, each signer has its own private/public key pair
	// identities must be in canonical order
	stakingSigners := unittest.IdentityListFixture(11, func(identity *flow.Identity) {
		stakingPriv := unittest.StakingPrivKeyFixture()
		identity.StakingPubKey = stakingPriv.PublicKey()

		me, err := local.New(identity.IdentitySkeleton, stakingPriv)
		require.NoError(t, err)

		signers[identity.NodeID] = verification.NewStakingSigner(me)
	}).Sort(order.Canonical[flow.Identity])

	// utility function which generates a valid timeout for every signer
	createTimeouts := func(participants flow.IdentitySkeletonList, view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) []*model.TimeoutObject {
		timeouts := make([]*model.TimeoutObject, 0, len(participants))
		for _, signer := range participants {
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

	stakingSignersSkeleton := stakingSigners.ToSkeleton()

	committee := mocks.NewDynamicCommittee(t)
	committee.On("IdentitiesByEpoch", mock.Anything).Return(stakingSignersSkeleton, nil)
	committee.On("IdentitiesByBlock", mock.Anything).Return(stakingSigners, nil)
	committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(stakingSignersSkeleton.TotalWeight()), nil)
	committee.On("TimeoutThresholdForView", mock.Anything).Return(committees.WeightThresholdToTimeout(stakingSignersSkeleton.TotalWeight()), nil)

	// create first QC for view N-1, this will be our olderQC
	olderQC := createRealQC(t, committee, stakingSignersSkeleton, signers, block)
	// now create a second QC for view N, this will be our newest QC
	nextBlock := helper.MakeBlock(
		helper.WithBlockView(view),
		helper.WithBlockProposer(leader.NodeID),
		helper.WithBlockQC(olderQC))
	newestQC := createRealQC(t, committee, stakingSignersSkeleton, signers, nextBlock)

	// At this point we have created two QCs for round N-1 and N.
	// Next step is create a TC for view N.

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

	aggregator, err := NewTimeoutSignatureAggregator(view, stakingSignersSkeleton, msig.CollectorTimeoutTag)
	require.NoError(t, err)

	notifier := mocks.NewTimeoutCollectorConsumer(t)
	notifier.On("OnPartialTcCreated", view, olderQC, (*flow.TimeoutCertificate)(nil)).Return().Once()
	notifier.On("OnTcConstructedFromTimeouts", mock.Anything).Run(onTCCreated).Return().Once()
	processor, err := NewTimeoutProcessor(unittest.Logger(), committee, validator, aggregator, notifier)
	require.NoError(t, err)

	// last view was successful, no lastViewTC in this case
	timeouts := createTimeouts(stakingSignersSkeleton, view, olderQC, nil)
	for _, timeout := range timeouts {
		err := processor.Process(timeout)
		require.NoError(t, err)
	}

	notifier.AssertExpectations(t)

	// at this point we have created QCs for view N-1 and N additionally a TC for view N, we can create TC for view N+1
	// with timeout objects containing both QC and TC for view N

	aggregator, err = NewTimeoutSignatureAggregator(view+1, stakingSignersSkeleton, msig.CollectorTimeoutTag)
	require.NoError(t, err)

	notifier = mocks.NewTimeoutCollectorConsumer(t)
	notifier.On("OnPartialTcCreated", view+1, newestQC, (*flow.TimeoutCertificate)(nil)).Return().Once()
	notifier.On("OnTcConstructedFromTimeouts", mock.Anything).Run(onTCCreated).Return().Once()
	processor, err = NewTimeoutProcessor(unittest.Logger(), committee, validator, aggregator, notifier)
	require.NoError(t, err)

	// part of committee will use QC, another part TC, this will result in aggregated signature consisting
	// of two types of messages with views N-1 and N representing the newest QC known to replicas.
	timeoutsWithQC := createTimeouts(stakingSignersSkeleton[:len(stakingSignersSkeleton)/2], view+1, newestQC, nil)
	timeoutsWithTC := createTimeouts(stakingSignersSkeleton[len(stakingSignersSkeleton)/2:], view+1, olderQC, lastViewTC)
	timeouts = append(timeoutsWithQC, timeoutsWithTC...)
	for _, timeout := range timeouts {
		err := processor.Process(timeout)
		require.NoError(t, err)
	}

	notifier.AssertExpectations(t)
}

// createRealQC is a helper function which generates a properly signed QC with real signatures for given block.
func createRealQC(
	t *testing.T,
	committee hotstuff.DynamicCommittee,
	signers flow.IdentitySkeletonList,
	signerObjects map[flow.Identifier]*verification.StakingSigner,
	block *model.Block,
) *flow.QuorumCertificate {
	leader := signers[0]
	proposal, err := signerObjects[leader.NodeID].CreateProposal(block)
	require.NoError(t, err)

	var createdQC *flow.QuorumCertificate
	onQCCreated := func(qc *flow.QuorumCertificate) {
		createdQC = qc
	}

	voteProcessorFactory := votecollector.NewStakingVoteProcessorFactory(committee, onQCCreated)
	voteProcessor, err := voteProcessorFactory.Create(unittest.Logger(), proposal)
	require.NoError(t, err)

	for _, signer := range signers[1:] {
		vote, err := signerObjects[signer.NodeID].CreateVote(block)
		require.NoError(t, err)
		err = voteProcessor.Process(vote)
		require.NoError(t, err)
	}

	require.NotNil(t, createdQC, "vote processor must create a valid QC at this point")
	return createdQC
}
