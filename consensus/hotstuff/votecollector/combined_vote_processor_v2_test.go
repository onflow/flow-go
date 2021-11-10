package votecollector

import (
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

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

func TestCombinedVoteProcessorV2(t *testing.T) {
	suite.Run(t, new(CombinedVoteProcessorV2TestSuite))
}

// CombinedVoteProcessorV2TestSuite is a test suite that holds mocked state for isolated testing of CombinedVoteProcessorV2.
type CombinedVoteProcessorV2TestSuite struct {
	VoteProcessorTestSuiteBase

	rbSharesTotal uint64

	packer *mockhotstuff.Packer

	reconstructor *mockhotstuff.RandomBeaconReconstructor

	minRequiredShares uint64
	processor         *CombinedVoteProcessorV2
}

func (s *CombinedVoteProcessorV2TestSuite) SetupTest() {
	s.VoteProcessorTestSuiteBase.SetupTest()

	s.reconstructor = &mockhotstuff.RandomBeaconReconstructor{}
	s.packer = &mockhotstuff.Packer{}
	s.proposal = helper.MakeProposal()

	s.minRequiredShares = 9 // we require 9 RB shares to reconstruct signature
	s.rbSharesTotal = 0

	// setup rb reconstructor
	s.reconstructor.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.rbSharesTotal++
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) bool {
		return s.rbSharesTotal >= s.minRequiredShares
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	s.reconstructor.On("EnoughShares").Return(func() bool {
		return s.rbSharesTotal >= s.minRequiredShares
	}).Maybe()

	s.processor = &CombinedVoteProcessorV2{
		log:              unittest.Logger(),
		block:            s.proposal.Block,
		stakingSigAggtor: s.stakingAggregator,
		rbRector:         s.reconstructor,
		onQCCreated:      s.onQCCreated,
		packer:           s.packer,
		minRequiredStake: s.minRequiredStake,
		done:             *atomic.NewBool(false),
	}
}

// TestInitialState tests that Block() and Status() return correct values after calling constructor
func (s *CombinedVoteProcessorV2TestSuite) TestInitialState() {
	require.Equal(s.T(), s.proposal.Block, s.processor.Block())
	require.Equal(s.T(), hotstuff.VoteCollectorStatusVerifying, s.processor.Status())
}

// TestProcess_VoteNotForProposal tests that CombinedVoteProcessorV2 accepts only votes for the block it was initialized with
// according to interface specification of `VoteProcessor`, we expect dedicated sentinel errors for votes
// for different views (`VoteForIncompatibleViewError`) _or_ block (`VoteForIncompatibleBlockError`).
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_VoteNotForProposal() {
	err := s.processor.Process(unittest.VoteFixture(unittest.WithVoteView(s.proposal.Block.View)))
	require.ErrorAs(s.T(), err, &VoteForIncompatibleBlockError)
	require.False(s.T(), model.IsInvalidVoteError(err))

	err = s.processor.Process(unittest.VoteFixture(unittest.WithVoteBlockID(s.proposal.Block.BlockID)))
	require.ErrorAs(s.T(), err, &VoteForIncompatibleViewError)
	require.False(s.T(), model.IsInvalidVoteError(err))

	s.stakingAggregator.AssertNotCalled(s.T(), "Verify")
	s.reconstructor.AssertNotCalled(s.T(), "Verify")
}

// TestProcess_InvalidSignatureFormat ensures that we process signatures only with valid format.
// If we have received vote with signature in invalid format we should return with sentinel error
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_InvalidSignatureFormat() {
	// valid length is 48 or 96
	generator := rapid.IntRange(0, 128).Filter(func(value int) bool {
		return value != 48 && value != 96
	})
	rapid.Check(s.T(), func(t *rapid.T) {
		// create a signature with invalid length
		vote := unittest.VoteForBlockFixture(s.proposal.Block, func(vote *model.Vote) {
			vote.SigData = unittest.RandomBytes(generator.Draw(t, "sig-size").(int))
		})
		err := s.processor.Process(vote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &msig.ErrInvalidFormat)
	})
}

// TestProcess_InvalidSignature tests that CombinedVoteProcessorV2 doesn't collect signatures for votes with invalid signature.
// Checks are made for cases where both staking and staking+beacon signatures were submitted.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_InvalidSignature() {
	exception := errors.New("unexpected-exception")
	// test for staking signatures
	s.Run("staking-sig", func() {
		stakingVote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())
		stakingSig := crypto.Signature(stakingVote.SigData)

		s.stakingAggregator.On("Verify", stakingVote.SignerID, stakingSig).Return(msig.ErrInvalidFormat).Once()

		// sentinel error from `ErrInvalidFormat` should be wrapped as `InvalidVoteError`
		err := s.processor.Process(stakingVote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &msig.ErrInvalidFormat)

		s.stakingAggregator.On("Verify", stakingVote.SignerID, stakingSig).Return(exception)

		// unexpected errors from `Verify` should be propagated, but should _not_ be wrapped as `InvalidVoteError`
		err = s.processor.Process(stakingVote)
		require.ErrorIs(s.T(), err, exception)              // unexpected errors from verifying the vote signature should be propagated
		require.False(s.T(), model.IsInvalidVoteError(err)) // but not interpreted as an invalid vote

		s.stakingAggregator.AssertNotCalled(s.T(), "TrustedAdd")
		// we shouldn't even validate second signature if first one is incorrect
		s.reconstructor.AssertNotCalled(s.T(), "Verify")
		s.reconstructor.AssertNotCalled(s.T(), "TrustedAdd")
	})
	s.Run("staking-double-sig", func() {
		doubleSigVote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		stakingSig := crypto.Signature(doubleSigVote.SigData[:48])

		s.stakingAggregator.On("Verify", doubleSigVote.SignerID, stakingSig).Return(msig.ErrInvalidFormat).Once()

		// sentinel error from `ErrInvalidFormat` should be wrapped as `InvalidVoteError`
		err := s.processor.Process(doubleSigVote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &msig.ErrInvalidFormat)

		s.stakingAggregator.On("Verify", doubleSigVote.SignerID, stakingSig).Return(exception)

		// unexpected errors from `Verify` should be propagated, but should _not_ be wrapped as `InvalidVoteError`
		err = s.processor.Process(doubleSigVote)
		require.ErrorIs(s.T(), err, exception)              // unexpected errors from verifying the vote signature should be propagated
		require.False(s.T(), model.IsInvalidVoteError(err)) // but not interpreted as an invalid vote

		s.stakingAggregator.AssertNotCalled(s.T(), "TrustedAdd")
		// we shouldn't even validate second signature if first one is incorrect
		s.reconstructor.AssertNotCalled(s.T(), "Verify")
		s.reconstructor.AssertNotCalled(s.T(), "TrustedAdd")
	})
	// test same cases for beacon signature
	s.Run("beacon-sig", func() {
		doubleSigVote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		stakingSig := crypto.Signature(doubleSigVote.SigData[:48])
		beaconSig := crypto.Signature(doubleSigVote.SigData[48:])

		// staking sig valid, beacon sig invalid
		s.stakingAggregator.On("Verify", doubleSigVote.SignerID, stakingSig).Return(nil).Once()
		s.reconstructor.On("Verify", doubleSigVote.SignerID, beaconSig).Return(msig.ErrInvalidFormat).Once()

		// expect sentinel error in case Verify returns ErrInvalidFormat
		err := s.processor.Process(doubleSigVote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &msig.ErrInvalidFormat)

		s.stakingAggregator.On("Verify", doubleSigVote.SignerID, stakingSig).Return(nil).Once()
		s.reconstructor.On("Verify", doubleSigVote.SignerID, beaconSig).Return(exception)

		// except exception
		err = s.processor.Process(doubleSigVote)
		require.ErrorIs(s.T(), err, exception)              // unexpected errors from verifying the vote signature should be propagated
		require.False(s.T(), model.IsInvalidVoteError(err)) // but not interpreted as an invalid vote

		s.stakingAggregator.AssertNotCalled(s.T(), "TrustedAdd")
		s.reconstructor.AssertNotCalled(s.T(), "TrustedAdd")
	})
}

// TestProcess_TrustedAddError tests a case where we were able to successfully verify signature but failed to collect it.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_TrustedAddError() {
	exception := errors.New("unexpected-exception")
	s.Run("staking-sig", func() {
		stakingVote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())
		*s.stakingAggregator = mockhotstuff.WeightedSignatureAggregator{}
		s.stakingAggregator.On("Verify", stakingVote.SignerID, mock.Anything).Return(nil).Once()
		s.stakingAggregator.On("TrustedAdd", stakingVote.SignerID, mock.Anything).Return(uint64(0), exception).Once()
		err := s.processor.Process(stakingVote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
	s.Run("beacon-sig", func() {
		doubleSigVote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		*s.stakingAggregator = mockhotstuff.WeightedSignatureAggregator{}
		*s.reconstructor = mockhotstuff.RandomBeaconReconstructor{}

		// first we will collect staking sig
		s.stakingAggregator.On("Verify", doubleSigVote.SignerID, mock.Anything).Return(nil).Once()
		s.stakingAggregator.On("TrustedAdd", doubleSigVote.SignerID, mock.Anything).Return(uint64(0), nil).Once()

		// then beacon sig
		s.reconstructor.On("Verify", doubleSigVote.SignerID, mock.Anything).Return(nil).Once()
		s.reconstructor.On("TrustedAdd", doubleSigVote.SignerID, mock.Anything).Return(false, exception).Once()

		err := s.processor.Process(doubleSigVote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
}

// TestProcess_BuildQCError tests all error paths during process of building QC.
// Building QC is a one time operation, we need to make sure that failing in one of the steps leads to exception.
// Since it's a one time operation we need a complicated test to test all conditions.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_BuildQCError() {
	mockAggregator := func(aggregator *mockhotstuff.WeightedSignatureAggregator) {
		aggregator.On("Verify", mock.Anything, mock.Anything).Return(nil)
		aggregator.On("TrustedAdd", mock.Anything, mock.Anything).Return(s.minRequiredStake, nil)
		aggregator.On("TotalWeight").Return(s.minRequiredStake)
	}

	stakingSigAggregator := &mockhotstuff.WeightedSignatureAggregator{}
	reconstructor := &mockhotstuff.RandomBeaconReconstructor{}
	packer := &mockhotstuff.Packer{}

	identities := unittest.IdentifierListFixture(5)

	// In this test we will mock all dependencies for happy path, and replace some branches with unhappy path
	// to simulate errors along the branches.

	mockAggregator(stakingSigAggregator)
	stakingSigAggregator.On("Aggregate").Return(identities, unittest.RandomBytes(128), nil)

	reconstructor.On("EnoughShares").Return(true)
	reconstructor.On("Reconstruct").Return(unittest.SignatureFixture(), nil)

	packer.On("Pack", mock.Anything, mock.Anything).Return(identities, unittest.RandomBytes(128), nil)

	// Helper factory function to create processors. We need new processor for every test case
	// because QC creation is one time operation and is triggered as soon as we have collected enough weight and shares.
	createProcessor := func(stakingAggregator *mockhotstuff.WeightedSignatureAggregator,
		rbReconstructor *mockhotstuff.RandomBeaconReconstructor,
		packer *mockhotstuff.Packer) *CombinedVoteProcessorV2 {
		return &CombinedVoteProcessorV2{
			log:              unittest.Logger(),
			block:            s.proposal.Block,
			stakingSigAggtor: stakingAggregator,
			rbRector:         rbReconstructor,
			onQCCreated:      s.onQCCreated,
			packer:           packer,
			minRequiredStake: s.minRequiredStake,
			done:             *atomic.NewBool(false),
		}
	}

	vote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())

	// in this test case we aren't able to aggregate staking signature
	s.Run("staking-sig-aggregate", func() {
		exception := errors.New("staking-aggregate-exception")
		stakingSigAggregator := &mockhotstuff.WeightedSignatureAggregator{}
		mockAggregator(stakingSigAggregator)
		stakingSigAggregator.On("Aggregate").Return(nil, nil, exception)
		processor := createProcessor(stakingSigAggregator, reconstructor, packer)
		err := processor.Process(vote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
	// in this test case we aren't able to reconstruct signature
	s.Run("reconstruct", func() {
		exception := errors.New("reconstruct-exception")
		reconstructor := &mockhotstuff.RandomBeaconReconstructor{}
		reconstructor.On("EnoughShares").Return(true)
		reconstructor.On("Reconstruct").Return(nil, exception)
		processor := createProcessor(stakingSigAggregator, reconstructor, packer)
		err := processor.Process(vote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
	// in this test case we aren't able to pack signatures
	s.Run("pack", func() {
		exception := errors.New("pack-qc-exception")
		packer := &mockhotstuff.Packer{}
		packer.On("Pack", mock.Anything, mock.Anything).Return(nil, nil, exception)
		processor := createProcessor(stakingSigAggregator, reconstructor, packer)
		err := processor.Process(vote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
}

// TestProcess_EnoughStakeNotEnoughShares tests a scenario where we first don't have enough stake,
// then we iteratively increase it to the point where we have enough staking weight. No QC should be created
// in this scenario since there is not enough random beacon shares.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_EnoughStakeNotEnoughShares() {
	for i := uint64(0); i < s.minRequiredStake; i += s.sigWeight {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.False(s.T(), s.processor.done.Load())
	s.reconstructor.AssertCalled(s.T(), "EnoughShares")
	s.onQCCreatedState.AssertNotCalled(s.T(), "onQCCreated")
}

// TestProcess_EnoughSharesNotEnoughStakes tests a scenario where we are collecting votes with staking and beacon sigs
// to the point where we have enough shares to reconstruct RB signature. No QC should be created
// in this scenario since there is not enough staking weight.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_EnoughSharesNotEnoughStakes() {
	// change sig weight to be really low, so we don't reach min staking weight while collecting
	// beacon signatures
	s.sigWeight = 10
	for i := uint64(0); i < s.minRequiredShares; i++ {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil).Once()
		s.reconstructor.On("Verify", vote.SignerID, mock.Anything).Return(nil).Once()
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.False(s.T(), s.processor.done.Load())
	s.reconstructor.AssertNotCalled(s.T(), "EnoughShares")
	s.onQCCreatedState.AssertNotCalled(s.T(), "onQCCreated")
	// verify if we indeed have enough shares
	require.True(s.T(), s.reconstructor.EnoughShares())
}

// TestProcess_ConcurrentCreatingQC tests a scenario where multiple goroutines process vote at same time,
// we expect only one QC created in this scenario.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_ConcurrentCreatingQC() {
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
	*s.reconstructor = mockhotstuff.RandomBeaconReconstructor{}
	s.reconstructor.On("Verify", mock.Anything, mock.Anything).Return(nil)
	s.reconstructor.On("Reconstruct").Return(unittest.SignatureFixture(), nil)
	s.reconstructor.On("EnoughShares").Return(true)

	// at this point sending any vote should result in creating QC.
	s.packer.On("Pack", s.proposal.Block.BlockID, mock.Anything).Return(stakingSigners, unittest.RandomBytes(128), nil)
	s.onQCCreatedState.On("onQCCreated", mock.Anything).Return(nil).Once()

	var startupWg, shutdownWg sync.WaitGroup

	vote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())
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

// TestCombinedVoteProcessorV2_PropertyCreatingQCCorrectness uses property testing to test correctness of concurrent votes processing.
// We randomly draw a committee with some number of staking, random beacon and byzantine nodes.
// Values are drawn in a way that 1 <= honestParticipants <= participants <= maxParticipants
// In each test iteration we expect to create a valid QC with all provided data as part of constructed QC.
func TestCombinedVoteProcessorV2_PropertyCreatingQCCorrectness(testifyT *testing.T) {
	maxParticipants := uint64(53)

	rapid.Check(testifyT, func(t *rapid.T) {
		// draw participants in range 1 <= participants <= maxParticipants
		participants := rapid.Uint64Range(1, maxParticipants).Draw(t, "participants").(uint64)
		beaconSignersCount := rapid.Uint64Range(participants/2+1, participants).Draw(t, "beaconSigners").(uint64)
		stakingSignersCount := participants - beaconSignersCount
		require.Equal(t, participants, stakingSignersCount+beaconSignersCount)

		// setup how many votes we need to create a QC
		// 1 <= honestParticipants <= participants <= maxParticipants
		honestParticipants := participants*2/3 + 1
		sigWeight := uint64(100)
		minRequiredStake := honestParticipants * sigWeight

		// proposing block
		block := helper.MakeBlock()

		t.Logf("running conf\n\t"+
			"staking signers: %v, beacon signers: %v\n\t"+
			"required stake: %v", stakingSignersCount, beaconSignersCount, minRequiredStake)

		stakingTotalWeight, collectedShares := uint64(0), atomic.NewUint64(0)

		// setup aggregators and reconstructor
		stakingAggregator := &mockhotstuff.WeightedSignatureAggregator{}
		reconstructor := &mockhotstuff.RandomBeaconReconstructor{}

		stakingSigners := unittest.IdentifierListFixture(int(stakingSignersCount))
		beaconSigners := unittest.IdentifierListFixture(int(beaconSignersCount))

		// lists to track signers that actually contributed their signatures
		var (
			aggregatedStakingSigners []flow.Identifier
		)

		// need separate locks to safely update vectors of voted signers
		stakingAggregatorLock := &sync.Mutex{}

		stakingAggregator.On("TotalWeight").Return(func() uint64 {
			return stakingTotalWeight
		})
		reconstructor.On("EnoughShares").Return(func() bool {
			return collectedShares.Load() >= beaconSignersCount
		})

		// mock expected calls to aggregators and reconstructor
		combinedSigs := unittest.SignaturesFixture(2)
		stakingAggregator.On("Aggregate").Return(
			func() []flow.Identifier {
				stakingAggregatorLock.Lock()
				defer stakingAggregatorLock.Unlock()
				return aggregatedStakingSigners
			},
			func() []byte { return combinedSigs[0] },
			func() error { return nil }).Once()
		reconstructor.On("Reconstruct").Return(combinedSigs[1], nil).Once()

		// mock expected call to Packer
		mergedSignerIDs := make([]flow.Identifier, 0)
		packedSigData := unittest.RandomBytes(128)
		packer := &mockhotstuff.Packer{}
		packer.On("Pack", block.BlockID, mock.Anything).Run(func(args mock.Arguments) {
			blockSigData := args.Get(1).(*hotstuff.BlockSignatureData)

			// check that aggregated signers are part of all votes signers
			// due to concurrent processing it is possible that Aggregate will return less that we have actually aggregated
			// but still enough to construct the QC
			require.Subset(t, aggregatedStakingSigners, blockSigData.StakingSigners)
			require.Nil(t, blockSigData.RandomBeaconSigners)
			require.Nil(t, blockSigData.AggregatedRandomBeaconSig)
			require.GreaterOrEqual(t, uint64(len(blockSigData.StakingSigners)),
				honestParticipants)

			expectedBlockSigData := &hotstuff.BlockSignatureData{
				StakingSigners:               blockSigData.StakingSigners,
				RandomBeaconSigners:          nil,
				AggregatedStakingSig:         []byte(combinedSigs[0]),
				AggregatedRandomBeaconSig:    nil,
				ReconstructedRandomBeaconSig: combinedSigs[1],
			}

			require.Equal(t, expectedBlockSigData, blockSigData)

			// fill merged signers with collected signers
			mergedSignerIDs = append(expectedBlockSigData.StakingSigners, expectedBlockSigData.RandomBeaconSigners...)
		}).Return(
			func(flow.Identifier, *hotstuff.BlockSignatureData) []flow.Identifier { return mergedSignerIDs },
			func(flow.Identifier, *hotstuff.BlockSignatureData) []byte { return packedSigData },
			func(flow.Identifier, *hotstuff.BlockSignatureData) error { return nil }).Once()

		// track if QC was created
		qcCreated := atomic.NewBool(false)

		// expected QC
		onQCCreated := func(qc *flow.QuorumCertificate) {
			// QC should be created only once
			if !qcCreated.CAS(false, true) {
				t.Fatalf("QC created more than once")
			}

			// ensure that QC contains correct field
			expectedQC := &flow.QuorumCertificate{
				View:      block.View,
				BlockID:   block.BlockID,
				SignerIDs: mergedSignerIDs,
				SigData:   packedSigData,
			}
			require.Equalf(t, expectedQC, qc, "QC should be equal to what we expect")
		}

		processor := &CombinedVoteProcessorV2{
			log:              unittest.Logger(),
			block:            block,
			stakingSigAggtor: stakingAggregator,
			rbRector:         reconstructor,
			onQCCreated:      onQCCreated,
			packer:           packer,
			minRequiredStake: minRequiredStake,
			done:             *atomic.NewBool(false),
		}

		votes := make([]*model.Vote, 0, stakingSignersCount+beaconSignersCount)

		expectStakingAggregatorCalls := func(vote *model.Vote) {
			expectedSig := crypto.Signature(vote.SigData[:48])
			stakingAggregator.On("Verify", vote.SignerID, expectedSig).Return(nil).Maybe()
			stakingAggregator.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
				signerID := args.Get(0).(flow.Identifier)
				stakingAggregatorLock.Lock()
				defer stakingAggregatorLock.Unlock()
				stakingTotalWeight += sigWeight
				aggregatedStakingSigners = append(aggregatedStakingSigners, signerID)
			}).Return(uint64(0), nil).Maybe()
		}

		// prepare votes
		for _, signer := range stakingSigners {
			vote := unittest.VoteForBlockFixture(processor.Block(), VoteWithStakingSig())
			vote.SignerID = signer
			// this will set up mock
			expectStakingAggregatorCalls(vote)
			votes = append(votes, vote)
		}
		for _, signer := range beaconSigners {
			vote := unittest.VoteForBlockFixture(processor.Block(), VoteWithDoubleSig())
			vote.SignerID = signer
			expectStakingAggregatorCalls(vote)
			expectedSig := crypto.Signature(vote.SigData[48:])
			reconstructor.On("Verify", vote.SignerID, expectedSig).Return(nil).Maybe()
			reconstructor.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
				collectedShares.Inc()
			}).Return(true, nil).Maybe()
			votes = append(votes, vote)
		}

		// shuffle votes in random order
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(votes), func(i, j int) {
			votes[i], votes[j] = votes[j], votes[i]
		})

		var startProcessing, finishProcessing sync.WaitGroup
		startProcessing.Add(1)
		// process votes concurrently by multiple workers
		for _, vote := range votes {
			finishProcessing.Add(1)
			go func(vote *model.Vote) {
				defer finishProcessing.Done()
				startProcessing.Wait()
				err := processor.Process(vote)
				require.NoError(t, err)
			}(vote)
		}

		// start all goroutines at the same time
		startProcessing.Done()
		finishProcessing.Wait()

		passed := processor.done.Load()
		passed = passed && qcCreated.Load()
		passed = passed && stakingAggregator.AssertExpectations(t)
		passed = passed && reconstructor.AssertExpectations(t)

		if !passed {
			t.Fatalf("Assertions weren't met, staking weight: %v, shares collected: %v", stakingTotalWeight, collectedShares.Load())
		}

		//processing extra votes shouldn't result in creating new QCs
		vote := unittest.VoteForBlockFixture(block, VoteWithDoubleSig())
		err := processor.Process(vote)
		require.NoError(t, err)
	})
}

// TestCombinedVoteProcessorV2_PropertyCreatingQCLiveness uses property testing to test liveness of concurrent votes processing.
// We randomly draw a committee and check if we are able to create a QC with minimal number of nodes.
// In each test iteration we expect to create a QC, we don't check correctness of data since it's checked by another test.
func TestCombinedVoteProcessorV2_PropertyCreatingQCLiveness(testifyT *testing.T) {
	rapid.Check(testifyT, func(t *rapid.T) {
		// draw beacon signers in range 1 <= beaconSignersCount <= 53
		beaconSignersCount := rapid.Uint64Range(1, 53).Draw(t, "beaconSigners").(uint64)
		// draw staking signers in range 0 <= stakingSignersCount <= 10
		stakingSignersCount := rapid.Uint64Range(0, 10).Draw(t, "stakingSigners").(uint64)

		stakingWeightRange, beaconWeightRange := rapid.Uint64Range(1, 10), rapid.Uint64Range(1, 10)

		minRequiredStake := uint64(0)
		// draw weight for each signer randomly
		stakingSigners := unittest.IdentityListFixture(int(stakingSignersCount), func(identity *flow.Identity) {
			identity.Stake = stakingWeightRange.Draw(t, identity.String()).(uint64)
			minRequiredStake += identity.Stake
		})
		beaconSigners := unittest.IdentityListFixture(int(beaconSignersCount), func(identity *flow.Identity) {
			identity.Stake = beaconWeightRange.Draw(t, identity.String()).(uint64)
			minRequiredStake += identity.Stake
		})

		// proposing block
		block := helper.MakeBlock()

		t.Logf("running conf\n\t"+
			"staking signers: %v, beacon signers: %v\n\t"+
			"required stake: %v", stakingSignersCount, beaconSignersCount, minRequiredStake)

		stakingTotalWeight, collectedShares := atomic.NewUint64(0), atomic.NewUint64(0)

		// setup aggregators and reconstructor
		stakingAggregator := &mockhotstuff.WeightedSignatureAggregator{}
		reconstructor := &mockhotstuff.RandomBeaconReconstructor{}

		stakingAggregator.On("TotalWeight").Return(func() uint64 {
			return stakingTotalWeight.Load()
		})
		// don't require shares
		reconstructor.On("EnoughShares").Return(func() bool {
			return collectedShares.Load() >= beaconSignersCount
		})

		// mock expected calls to aggregator and reconstructor
		combinedSigs := unittest.SignaturesFixture(2)
		stakingAggregator.On("Aggregate").Return(stakingSigners.NodeIDs(), []byte(combinedSigs[0]), nil).Once()
		reconstructor.On("Reconstruct").Return(combinedSigs[1], nil).Once()

		// mock expected call to Packer
		mergedSignerIDs := append(stakingSigners.NodeIDs(), beaconSigners.NodeIDs()...)
		packedSigData := unittest.RandomBytes(128)
		packer := &mockhotstuff.Packer{}
		packer.On("Pack", block.BlockID, mock.Anything).Return(mergedSignerIDs, packedSigData, nil)

		// track if QC was created
		qcCreated := atomic.NewBool(false)

		// expected QC
		onQCCreated := func(qc *flow.QuorumCertificate) {
			// QC should be created only once
			if !qcCreated.CAS(false, true) {
				t.Fatalf("QC created more than once")
			}
		}

		processor := &CombinedVoteProcessorV2{
			log:              unittest.Logger(),
			block:            block,
			stakingSigAggtor: stakingAggregator,
			rbRector:         reconstructor,
			onQCCreated:      onQCCreated,
			packer:           packer,
			minRequiredStake: minRequiredStake,
			done:             *atomic.NewBool(false),
		}

		votes := make([]*model.Vote, 0, stakingSignersCount+beaconSignersCount)

		expectStakingAggregatorCalls := func(vote *model.Vote, stake uint64) {
			expectedSig := crypto.Signature(vote.SigData[:48])
			stakingAggregator.On("Verify", vote.SignerID, expectedSig).Return(nil).Maybe()
			stakingAggregator.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
				stakingTotalWeight.Add(stake)
			}).Return(uint64(0), nil).Maybe()
		}

		// prepare votes
		for _, signer := range stakingSigners {
			vote := unittest.VoteForBlockFixture(processor.Block(), VoteWithStakingSig())
			vote.SignerID = signer.ID()
			expectStakingAggregatorCalls(vote, signer.Stake)
			votes = append(votes, vote)
		}
		for _, signer := range beaconSigners {
			vote := unittest.VoteForBlockFixture(processor.Block(), VoteWithDoubleSig())
			vote.SignerID = signer.ID()
			expectStakingAggregatorCalls(vote, signer.Stake)
			expectedSig := crypto.Signature(vote.SigData[48:])
			reconstructor.On("Verify", vote.SignerID, expectedSig).Return(nil).Maybe()
			reconstructor.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
				collectedShares.Inc()
			}).Return(true, nil).Maybe()
			votes = append(votes, vote)
		}

		// shuffle votes in random order
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(votes), func(i, j int) {
			votes[i], votes[j] = votes[j], votes[i]
		})

		var startProcessing, finishProcessing sync.WaitGroup
		startProcessing.Add(1)
		// process votes concurrently by multiple workers
		for _, vote := range votes {
			finishProcessing.Add(1)
			go func(vote *model.Vote) {
				defer finishProcessing.Done()
				startProcessing.Wait()
				err := processor.Process(vote)
				require.NoError(t, err)
			}(vote)
		}

		// start all goroutines at the same time
		startProcessing.Done()
		finishProcessing.Wait()

		passed := processor.done.Load()
		passed = passed && qcCreated.Load()
		passed = passed && stakingAggregator.AssertExpectations(t)
		passed = passed && reconstructor.AssertExpectations(t)

		if !passed {
			t.Fatalf("Assertions weren't met, staking weight: %v, collected shares: %v", stakingTotalWeight.Load(), collectedShares.Load())
		}
	})
}

func VoteWithStakingSig() func(*model.Vote) {
	return func(vote *model.Vote) {
		vote.SigData = unittest.RandomBytes(48)
	}
}

func VoteWithDoubleSig() func(*model.Vote) {
	return func(vote *model.Vote) {
		vote.SigData = unittest.RandomBytes(96)
	}
}
