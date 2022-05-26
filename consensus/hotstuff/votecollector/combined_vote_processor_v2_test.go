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

	bootstrapDKG "github.com/onflow/flow-go/cmd/bootstrap/dkg"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	hsig "github.com/onflow/flow-go/consensus/hotstuff/signature"
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/seed"
	storagemock "github.com/onflow/flow-go/storage/mock"
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
		log:               unittest.Logger(),
		block:             s.proposal.Block,
		stakingSigAggtor:  s.stakingAggregator,
		rbRector:          s.reconstructor,
		onQCCreated:       s.onQCCreated,
		packer:            s.packer,
		minRequiredWeight: s.minRequiredWeight,
		done:              *atomic.NewBool(false),
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

	// valid length is SigLen or 2*SigLen
	generator := rapid.IntRange(0, 128).Filter(func(value int) bool {
		return value != hsig.SigLen && value != 2*hsig.SigLen
	})
	rapid.Check(s.T(), func(t *rapid.T) {
		// create a signature with invalid length
		vote := unittest.VoteForBlockFixture(s.proposal.Block, func(vote *model.Vote) {
			vote.SigData = unittest.RandomBytes(generator.Draw(t, "sig-size").(int))
		})
		err := s.processor.Process(vote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &model.ErrInvalidFormat)
	})
}

// TestProcess_InvalidSignature tests that CombinedVoteProcessorV2 rejects invalid votes for the following scenarios:
//  1) vote where `SignerID` is not a valid consensus participant;
//     we test correct handling of votes that only a staking signature as well as votes with staking+beacon signatures
//  2) vote from a valid consensus participant
//     case 2a: vote contains only a staking signatures that is invalid
//     case 2b: vote contains _invalid staking sig_, and valid random beacon sig
//     case 2c: vote contains valid staking sig, but _invalid beacon sig_
// In all cases, the CombinedVoteProcessor should interpret these failure cases as invalid votes
// and return an InvalidVoteError.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_InvalidSignature() {
	// Scenario 1) vote where `SignerID` is not a valid consensus participant;
	// sentinel error `InvalidSignerError` from WeightedSignatureAggregator should be wrapped as `InvalidVoteError`
	s.Run("vote with invalid signerID", func() {
		// vote with only a staking signature
		stakingOnlyVote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())
		s.stakingAggregator.On("Verify", stakingOnlyVote.SignerID, mock.Anything).Return(model.NewInvalidSignerErrorf("")).Once()
		err := s.processor.Process(stakingOnlyVote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.True(s.T(), model.IsInvalidSignerError(err))

		// vote with staking+beacon signatures
		doubleSigVote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		s.stakingAggregator.On("Verify", doubleSigVote.SignerID, mock.Anything).Return(model.NewInvalidSignerErrorf("")).Maybe()
		s.reconstructor.On("Verify", doubleSigVote.SignerID, mock.Anything).Return(model.NewInvalidSignerErrorf("")).Maybe()
		err = s.processor.Process(doubleSigVote)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.True(s.T(), model.IsInvalidSignerError(err))

		s.stakingAggregator.AssertNotCalled(s.T(), "TrustedAdd")
		s.reconstructor.AssertNotCalled(s.T(), "TrustedAdd")
	})

	// Scenario 2) vote from a valid consensus participant but included sig(s) are cryptographically invalid;
	// sentinel error `ErrInvalidSignature` from WeightedSignatureAggregator should be wrapped as `InvalidVoteError`
	s.Run("vote from valid participant but with invalid signature(s)", func() {
		// case 2a: vote contains only a staking signatures that is invalid
		voteA := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())
		s.stakingAggregator.On("Verify", voteA.SignerID, mock.Anything).Return(model.ErrInvalidSignature).Once()
		err := s.processor.Process(voteA)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &model.ErrInvalidSignature)

		// case 2b: vote contains _invalid staking sig_, and valid random beacon sig
		voteB := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		s.stakingAggregator.On("Verify", voteB.SignerID, mock.Anything).Return(model.ErrInvalidSignature).Once()
		s.reconstructor.On("Verify", voteB.SignerID, mock.Anything).Return(nil).Maybe()
		err = s.processor.Process(voteB)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &model.ErrInvalidSignature)

		// case 2c: vote contains valid staking sig, but _invalid beacon sig_
		voteC := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		s.stakingAggregator.On("Verify", voteC.SignerID, mock.Anything).Return(nil).Maybe()
		s.reconstructor.On("Verify", voteC.SignerID, mock.Anything).Return(model.ErrInvalidSignature).Once()
		err = s.processor.Process(voteC)
		require.Error(s.T(), err)
		require.True(s.T(), model.IsInvalidVoteError(err))
		require.ErrorAs(s.T(), err, &model.ErrInvalidSignature)

		s.stakingAggregator.AssertNotCalled(s.T(), "TrustedAdd")
		s.reconstructor.AssertNotCalled(s.T(), "TrustedAdd")
	})
}

// TestProcess_TrustedAdd_Exception tests that unexpected exceptions returned by
// WeightedSignatureAggregator.Verify(..) are _not_ interpreted as invalid votes
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_Verify_Exception() {
	exception := errors.New("unexpected-exception")

	s.Run("vote with staking-sig only", func() {
		stakingVote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())
		s.stakingAggregator.On("Verify", stakingVote.SignerID, mock.Anything).Return(exception).Once()
		err := s.processor.Process(stakingVote)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})

	s.Run("vote with staking+beacon sig", func() {
		// verifying staking sig leads to unexpected exception
		doubleSigVoteA := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		s.stakingAggregator.On("Verify", doubleSigVoteA.SignerID, mock.Anything).Return(exception).Once()
		s.reconstructor.On("Verify", doubleSigVoteA.SignerID, mock.Anything).Return(nil).Maybe()
		err := s.processor.Process(doubleSigVoteA)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))

		// verifying beacon sig leads to unexpected exception
		doubleSigVoteB := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithDoubleSig())
		s.stakingAggregator.On("Verify", doubleSigVoteB.SignerID, mock.Anything).Return(nil).Maybe()
		s.reconstructor.On("Verify", doubleSigVoteB.SignerID, mock.Anything).Return(exception).Once()
		err = s.processor.Process(doubleSigVoteB)
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, exception)
		require.False(s.T(), model.IsInvalidVoteError(err))
	})
}

// TestProcess_TrustedAdd_Exception tests that unexpected exceptions returned by
// WeightedSignatureAggregator.Verify(..) are propagated, but _not_ interpreted as invalid votes
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_TrustedAdd_Exception() {
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
		aggregator.On("TrustedAdd", mock.Anything, mock.Anything).Return(s.minRequiredWeight, nil)
		aggregator.On("TotalWeight").Return(s.minRequiredWeight)
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
			log:               unittest.Logger(),
			block:             s.proposal.Block,
			stakingSigAggtor:  stakingAggregator,
			rbRector:          rbReconstructor,
			onQCCreated:       s.onQCCreated,
			packer:            packer,
			minRequiredWeight: s.minRequiredWeight,
			done:              *atomic.NewBool(false),
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

// TestProcess_EnoughWeightNotEnoughShares tests a scenario where we first don't have enough weight,
// then we iteratively increase it to the point where we have enough staking weight. No QC should be created
// in this scenario since there is not enough random beacon shares.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_EnoughWeightNotEnoughShares() {
	for i := uint64(0); i < s.minRequiredWeight; i += s.sigWeight {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, VoteWithStakingSig())
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.False(s.T(), s.processor.done.Load())
	s.reconstructor.AssertCalled(s.T(), "EnoughShares")
	s.onQCCreatedState.AssertNotCalled(s.T(), "onQCCreated")
}

// TestProcess_EnoughSharesNotEnoughWeight tests a scenario where we are collecting votes with staking and beacon sigs
// to the point where we have enough shares to reconstruct RB signature. No QC should be created
// in this scenario since there is not enough staking weight.
func (s *CombinedVoteProcessorV2TestSuite) TestProcess_EnoughSharesNotEnoughWeight() {
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
		aggregator.On("TrustedAdd", mock.Anything, mock.Anything).Return(s.minRequiredWeight, nil)
		aggregator.On("TotalWeight").Return(s.minRequiredWeight)
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
		minRequiredWeight := honestParticipants * sigWeight

		// proposing block
		block := helper.MakeBlock()

		t.Logf("running conf\n\t"+
			"staking signers: %v, beacon signers: %v\n\t"+
			"required weight: %v", stakingSignersCount, beaconSignersCount, minRequiredWeight)

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
			log:               unittest.Logger(),
			block:             block,
			stakingSigAggtor:  stakingAggregator,
			rbRector:          reconstructor,
			onQCCreated:       onQCCreated,
			packer:            packer,
			minRequiredWeight: minRequiredWeight,
			done:              *atomic.NewBool(false),
		}

		votes := make([]*model.Vote, 0, stakingSignersCount+beaconSignersCount)

		expectStakingAggregatorCalls := func(vote *model.Vote) {
			expectedSig := crypto.Signature(vote.SigData[:hsig.SigLen])
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
			expectedSig := crypto.Signature(vote.SigData[hsig.SigLen:])
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

		minRequiredWeight := uint64(0)
		// draw weight for each signer randomly
		stakingSigners := unittest.IdentityListFixture(int(stakingSignersCount), func(identity *flow.Identity) {
			identity.Weight = stakingWeightRange.Draw(t, identity.String()).(uint64)
			minRequiredWeight += identity.Weight
		})
		beaconSigners := unittest.IdentityListFixture(int(beaconSignersCount), func(identity *flow.Identity) {
			identity.Weight = beaconWeightRange.Draw(t, identity.String()).(uint64)
			minRequiredWeight += identity.Weight
		})

		// proposing block
		block := helper.MakeBlock()

		t.Logf("running conf\n\t"+
			"staking signers: %v, beacon signers: %v\n\t"+
			"required weight: %v", stakingSignersCount, beaconSignersCount, minRequiredWeight)

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
			log:               unittest.Logger(),
			block:             block,
			stakingSigAggtor:  stakingAggregator,
			rbRector:          reconstructor,
			onQCCreated:       onQCCreated,
			packer:            packer,
			minRequiredWeight: minRequiredWeight,
			done:              *atomic.NewBool(false),
		}

		votes := make([]*model.Vote, 0, stakingSignersCount+beaconSignersCount)

		expectStakingAggregatorCalls := func(vote *model.Vote, weight uint64) {
			expectedSig := crypto.Signature(vote.SigData[:hsig.SigLen])
			stakingAggregator.On("Verify", vote.SignerID, expectedSig).Return(nil).Maybe()
			stakingAggregator.On("TrustedAdd", vote.SignerID, expectedSig).Run(func(args mock.Arguments) {
				stakingTotalWeight.Add(weight)
			}).Return(uint64(0), nil).Maybe()
		}

		// prepare votes
		for _, signer := range stakingSigners {
			vote := unittest.VoteForBlockFixture(processor.Block(), VoteWithStakingSig())
			vote.SignerID = signer.ID()
			expectStakingAggregatorCalls(vote, signer.Weight)
			votes = append(votes, vote)
		}
		for _, signer := range beaconSigners {
			vote := unittest.VoteForBlockFixture(processor.Block(), VoteWithDoubleSig())
			vote.SignerID = signer.ID()
			expectStakingAggregatorCalls(vote, signer.Weight)
			expectedSig := crypto.Signature(vote.SigData[hsig.SigLen:])
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

// TestCombinedVoteProcessorV2_BuildVerifyQC tests a complete path from creating votes to collecting votes and then
// building & verifying QC.
// We start with leader proposing a block, then new leader collects votes and builds a QC.
// Need to verify that QC that was produced is valid and can be embedded in new proposal.
func TestCombinedVoteProcessorV2_BuildVerifyQC(t *testing.T) {
	epochCounter := uint64(3)
	epochLookup := &modulemock.EpochLookup{}
	view := uint64(20)
	epochLookup.On("EpochForViewWithFallback", view).Return(epochCounter, nil)

	// all committee members run DKG
	dkgData, err := bootstrapDKG.RunFastKG(11, unittest.RandomBytes(32))
	require.NoError(t, err)

	// signers hold objects that are created with private key and can sign votes and proposals
	signers := make(map[flow.Identifier]*verification.CombinedSigner)

	// prepare staking signers, each signer has it's own private/public key pair
	// stakingSigners sign only with staking key, meaning they have failed DKG
	stakingSigners := unittest.IdentityListFixture(3)
	beaconSigners := unittest.IdentityListFixture(8)
	allIdentities := append(stakingSigners, beaconSigners...)
	require.Equal(t, len(dkgData.PubKeyShares), len(allIdentities))
	dkgParticipants := make(map[flow.Identifier]flow.DKGParticipant)
	// fill dkg participants data
	for index, identity := range allIdentities {
		dkgParticipants[identity.NodeID] = flow.DKGParticipant{
			Index:    uint(index),
			KeyShare: dkgData.PubKeyShares[index],
		}
	}

	for _, identity := range stakingSigners {
		stakingPriv := unittest.StakingPrivKeyFixture()
		identity.StakingPubKey = stakingPriv.PublicKey()

		keys := &storagemock.SafeBeaconKeys{}
		// there is no DKG key for this epoch
		keys.On("RetrieveMyBeaconPrivateKey", epochCounter).Return(nil, false, nil)

		beaconSignerStore := hsig.NewEpochAwareRandomBeaconKeyStore(epochLookup, keys)

		me, err := local.New(identity, stakingPriv)
		require.NoError(t, err)

		signers[identity.NodeID] = verification.NewCombinedSigner(me, beaconSignerStore)
	}

	for _, identity := range beaconSigners {
		stakingPriv := unittest.StakingPrivKeyFixture()
		identity.StakingPubKey = stakingPriv.PublicKey()

		participantData := dkgParticipants[identity.NodeID]

		dkgKey := encodable.RandomBeaconPrivKey{
			PrivateKey: dkgData.PrivKeyShares[participantData.Index],
		}

		keys := &storagemock.SafeBeaconKeys{}
		// there is DKG key for this epoch
		keys.On("RetrieveMyBeaconPrivateKey", epochCounter).Return(dkgKey, true, nil)

		beaconSignerStore := hsig.NewEpochAwareRandomBeaconKeyStore(epochLookup, keys)

		me, err := local.New(identity, stakingPriv)
		require.NoError(t, err)

		signers[identity.NodeID] = verification.NewCombinedSigner(me, beaconSignerStore)
	}

	leader := stakingSigners[0]

	block := helper.MakeBlock(helper.WithBlockView(view),
		helper.WithBlockProposer(leader.NodeID))

	inmemDKG, err := inmem.DKGFromEncodable(inmem.EncodableDKG{
		GroupKey: encodable.RandomBeaconPubKey{
			PublicKey: dkgData.PubGroupKey,
		},
		Participants: dkgParticipants,
	})
	require.NoError(t, err)

	committee := &mockhotstuff.Committee{}
	committee.On("Identities", block.BlockID, mock.Anything).Return(allIdentities, nil)
	committee.On("DKG", block.BlockID).Return(inmemDKG, nil)

	votes := make([]*model.Vote, 0, len(allIdentities))

	// first staking signer will be leader collecting votes for proposal
	// prepare votes for every member of committee except leader
	for _, signer := range allIdentities[1:] {
		vote, err := signers[signer.NodeID].CreateVote(block)
		require.NoError(t, err)
		votes = append(votes, vote)
	}

	// create and sign proposal
	proposal, err := signers[leader.NodeID].CreateProposal(block)
	require.NoError(t, err)

	qcCreated := false
	onQCCreated := func(qc *flow.QuorumCertificate) {
		packer := hsig.NewConsensusSigDataPacker(committee)

		// create verifier that will do crypto checks of created QC
		verifier := verification.NewCombinedVerifier(committee, packer)
		forks := &mockhotstuff.Forks{}
		// create validator which will do compliance and crypto checked of created QC
		validator := hotstuffvalidator.New(committee, forks, verifier)
		// check if QC is valid against parent
		err := validator.ValidateQC(qc, block)
		require.NoError(t, err)

		qcCreated = true
	}

	voteProcessorFactory := NewCombinedVoteProcessorFactory(committee, onQCCreated)
	voteProcessor, err := voteProcessorFactory.Create(unittest.Logger(), proposal)
	require.NoError(t, err)

	// process votes by new leader, this will result in producing new QC
	for _, vote := range votes {
		err := voteProcessor.Process(vote)
		require.NoError(t, err)
	}

	require.True(t, qcCreated)
}

func VoteWithStakingSig() func(*model.Vote) {
	return func(vote *model.Vote) {
		vote.SigData = unittest.RandomBytes(hsig.SigLen)
	}
}

func VoteWithDoubleSig() func(*model.Vote) {
	return func(vote *model.Vote) {
		vote.SigData = unittest.RandomBytes(96)
	}
}

// TestReadRandomSourceFromPackedQCV2 tests that a constructed QC can be unpacked and from which the random source can be
// retrieved
func TestReadRandomSourceFromPackedQCV2(t *testing.T) {

	// for V2 there is no aggregated random beacon sig or random beacon signers
	allSigners := unittest.IdentityListFixture(5)
	// staking signers are only subset of all signers
	stakingSigners := allSigners.NodeIDs()[:3]

	aggregatedStakingSig := unittest.SignatureFixture()
	reconstructedBeaconSig := unittest.SignatureFixture()

	blockSigData := buildBlockSignatureDataForV2(stakingSigners, aggregatedStakingSig, reconstructedBeaconSig)

	// making a mock block
	header := unittest.BlockHeaderFixture()
	block := model.BlockFromFlow(&header, header.View-1)

	// create a packer
	committee := &mockhotstuff.Committee{}
	committee.On("Identities", block.BlockID, mock.Anything).Return(allSigners, nil)
	packer := signature.NewConsensusSigDataPacker(committee)

	qc, err := buildQCWithPackerAndSigData(packer, block, blockSigData)
	require.NoError(t, err)

	randomSource, err := seed.FromParentQCSignature(qc.SigData)
	require.NoError(t, err)

	randomSourceAgain, err := seed.FromParentQCSignature(qc.SigData)
	require.NoError(t, err)

	// verify the random source is deterministic
	require.Equal(t, randomSource, randomSourceAgain)
}
