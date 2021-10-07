package votecollector

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCombinedVoteProcessor(t *testing.T) {
	suite.Run(t, new(CombinedVoteProcessorTestSuite))
}

// CombinedVoteProcessorTestSuite is a test suite that holds mocked state for isolated testing of CombinedVoteProcessor.
type CombinedVoteProcessorTestSuite struct {
	suite.Suite

	sigWeight            uint64
	stakingTotalWeight   uint64
	thresholdTotalWeight uint64
	rbSharesTotal        uint64
	onQCCreatedState     mock.Mock

	packer            *mockhotstuff.Packer
	stakingAggregator *mockhotstuff.WeightedSignatureAggregator
	rbSigAggregator   *mockhotstuff.WeightedSignatureAggregator
	reconstructor     *mockhotstuff.RandomBeaconReconstructor
	minRequiredStake  uint64
	minRequiredShares uint64
	proposal          *model.Proposal
	processor         *CombinedVoteProcessor
}

func (s *CombinedVoteProcessorTestSuite) SetupTest() {
	s.stakingAggregator = &mockhotstuff.WeightedSignatureAggregator{}
	s.rbSigAggregator = &mockhotstuff.WeightedSignatureAggregator{}
	s.reconstructor = &mockhotstuff.RandomBeaconReconstructor{}
	s.packer = &mockhotstuff.Packer{}
	s.proposal = helper.MakeProposal()
	// let's assume we have 19 nodes each with stake 100
	s.sigWeight = 100
	s.minRequiredStake = 1300 // we require at least 13 sigs to collect min weight
	s.minRequiredShares = 9   // we require 9 RB shares to reconstruct signature
	s.thresholdTotalWeight, s.stakingTotalWeight, s.rbSharesTotal = 0, 0, 0

	// setup staking signature aggregator
	s.stakingAggregator.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.stakingTotalWeight += s.sigWeight
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) uint64 {
		return s.stakingTotalWeight
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	s.stakingAggregator.On("TotalWeight").Return(func() uint64 {
		return s.stakingTotalWeight
	}).Maybe()

	// setup threshold signature aggregator
	s.rbSigAggregator.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.thresholdTotalWeight += s.sigWeight
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) uint64 {
		return s.thresholdTotalWeight
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	s.rbSigAggregator.On("TotalWeight").Return(func() uint64 {
		return s.thresholdTotalWeight
	}).Maybe()

	// setup rb reconstructor
	s.reconstructor.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.rbSharesTotal++
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) bool {
		return s.rbSharesTotal >= s.minRequiredShares
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	s.reconstructor.On("HasSufficientShares").Return(func() bool {
		return s.rbSharesTotal >= s.minRequiredShares
	}).Maybe()

	s.processor = newCombinedVoteProcessor(
		unittest.Logger(),
		s.proposal.Block,
		s.stakingAggregator,
		s.rbSigAggregator,
		s.reconstructor,
		s.onQCCreated,
		s.packer,
		s.minRequiredStake,
	)
}

// onQCCreated is a special function that registers call in mocked state.
// ATTENTION: don't change name of this function since the same name is used in:
// s.onQCCreatedState.On("onQCCreated") statements
func (s *CombinedVoteProcessorTestSuite) onQCCreated(qc *flow.QuorumCertificate) {
	s.onQCCreatedState.Called(qc)
}

// TestInitialState tests that Block() and Status() return correct values after calling constructor
func (s *CombinedVoteProcessorTestSuite) TestInitialState() {
	require.Equal(s.T(), s.proposal.Block, s.processor.Block())
	require.Equal(s.T(), hotstuff.VoteCollectorStatusVerifying, s.processor.Status())
}

// TestProcess_VoteNotForProposal tests that CombinedVoteProcessor accepts only votes for the block it was initialized with. According to interface specification of `VoteProcessor`, we expect dedicated sentinel errors for votes for different views (`VoteForIncompatibleViewError`) _or_ block  (`VoteForIncompatibleBlockError`).
func (s *CombinedVoteProcessorTestSuite) TestProcess_VoteNotForProposal() {
	err := s.processor.Process(unittest.VoteFixture(unittest.WithVoteView(s.proposal.Block.View)))
	require.ErrorAs(s.T(), err, &VoteForIncompatibleBlockError)
	err = s.processor.Process(unittest.VoteFixture(unittest.WithVoteBlockID(s.proposal.Block.BlockID)))
	require.ErrorAs(s.T(), err, &VoteForIncompatibleViewError)
	s.stakingAggregator.AssertNotCalled(s.T(), "Verify")
	s.rbSigAggregator.AssertNotCalled(s.T(), "Verify")
}

// TestProcess_InvalidSignatureFormat ensures that we process signatures only with valid format.
// If we have received vote with signature in invalid format we should return with sentinel error
func (s *CombinedVoteProcessorTestSuite) TestProcess_InvalidSignatureFormat() {
	// signature is random in this case
	vote := unittest.VoteForBlockFixture(s.proposal.Block)
	err := s.processor.Process(vote)
	require.Error(s.T(), err)
	require.True(s.T(), model.IsInvalidVoteError(err))
	require.ErrorAs(s.T(), err, &msig.ErrInvalidFormat)
}

// TestProcess_InvalidSignature tests that CombinedVoteProcessor doesn't collect signatures for votes with invalid signature.
// Checks are made for cases where both staking and threshold signatures were submitted.
func (s *CombinedVoteProcessorTestSuite) TestProcess_InvalidSignature() {
	exception := errors.New("unexpected-exception")
	// test for staking signatures
	s.Run("staking-sig", func() {
		stakingVote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())

		s.stakingAggregator.On("Verify", stakingVote.SignerID, mock.Anything).Return(msig.ErrInvalidFormat).Once()

		// sentinel error from `ErrInvalidFormat` should be wrapped as `InvalidVoteError`
		err := s.processor.Process(stakingVote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &msig.ErrInvalidFormat)

		s.stakingAggregator.On("Verify", stakingVote.SignerID, mock.Anything).Return(exception)

		// unexpected errors from `Verify` should be propagated, but should _not_ be wrapped as `InvalidVoteError`
		err = s.processor.Process(stakingVote)
		require.ErrorIs(s.T(), err, exception)              // unexpected errors from verifying the vote signature should be propagated
		require.False(s.T(), model.IsInvalidVoteError(err)) // but not interpreted as an invalid vote

		s.stakingAggregator.AssertNotCalled(s.T(), "TrustedAdd")
	})
	// test same cases for threshold signature
	s.Run("threshold-sig", func() {
		thresholdVote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())

		s.rbSigAggregator.On("Verify", thresholdVote.SignerID, mock.Anything).Return(msig.ErrInvalidFormat).Once()

		// expect sentinel error in case Verify returns ErrInvalidFormat
		err := s.processor.Process(thresholdVote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &msig.ErrInvalidFormat)

		s.rbSigAggregator.On("Verify", thresholdVote.SignerID, mock.Anything).Return(exception)

		// except exception
		err = s.processor.Process(thresholdVote)
		require.ErrorIs(s.T(), err, exception)              // unexpected errors from verifying the vote signature should be propagated
		require.False(s.T(), model.IsInvalidVoteError(err)) // but not interpreted as an invalid vote

		s.rbSigAggregator.AssertNotCalled(s.T(), "TrustedAdd")
		s.reconstructor.AssertNotCalled(s.T(), "TrustedAdd")
	})

}

// TestProcess_TrustedAddError tests a case where we were able to successfully verify signature but failed to collect it.
func (s *CombinedVoteProcessorTestSuite) TestProcess_TrustedAddError() {
	exception := errors.New("unexpected-exception")
	s.Run("staking-sig", func() {
		stakingVote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())
		*s.stakingAggregator = mockhotstuff.WeightedSignatureAggregator{}
		s.stakingAggregator.On("Verify", stakingVote.SignerID, mock.Anything).Return(nil).Once()
		s.stakingAggregator.On("TrustedAdd", stakingVote.SignerID, mock.Anything).Return(uint64(0), exception).Once()
		err := s.processor.Process(stakingVote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
	s.Run("threshold-sig", func() {
		thresholdVote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
		*s.rbSigAggregator = mockhotstuff.WeightedSignatureAggregator{}
		*s.reconstructor = mockhotstuff.RandomBeaconReconstructor{}
		s.rbSigAggregator.On("Verify", thresholdVote.SignerID, mock.Anything).Return(nil)
		s.rbSigAggregator.On("TrustedAdd", thresholdVote.SignerID, mock.Anything).Return(uint64(0), exception).Once()
		err := s.processor.Process(thresholdVote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
		// test also if reconstructor failed to add it
		s.rbSigAggregator.On("TrustedAdd", thresholdVote.SignerID, mock.Anything).Return(s.sigWeight, nil).Once()
		s.reconstructor.On("TrustedAdd", thresholdVote.SignerID, mock.Anything).Return(false, exception).Once()
		err = s.processor.Process(thresholdVote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
}

// TestProcess_BuildQCError tests all error paths during process of building QC.
// Building QC is a one time operation, we need to make sure that failing in one of the steps leads to exception.
// Since it's a one time operation we need a complicated test to test all conditions.
func (s *CombinedVoteProcessorTestSuite) TestProcess_BuildQCError() {
	mockAggregator := func(aggregator *mockhotstuff.WeightedSignatureAggregator) {
		aggregator.On("Verify", mock.Anything, mock.Anything).Return(nil)
		aggregator.On("TrustedAdd", mock.Anything, mock.Anything).Return(s.minRequiredStake, nil)
		aggregator.On("TotalWeight").Return(s.minRequiredStake)
	}

	stakingSigAggregator := &mockhotstuff.WeightedSignatureAggregator{}
	thresholdSigAggregator := &mockhotstuff.WeightedSignatureAggregator{}
	reconstructor := &mockhotstuff.RandomBeaconReconstructor{}
	packer := &mockhotstuff.Packer{}

	identities := unittest.IdentifierListFixture(5)

	// In this test we will mock all dependencies for happy path, and replace some branches with unhappy path
	// to simulate errors along the branches.

	mockAggregator(stakingSigAggregator)
	stakingSigAggregator.On("Aggregate").Return(identities, unittest.RandomBytes(128), nil)

	mockAggregator(thresholdSigAggregator)
	thresholdSigAggregator.On("Aggregate").Return(identities, unittest.RandomBytes(128), nil)

	reconstructor.On("HasSufficientShares").Return(true)
	reconstructor.On("Reconstruct").Return(unittest.SignatureFixture(), nil)

	packer.On("Pack", mock.Anything, mock.Anything).Return(identities, unittest.RandomBytes(128), nil)

	// Helper factory function to create processors. We need new processor for every test case
	// because QC creation is one time operation and is triggered as soon as we have collected enough weight and shares.
	createProcessor := func(stakingAggregator *mockhotstuff.WeightedSignatureAggregator,
		rbSigAggregator *mockhotstuff.WeightedSignatureAggregator,
		rbReconstructor *mockhotstuff.RandomBeaconReconstructor,
		packer *mockhotstuff.Packer) *CombinedVoteProcessor {
		return newCombinedVoteProcessor(
			unittest.Logger(),
			s.proposal.Block,
			stakingAggregator,
			rbSigAggregator,
			rbReconstructor,
			s.onQCCreated,
			packer,
			s.minRequiredStake,
		)
	}

	vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())

	// in this test case we aren't able to aggregate staking signature
	s.Run("staking-sig-aggregate", func() {
		exception := errors.New("staking-aggregate-exception")
		stakingSigAggregator := &mockhotstuff.WeightedSignatureAggregator{}
		mockAggregator(stakingSigAggregator)
		stakingSigAggregator.On("Aggregate").Return(nil, nil, exception)
		processor := createProcessor(stakingSigAggregator, thresholdSigAggregator, reconstructor, packer)
		err := processor.Process(vote)
		require.ErrorIs(s.T(), err, exception)
	})
	// in this test case we aren't able to aggregate threshold signature
	s.Run("threshold-sig-aggregate", func() {
		exception := errors.New("threshold-aggregate-exception")
		thresholdSigAggregator := &mockhotstuff.WeightedSignatureAggregator{}
		mockAggregator(thresholdSigAggregator)
		thresholdSigAggregator.On("Aggregate").Return(nil, nil, exception)
		processor := createProcessor(stakingSigAggregator, thresholdSigAggregator, reconstructor, packer)
		err := processor.Process(vote)
		require.ErrorIs(s.T(), err, exception)
	})
	// in this test case we aren't able to reconstruct signature
	s.Run("reconstruct", func() {
		exception := errors.New("reconstruct-exception")
		reconstructor := &mockhotstuff.RandomBeaconReconstructor{}
		reconstructor.On("HasSufficientShares").Return(true)
		reconstructor.On("Reconstruct").Return(nil, exception)
		processor := createProcessor(stakingSigAggregator, thresholdSigAggregator, reconstructor, packer)
		err := processor.Process(vote)
		require.ErrorIs(s.T(), err, exception)
	})
	// in this test case we aren't able to pack signatures
	s.Run("pack", func() {
		exception := errors.New("pack-qc-exception")
		packer := &mockhotstuff.Packer{}
		packer.On("Pack", mock.Anything, mock.Anything).Return(nil, nil, exception)
		processor := createProcessor(stakingSigAggregator, thresholdSigAggregator, reconstructor, packer)
		err := processor.Process(vote)
		require.ErrorIs(s.T(), err, exception)
	})
}

// TestProcess_EnoughStakeNotEnoughShares tests a scenario where we first don't have enough stake,
// then we iteratively increase it to the point where we have enough staking weight. No QC should be created
// in this scenario since there is not enough random beacon shares.
func (s *CombinedVoteProcessorTestSuite) TestProcess_EnoughStakeNotEnoughShares() {
	for i := uint64(0); i < s.minRequiredStake; i += s.sigWeight {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.False(s.T(), s.processor.done.Load())
	s.reconstructor.AssertCalled(s.T(), "HasSufficientShares")
	s.onQCCreatedState.AssertNotCalled(s.T(), "onQCCreated")
}

// TestProcess_EnoughStakeNotEnoughShares tests a scenario where we are collecting only threshold signatures
// to the point where we have enough shares to reconstruct RB signature. No QC should be created
// in this scenario since there is not enough staking weight.
func (s *CombinedVoteProcessorTestSuite) TestProcess_EnoughSharesNotEnoughStakes() {
	// change sig weight to be really low, so we don't reach min staking weight while collecting
	// threshold signatures
	s.sigWeight = 10
	for i := uint64(0); i < s.minRequiredShares; i++ {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
		s.rbSigAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.False(s.T(), s.processor.done.Load())
	s.reconstructor.AssertNotCalled(s.T(), "HasSufficientShares")
	s.onQCCreatedState.AssertNotCalled(s.T(), "onQCCreated")
	// verify if we indeed have enough shares
	require.True(s.T(), s.reconstructor.HasSufficientShares())
}

// TestProcess_CreatingQC tests a scenario when we have collected enough staking weight and random beacon shares
// and proceed to build QC. Created QC has to have all signatures and identities aggregated by
// aggregators and packed with consensus packer.
func (s *CombinedVoteProcessorTestSuite) TestProcess_CreatingQC() {
	// generate staking signatures till we reach enough stake
	for i := uint64(0); i < s.minRequiredStake; i += s.sigWeight {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	// prepare for aggregation, as soon as we will collect enough shares we will try to create QC
	stakingSigners := unittest.IdentifierListFixture(7)
	thresholdSigners := unittest.IdentifierListFixture(7)
	expectedSigs := unittest.SignaturesFixture(3)
	s.stakingAggregator.On("Aggregate").Return(stakingSigners, []byte(expectedSigs[0]), nil)
	s.rbSigAggregator.On("Aggregate").Return(thresholdSigners, []byte(expectedSigs[1]), nil)
	s.reconstructor.On("Reconstruct").Return(expectedSigs[2], nil)
	expectedSigData := unittest.RandomBytes(128)

	mergedSignerIDs := make([]flow.Identifier, 0, len(stakingSigners)+len(thresholdSigners))
	// merge both staking and threshold signers into one list
	mergedSignerIDs = append(mergedSignerIDs, stakingSigners...)
	mergedSignerIDs = append(mergedSignerIDs, thresholdSigners...)

	s.packer.On("Pack", s.proposal.Block.BlockID, mock.Anything).Return(mergedSignerIDs, expectedSigData, nil)
	s.onQCCreatedState.On("onQCCreated", mock.Anything).Run(func(args mock.Arguments) {
		qc := args.Get(0).(*flow.QuorumCertificate)
		// ensure that QC contains correct field
		expectedQC := &flow.QuorumCertificate{
			View:      s.proposal.Block.View,
			BlockID:   s.proposal.Block.BlockID,
			SignerIDs: mergedSignerIDs,
			SigData:   qc.SigData,
		}
		require.Equal(s.T(), expectedQC, qc)
	}).Return(nil).Once()

	// generate threshold signatures till we have enough random beacon shares
	for i := uint64(0); i < s.minRequiredShares; i++ {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
		s.rbSigAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.True(s.T(), s.processor.done.Load())
	s.onQCCreatedState.AssertExpectations(s.T())

	// processing extra votes shouldn't result in creating new QCs
	vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
	s.rbSigAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
	err := s.processor.Process(vote)
	require.NoError(s.T(), err)

	s.onQCCreatedState.AssertExpectations(s.T())
}

// TestProcess_ConcurrentCreatingQC tests a scenario where multiple goroutines process vote at same time,
// we expect only one QC created in this scenario.
func (s *CombinedVoteProcessorTestSuite) TestProcess_ConcurrentCreatingQC() {
	stakingSigners := unittest.IdentifierListFixture(10)
	mockAggregator := func(aggregator *mockhotstuff.WeightedSignatureAggregator) {
		aggregator.On("Verify", mock.Anything, mock.Anything).Return(nil)
		aggregator.On("TrustedAdd", mock.Anything, mock.Anything).Return(s.minRequiredStake, nil)
		aggregator.On("TotalWeight").Return(s.minRequiredStake)
		aggregator.On("Aggregate").Return(stakingSigners, unittest.RandomBytes(128), nil)
	}

	// mock aggregators, so we have enough weight and shares for creating QC
	*s.stakingAggregator = mockhotstuff.WeightedSignatureAggregator{}
	mockAggregator(s.stakingAggregator)
	*s.rbSigAggregator = mockhotstuff.WeightedSignatureAggregator{}
	mockAggregator(s.rbSigAggregator)
	*s.reconstructor = mockhotstuff.RandomBeaconReconstructor{}
	s.reconstructor.On("Reconstruct").Return(unittest.SignatureFixture(), nil)
	s.reconstructor.On("HasSufficientShares").Return(true)

	// at this point sending any vote should result in creating QC.
	s.packer.On("Pack", s.proposal.Block.BlockID, mock.Anything).Return(stakingSigners, unittest.RandomBytes(128), nil)
	s.onQCCreatedState.On("onQCCreated", mock.Anything).Return(nil).Once()

	var startupWg, shutdownWg sync.WaitGroup

	vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())
	startupWg.Add(1)
	// prepare goroutines, so they are ready to submit a vote at roughly same time
	for i := 0; i < 5; i++ {
		shutdownWg.Add(1)
		go func() {
			defer shutdownWg.Done()
			startupWg.Wait()
			err := s.processor.Process(vote)
			require.NoError(s.T(), err)
		}()
	}

	startupWg.Done()

	// wait for all routines to finish
	shutdownWg.Wait()

	s.onQCCreatedState.AssertNumberOfCalls(s.T(), "onQCCreated", 1)
}
