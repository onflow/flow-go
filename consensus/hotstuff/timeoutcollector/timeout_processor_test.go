package timeoutcollector

import (
	"errors"
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
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTimeoutProcessor(t *testing.T) {
	suite.Run(t, new(TimeoutProcessorTestSuite))
}

// StakingVoteProcessorTestSuite is a test suite that holds mocked state for isolated testing of StakingVoteProcessor.
type TimeoutProcessorTestSuite struct {
	suite.Suite

	participants       flow.IdentityList
	view               uint64
	committee          *mocks.Replicas
	validator          *mocks.Validator
	sigAggregator      *mocks.TimeoutSignatureAggregator
	sigWeight          uint64
	totalWeight        uint64
	onTCCreated        *mocks.OnTCCreated
	onPartialTCCreated *mocks.OnPartialTCCreated
	processor          *TimeoutProcessor
}

func (s *TimeoutProcessorTestSuite) SetupTest() {
	var err error
	s.sigWeight = 1000
	s.committee = &mocks.Replicas{}
	s.validator = &mocks.Validator{}
	s.sigAggregator = &mocks.TimeoutSignatureAggregator{}
	s.onTCCreated = mocks.NewOnTCCreated(s.T())
	s.onPartialTCCreated = mocks.NewOnPartialTCCreated(s.T())
	s.participants = unittest.IdentityListFixture(11, unittest.WithWeight(s.sigWeight))
	s.view = (uint64)(rand.Uint32() + 100)
	s.totalWeight = 0

	s.committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(s.participants.TotalWeight()), nil)
	s.committee.On("TimeoutThresholdForView", mock.Anything).Return(committees.WeightThresholdToTimeout(s.participants.TotalWeight()), nil)
	s.sigAggregator.On("View").Return(s.view).Maybe()
	s.sigAggregator.On("VerifyAndAdd", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.totalWeight += s.sigWeight
	}).Return(func(signerID flow.Identifier, sig crypto.Signature, newestQCView uint64) uint64 {
		return s.totalWeight
	}, func(signerID flow.Identifier, sig crypto.Signature, newestQCView uint64) error {
		return nil
	}).Maybe()
	s.sigAggregator.On("TotalWeight").Return(func() uint64 {
		return s.totalWeight
	}).Maybe()

	s.processor, err = NewTimeoutProcessor(s.committee,
		s.validator,
		s.sigAggregator,
		s.onPartialTCCreated.Execute,
		s.onTCCreated.Execute,
	)
	require.NoError(s.T(), err)
}

// TestProcess_TimeoutNotForView tests that TimeoutProcessor accepts only timeouts for the view it was initialized with
// We expect dedicated sentinel errors for timeouts for different views (`ErrTimeoutForIncompatibleView`).
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutNotForView() {
	err := s.processor.Process(helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view + 1)))
	require.ErrorIs(s.T(), err, ErrTimeoutForIncompatibleView)
	require.False(s.T(), model.IsInvalidTimeoutError(err))

	s.sigAggregator.AssertNotCalled(s.T(), "Verify")
}

// TestProcess_TimeoutWithoutQC tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout doesn't contain QC.
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutWithoutQC() {
	err := s.processor.Process(helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(nil)))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_TimeoutNewerHighestQC tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a QC with QC.View > timeout.View, QC can be only with lower view than timeout.
func (s *TimeoutProcessorTestSuite) TestProcess_TimeoutNewerHighestQC() {
	err := s.processor.Process(helper.TimeoutObjectFixture(helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(s.view)))))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewTCRequiredButNotPresent tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a proof that sender legitimately entered timeout.View but it has wrong view meaning he used TC from previous rounds.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewTCWrongView() {
	// if TC is included it must have timeout.View == timeout.LastViewTC.View+1
	err := s.processor.Process(helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(s.view-10))),
		helper.WithTimeoutLastViewTC(helper.MakeTC(helper.WithTCView(s.view)))))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewHighestQCInvalidView tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout contains a proof that sender legitimately entered timeout.View but included HighestQC has older view
// than QC included in TC. For honest nodes this shouldn't happen.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewHighestQCInvalidView() {
	err := s.processor.Process(helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(s.view-10))),
		helper.WithTimeoutLastViewTC(
			helper.MakeTC(
				helper.WithTCView(s.view-1),
				helper.WithTCNewestQC(helper.MakeQC(helper.WithQCView(s.view-5)))))))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_LastViewTCRequiredButNotPresent tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout must contain a proof that sender legitimately entered timeout.View but doesn't have it.
func (s *TimeoutProcessorTestSuite) TestProcess_LastViewTCRequiredButNotPresent() {
	// if last view is not successful(timeout.View != timeout.HighestQC.View+1) then this
	// timeout must contain valid timeout.LastViewTC
	err := s.processor.Process(helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(s.view-10))),
		helper.WithTimeoutLastViewTC(nil)))
	require.True(s.T(), model.IsInvalidTimeoutError(err))
}

// TestProcess_IncludedQCInvalid tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout is well-formed but included QC is invalid
func (s *TimeoutProcessorTestSuite) TestProcess_IncludedQCInvalid() {
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(s.view-1))),
		helper.WithTimeoutLastViewTC(
			helper.MakeTC(helper.WithTCView(s.view-1),
				helper.WithTCNewestQC(helper.MakeQC(helper.WithQCView(s.view-1))))),
	)

	exception := errors.New("validate-qc-failed")
	s.validator.On("ValidateQC", timeout.NewestQC).Return(exception)

	err := s.processor.Process(timeout)
	require.True(s.T(), model.IsInvalidTimeoutError(err))
	require.ErrorIs(s.T(), err, exception)
}

// TestProcess_IncludedTCInvalid tests that TimeoutProcessor fails with model.InvalidTimeoutError if
// timeout is well-formed but included TC is invalid
func (s *TimeoutProcessorTestSuite) TestProcess_IncludedTCInvalid() {
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(s.view-1))),
		helper.WithTimeoutLastViewTC(
			helper.MakeTC(helper.WithTCView(s.view-1),
				helper.WithTCNewestQC(helper.MakeQC(helper.WithQCView(s.view-1))))),
	)

	exception := errors.New("validate-qc-failed")
	s.validator.On("ValidateQC", timeout.NewestQC).Return(nil)
	s.validator.On("ValidateTC", timeout.LastViewTC).Return(exception)

	err := s.processor.Process(timeout)
	require.True(s.T(), model.IsInvalidTimeoutError(err))
	require.ErrorIs(s.T(), err, exception)
}

// TestProcess_ValidTimeout tests that processing a valid timeout succeeds without error
func (s *TimeoutProcessorTestSuite) TestProcess_ValidTimeout() {
	timeout := helper.TimeoutObjectFixture(
		helper.WithTimeoutObjectView(s.view),
		helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(s.view-1))),
		helper.WithTimeoutLastViewTC(
			helper.MakeTC(helper.WithTCView(s.view-1),
				helper.WithTCNewestQC(helper.MakeQC(helper.WithQCView(s.view-1))))),
	)

	s.validator.On("ValidateQC", timeout.NewestQC).Return(nil)
	s.validator.On("ValidateTC", timeout.LastViewTC).Return(nil)
	s.sigAggregator.On("VerifyAndAdd", timeout.SignerID, timeout.SigData, timeout.NewestQC.View).Return(uint64(0), nil)

	err := s.processor.Process(timeout)
	require.NoError(s.T(), err)
	s.validator.AssertCalled(s.T(), "ValidateQC", timeout.NewestQC)
	s.validator.AssertCalled(s.T(), "ValidateTC", timeout.LastViewTC)
	s.sigAggregator.AssertCalled(s.T(), "VerifyAndAdd", timeout.SignerID, timeout.SigData, timeout.NewestQC.View)
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

	signerIndices, err := signature.EncodeSignersToIndices(s.participants.NodeIDs(), signers.NodeIDs())
	require.NoError(s.T(), err)
	expectedSig := crypto.Signature(unittest.RandomBytes(128))
	s.validator.On("ValidateQC", mock.Anything).Return(nil)
	s.validator.On("ValidateTC", mock.Anything).Return(nil)
	s.onPartialTCCreated.On("Execute", s.view).Return(nil).Once()
	s.onTCCreated.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
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
	s.onTCCreated.AssertExpectations(s.T())
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

	s.onTCCreated.AssertExpectations(s.T())
	s.validator.AssertExpectations(s.T())
}

// TestProcess_ConcurrentCreatingTC tests a scenario where multiple goroutines process timeout at same time,
// we expect only one TC created in this scenario.
func (s *TimeoutProcessorTestSuite) TestProcess_ConcurrentCreatingTC() {
	s.validator.On("ValidateQC", mock.Anything).Return(nil)
	s.onPartialTCCreated.On("Execute", mock.Anything).Return(nil).Once()
	s.onTCCreated.On("Execute", mock.Anything).Return(nil).Once()
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
//
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
	// create validator which will do compliance and crypto checked of created TC
	validator := hotstuffvalidator.New(committee, verifier)

	var lastViewTC *flow.TimeoutCertificate
	onTCCreated := mocks.NewOnTCCreated(t)
	onTCCreated.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
		tc := args.Get(0).(*flow.TimeoutCertificate)
		// check if resulted TC is valid
		err := validator.ValidateTC(tc)
		require.NoError(t, err)
		lastViewTC = tc
	}).Times(2) // should be called twice

	aggregator, err := NewTimeoutSignatureAggregator(view, stakingSigners, encoding.CollectorVoteTag)
	require.NoError(t, err)

	onPartialTCCreated := mocks.NewOnPartialTCCreated(t)
	onPartialTCCreated.On("Execute", view).Once()
	processor, err := NewTimeoutProcessor(committee, validator, aggregator, onPartialTCCreated.Execute, onTCCreated.Execute)
	require.NoError(t, err)

	// last view was successful, no lastViewTC in this case
	timeouts := createTimeouts(view, newestQC, nil)
	for _, timeout := range timeouts {
		err := processor.Process(timeout)
		require.NoError(t, err)
	}

	onTCCreated.AssertNumberOfCalls(t, "Execute", 1)

	aggregator, err = NewTimeoutSignatureAggregator(view+1, stakingSigners, encoding.CollectorVoteTag)
	require.NoError(t, err)

	onPartialTCCreated.On("Execute", view+1).Once()
	processor, err = NewTimeoutProcessor(committee, validator, aggregator, onPartialTCCreated.Execute, onTCCreated.Execute)
	require.NoError(t, err)

	// last view ended with TC, need to include lastViewTC
	timeouts = createTimeouts(view+1, newestQC, lastViewTC)
	for _, timeout := range timeouts {
		err := processor.Process(timeout)
		require.NoError(t, err)
	}

	onTCCreated.AssertNumberOfCalls(t, "Execute", 2)
}
