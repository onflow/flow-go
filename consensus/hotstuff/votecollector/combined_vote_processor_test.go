package votecollector

import (
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
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCombinedVoteProcessor(t *testing.T) {
	suite.Run(t, new(CombinedVoteProcessorTestSuite))
}

// 1. if the proposal is invalid, it should return InvalidBlockError
// 2. if the proposal is valid, then a vote processor should be created. The status of created processor is Verifying

// 10. if a vote is invalid, it should return InvalidVoteError
// 11. concurrency: when sending concurrently with votes that have just enough stakes and more beacon shares, a QC should be built
// 12. concurrency: when sending concurrently with votes that have more stakes and just enough beacon shares, a QC should be built

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
	s.proposal = helper.MakeProposal(s.T())
	s.sigWeight = 100
	s.minRequiredStake = 1000 // we require at least 10 sigs to collect min weight
	s.minRequiredShares = 10  // we require 10 RB shares to reconstruct signature
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

func (s *CombinedVoteProcessorTestSuite) onQCCreated(qc *flow.QuorumCertificate) {
	s.onQCCreatedState.Called(qc)
}

// TestInitialState tests that Block() and Status() return correct values after calling constructor
func (s *CombinedVoteProcessorTestSuite) TestInitialState() {
	require.Equal(s.T(), s.processor.Block(), s.processor.Block())
	require.Equal(s.T(), hotstuff.VoteCollectorStatusVerifying, s.processor.Status())
}

// TestProcess_NotEnoughStakingWeight tests a scenario where we first don't have enough stake,
// then we iteratively increase it to the point where we have enough staking weight. No QC should be created
// in this scenario since there is not enough random beacon shares.
func (s *CombinedVoteProcessorTestSuite) TestProcess_NotEnoughStakingWeight() {
	for i := uint64(0); i < s.minRequiredStake; i += s.sigWeight {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.False(s.T(), s.processor.done.Load())
	s.reconstructor.AssertCalled(s.T(), "HasSufficientShares")
	s.onQCCreatedState.AssertExpectations(s.T())
}

// TestProcess_CreatingQC tests a scenario when we have collected enough staking weight and random beacon shares
// and proceed to build QC. Created QC has to have all signatures and identities aggregated by
// aggregators and packed with consensus packer.
func (s *CombinedVoteProcessorTestSuite) TestProcess_CreatingQC() {
	for i := uint64(0); i < s.minRequiredStake; i += s.sigWeight {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithStakingSig())
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	stakingSigners := unittest.IdentifierListFixture(10)
	expectedSigs := unittest.SignaturesFixture(3)
	s.stakingAggregator.On("Aggregate").Return(stakingSigners, expectedSigs[0], nil)
	s.rbSigAggregator.On("Aggregate").Return(stakingSigners, expectedSigs[1], nil)
	s.reconstructor.On("Reconstruct").Return(expectedSigs[2], nil)
	expectedSigData := unittest.RandomBytes(128)

	s.packer.On("Pack", s.proposal.Block.BlockID, mock.Anything).Return(stakingSigners, expectedSigData, nil)
	s.onQCCreatedState.On("onQCCreated", mock.Anything).Run(func(args mock.Arguments) {
		qc := args.Get(0).(*flow.QuorumCertificate)
		// ensure that QC contains correct field
		expectedQC := &flow.QuorumCertificate{
			View:      s.proposal.Block.View,
			BlockID:   s.proposal.Block.BlockID,
			SignerIDs: qc.SignerIDs,
			SigData:   qc.SigData,
		}
		require.Equal(s.T(), expectedQC, qc)
	}).Return(nil)

	for i := uint64(0); i < s.minRequiredShares; i++ {
		vote := unittest.VoteForBlockFixture(s.proposal.Block, unittest.VoteWithThresholdSig())
		s.rbSigAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.True(s.T(), s.processor.done.Load())
	s.onQCCreatedState.AssertExpectations(s.T())
}
