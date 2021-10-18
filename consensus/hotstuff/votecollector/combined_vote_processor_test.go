package votecollector

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"
	"pgregory.net/rapid"

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
	VoteProcessorTestSuiteBase

	thresholdTotalWeight uint64
	rbSharesTotal        uint64

	packer *mockhotstuff.Packer

	rbSigAggregator *mockhotstuff.WeightedSignatureAggregator
	reconstructor   *mockhotstuff.RandomBeaconReconstructor

	minRequiredShares uint64
	processor         *CombinedVoteProcessor
}

func (s *CombinedVoteProcessorTestSuite) SetupTest() {
	s.VoteProcessorTestSuiteBase.SetupTest()

	s.rbSigAggregator = &mockhotstuff.WeightedSignatureAggregator{}
	s.reconstructor = &mockhotstuff.RandomBeaconReconstructor{}
	s.packer = &mockhotstuff.Packer{}
	s.proposal = helper.MakeProposal()

	s.minRequiredShares = 9 // we require 9 RB shares to reconstruct signature
	s.thresholdTotalWeight, s.rbSharesTotal = 0, 0

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

	s.processor = &CombinedVoteProcessor{
		log:              unittest.Logger(),
		block:            s.proposal.Block,
		stakingSigAggtor: s.stakingAggregator,
		rbSigAggtor:      s.rbSigAggregator,
		rbRector:         s.reconstructor,
		onQCCreated:      s.onQCCreated,
		packer:           s.packer,
		minRequiredStake: s.minRequiredStake,
		done:             *atomic.NewBool(false),
	}
}

// TestInitialState tests that Block() and Status() return correct values after calling constructor
func (s *CombinedVoteProcessorTestSuite) TestInitialState() {
	require.Equal(s.T(), s.proposal.Block, s.processor.Block())
	require.Equal(s.T(), hotstuff.VoteCollectorStatusVerifying, s.processor.Status())
}

// TestProcess_VoteNotForProposal tests that CombinedVoteProcessor accepts only votes for the block it was initialized with
// according to interface specification of `VoteProcessor`, we expect dedicated sentinel errors for votes
// for different views (`VoteForIncompatibleViewError`) _or_ block (`VoteForIncompatibleBlockError`).
func (s *CombinedVoteProcessorTestSuite) TestProcess_VoteNotForProposal() {
	err := s.processor.Process(unittest.VoteFixture(unittest.WithVoteView(s.proposal.Block.View)))
	require.ErrorAs(s.T(), err, &VoteForIncompatibleBlockError)
	require.False(s.T(), model.IsInvalidVoteError(err))

	err = s.processor.Process(unittest.VoteFixture(unittest.WithVoteBlockID(s.proposal.Block.BlockID)))
	require.ErrorAs(s.T(), err, &VoteForIncompatibleViewError)
	require.False(s.T(), model.IsInvalidVoteError(err))

	s.stakingAggregator.AssertNotCalled(s.T(), "Verify")
	s.rbSigAggregator.AssertNotCalled(s.T(), "Verify")
}

// TestProcess_InvalidSignatureFormat ensures that we process signatures only with valid format.
// If we have received vote with signature in invalid format we should return with sentinel error
func (s *CombinedVoteProcessorTestSuite) TestProcess_InvalidSignatureFormat() {
	// signature is random in this case
	vote := unittest.VoteForBlockFixture(s.proposal.Block, func(vote *model.Vote) {
		vote.SigData[0] = byte(42)
	})
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
		return &CombinedVoteProcessor{
			log:              unittest.Logger(),
			block:            s.proposal.Block,
			stakingSigAggtor: stakingAggregator,
			rbSigAggtor:      rbSigAggregator,
			rbRector:         rbReconstructor,
			onQCCreated:      s.onQCCreated,
			packer:           packer,
			minRequiredStake: s.minRequiredStake,
			done:             *atomic.NewBool(false),
		}
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
		require.False(s.T(), model.IsInvalidVoteError(err))
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
		require.False(s.T(), model.IsInvalidVoteError(err))
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
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
	// in this test case we aren't able to pack signatures
	s.Run("pack", func() {
		exception := errors.New("pack-qc-exception")
		packer := &mockhotstuff.Packer{}
		packer.On("Pack", mock.Anything, mock.Anything).Return(nil, nil, exception)
		processor := createProcessor(stakingSigAggregator, thresholdSigAggregator, reconstructor, packer)
		err := processor.Process(vote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
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

// TestProcess_EnoughSharesNotEnoughStakes tests a scenario where we are collecting only threshold signatures
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
	// prepare test setup: 5 votes with staking sigs and 9 votes with random beacon sigs
	stakingSigners := unittest.IdentifierListFixture(5)
	beaconSigners := unittest.IdentifierListFixture(9)

	// setup aggregators and reconstructor
	*s.stakingAggregator = mockhotstuff.WeightedSignatureAggregator{}
	*s.rbSigAggregator = mockhotstuff.WeightedSignatureAggregator{}
	*s.reconstructor = mockhotstuff.RandomBeaconReconstructor{}

	s.stakingAggregator.On("TotalWeight").Return(func() uint64 {
		return s.stakingTotalWeight
	})

	s.rbSigAggregator.On("TotalWeight").Return(func() uint64 {
		return s.thresholdTotalWeight
	})

	s.reconstructor.On("HasSufficientShares").Return(func() bool {
		return s.rbSharesTotal >= s.minRequiredShares
	})

	// mock expected calls to aggregators and reconstructor
	combinedSigs := unittest.SignaturesFixture(3)
	s.stakingAggregator.On("Aggregate").Return(stakingSigners, []byte(combinedSigs[0]), nil).Once()
	s.rbSigAggregator.On("Aggregate").Return(beaconSigners, []byte(combinedSigs[1]), nil).Once()
	s.reconstructor.On("Reconstruct").Return(combinedSigs[2], nil).Once()

	// mock expected call to Packer
	mergedSignerIDs := append(append([]flow.Identifier{}, stakingSigners...), beaconSigners...)
	expectedBlockSigData := &hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners,
		RandomBeaconSigners:          beaconSigners,
		AggregatedStakingSig:         []byte(combinedSigs[0]),
		AggregatedRandomBeaconSig:    []byte(combinedSigs[1]),
		ReconstructedRandomBeaconSig: combinedSigs[2],
	}
	packedSigData := unittest.RandomBytes(128)
	s.packer.On("Pack", s.proposal.Block.BlockID, expectedBlockSigData).Return(mergedSignerIDs, packedSigData, nil).Once()

	// expected QC
	s.onQCCreatedState.On("onQCCreated", mock.Anything).Run(func(args mock.Arguments) {
		qc := args.Get(0).(*flow.QuorumCertificate)
		// ensure that QC contains correct field
		expectedQC := &flow.QuorumCertificate{
			View:      s.proposal.Block.View,
			BlockID:   s.proposal.Block.BlockID,
			SignerIDs: mergedSignerIDs,
			SigData:   packedSigData,
		}
		require.Equal(s.T(), expectedQC, qc)
	}).Return(nil).Once()

	// add votes
	for _, signer := range stakingSigners {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())
		vote.SignerID = signer
		expectedSig := crypto.Signature(vote.SigData[1:])
		s.stakingAggregator.On("Verify", vote.SignerID, expectedSig).Return(nil).Once()
		s.stakingAggregator.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
			s.stakingTotalWeight += s.sigWeight
		}).Return(s.stakingTotalWeight, nil).Once()
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}
	for _, signer := range beaconSigners {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
		vote.SignerID = signer
		expectedSig := crypto.Signature(vote.SigData[1:])
		s.rbSigAggregator.On("Verify", vote.SignerID, expectedSig).Return(nil).Once()
		s.rbSigAggregator.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
			s.thresholdTotalWeight += s.sigWeight
		}).Return(s.thresholdTotalWeight, nil).Once()
		s.reconstructor.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
			s.rbSharesTotal++
		}).Return(func(signerID flow.Identifier, sig crypto.Signature) bool {
			return s.rbSharesTotal >= s.minRequiredShares
		}, func(signerID flow.Identifier, sig crypto.Signature) error {
			return nil
		}).Once()

		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.True(s.T(), s.processor.done.Load())
	s.onQCCreatedState.AssertExpectations(s.T())
	s.rbSigAggregator.AssertExpectations(s.T())
	s.stakingAggregator.AssertExpectations(s.T())
	s.reconstructor.AssertExpectations(s.T())

	// processing extra votes shouldn't result in creating new QCs
	vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
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

// TestCombinedVoteProcessor_PropertyCreatingQC uses property testing to test happy path scenario.
// We randomly draw a committee with some number of staking, random beacon and byzantine nodes.
// Nodes are drawn in a way that have enough voting power to construct a QC.
// In each test iteration we expect to create a valid QC with all provided data as part of constructed QC.
func TestCombinedVoteProcessor_PropertyCreatingQC(testifyT *testing.T) {
	totalParticipants := uint64(17)
	maxBftParticipants := (totalParticipants - 1) / 3

	rapid.Check(testifyT, func(t *rapid.T) {
		// generate a new suite for each run
		s := new(CombinedVoteProcessorTestSuite)
		s.SetT(testifyT)
		s.SetupTest()

		f := rapid.Uint64Range(0, maxBftParticipants).Draw(t, "f").(uint64)
		superMajority := totalParticipants - f
		stakingSignersCount := rapid.Uint64Range(0, superMajority).Draw(t, "stakingSigners").(uint64)
		beaconSignersCount := superMajority - stakingSignersCount
		require.Equal(t, superMajority, stakingSignersCount+beaconSignersCount)

		// we could go here with 2f + 1, but it's simpler for test setup to require all signers to sign
		// and consume their votes.
		s.processor.minRequiredStake = superMajority * s.sigWeight
		s.minRequiredShares = beaconSignersCount

		t.Logf("running conf\n\t"+
			"staking signers: %v, beacon signers: %v\n\t"+
			"required stake: %v, required shares: %v", stakingSignersCount, beaconSignersCount, s.processor.minRequiredStake, s.minRequiredShares)

		// setup aggregators and reconstructor
		*s.stakingAggregator = mockhotstuff.WeightedSignatureAggregator{}
		*s.rbSigAggregator = mockhotstuff.WeightedSignatureAggregator{}
		*s.reconstructor = mockhotstuff.RandomBeaconReconstructor{}

		s.stakingTotalWeight, s.thresholdTotalWeight, s.rbSharesTotal = 0, 0, 0

		s.stakingAggregator.On("TotalWeight").Return(func() uint64 {
			return s.stakingTotalWeight
		})

		s.rbSigAggregator.On("TotalWeight").Return(func() uint64 {
			return s.thresholdTotalWeight
		})

		s.reconstructor.On("HasSufficientShares").Return(func() bool {
			return s.rbSharesTotal >= s.minRequiredShares
		})

		stakingSigners := unittest.IdentifierListFixture(int(stakingSignersCount))
		beaconSigners := unittest.IdentifierListFixture(int(beaconSignersCount))

		// mock expected calls to aggregators and reconstructor
		combinedSigs := unittest.SignaturesFixture(3)
		s.stakingAggregator.On("Aggregate").Return(stakingSigners, []byte(combinedSigs[0]), nil).Once()
		s.rbSigAggregator.On("Aggregate").Return(beaconSigners, []byte(combinedSigs[1]), nil).Once()
		s.reconstructor.On("Reconstruct").Return(combinedSigs[2], nil).Once()

		// mock expected call to Packer
		mergedSignerIDs := append(append([]flow.Identifier{}, stakingSigners...), beaconSigners...)
		expectedBlockSigData := &hotstuff.BlockSignatureData{
			StakingSigners:               stakingSigners,
			RandomBeaconSigners:          beaconSigners,
			AggregatedStakingSig:         []byte(combinedSigs[0]),
			AggregatedRandomBeaconSig:    []byte(combinedSigs[1]),
			ReconstructedRandomBeaconSig: combinedSigs[2],
		}
		packedSigData := unittest.RandomBytes(128)
		s.packer.On("Pack", s.proposal.Block.BlockID, expectedBlockSigData).Return(mergedSignerIDs, packedSigData, nil).Once()

		// expected QC
		s.onQCCreatedState.On("onQCCreated", mock.Anything).Run(func(args mock.Arguments) {
			qc := args.Get(0).(*flow.QuorumCertificate)
			// ensure that QC contains correct field
			expectedQC := &flow.QuorumCertificate{
				View:      s.proposal.Block.View,
				BlockID:   s.proposal.Block.BlockID,
				SignerIDs: mergedSignerIDs,
				SigData:   packedSigData,
			}
			require.Equalf(t, expectedQC, qc, "QC should be equal to what we expect")
		}).Return(nil).Once()

		// add votes
		for _, signer := range stakingSigners {
			vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())
			vote.SignerID = signer
			expectedSig := crypto.Signature(vote.SigData[1:])
			s.stakingAggregator.On("Verify", vote.SignerID, expectedSig).Return(nil).Once()
			s.stakingAggregator.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
				s.stakingTotalWeight += s.sigWeight
			}).Return(s.stakingTotalWeight, nil).Once()
			err := s.processor.Process(vote)
			require.NoError(t, err)
		}
		for _, signer := range beaconSigners {
			vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
			vote.SignerID = signer
			expectedSig := crypto.Signature(vote.SigData[1:])
			s.rbSigAggregator.On("Verify", vote.SignerID, expectedSig).Return(nil).Once()
			s.rbSigAggregator.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
				s.thresholdTotalWeight += s.sigWeight
			}).Return(s.thresholdTotalWeight, nil).Once()
			s.reconstructor.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
				s.rbSharesTotal++
			}).Return(func(signerID flow.Identifier, sig crypto.Signature) bool {
				return s.rbSharesTotal >= s.minRequiredShares
			}, func(signerID flow.Identifier, sig crypto.Signature) error {
				return nil
			}).Once()

			err := s.processor.Process(vote)
			require.NoError(t, err)
		}

		passed := s.processor.done.Load()
		passed = passed && s.onQCCreatedState.AssertExpectations(t)
		passed = passed && s.rbSigAggregator.AssertExpectations(t)
		passed = passed && s.stakingAggregator.AssertExpectations(t)
		passed = passed && s.reconstructor.AssertExpectations(t)

		if !passed {
			t.Fatalf("Assertions weren't met, staking weight: %v, threshold weight: %v, shares collected: %v", s.stakingTotalWeight, s.thresholdTotalWeight, s.rbSharesTotal)
		}

		//processing extra votes shouldn't result in creating new QCs
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
		err := s.processor.Process(vote)
		require.NoError(t, err)
		s.onQCCreatedState.AssertExpectations(t)
	})
}
