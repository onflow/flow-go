package votecollector

import (
	"errors"
	"github.com/onflow/flow-go/consensus/hotstuff"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
)

func TestStakingVoteProcessor(t *testing.T) {
	suite.Run(t, new(StakingVoteProcessorTestSuite))
}

// StakingVoteProcessorTestSuite is a test suite that holds mocked state for isolated testing of StakingVoteProcessor.
type StakingVoteProcessorTestSuite struct {
	VoteProcessorTestSuiteBase

	processor *StakingVoteProcessor
}

func (s *StakingVoteProcessorTestSuite) SetupTest() {
	s.VoteProcessorTestSuiteBase.SetupTest()

	s.processor = newStakingVoteProcessor(
		unittest.Logger(),
		s.proposal.Block,
		s.stakingAggregator,
		s.onQCCreated,
		s.minRequiredStake,
	)
}

// TestInitialState tests that Block() and Status() return correct values after calling constructor
func (s *StakingVoteProcessorTestSuite) TestInitialState() {
	require.Equal(s.T(), s.processor.Block(), s.processor.Block())
	require.Equal(s.T(), hotstuff.VoteCollectorStatusVerifying, s.processor.Status())
}

// TestProcess_VoteNotForProposal tests that vote should pass to validation only if it has correct
// view and block ID matching proposal that is locked in StakingVoteProcessor
func (s *StakingVoteProcessorTestSuite) TestProcess_VoteNotForProposal() {
	err := s.processor.Process(unittest.VoteFixture(unittest.WithVoteView(s.proposal.Block.View)))
	require.Error(s.T(), err)
	err = s.processor.Process(unittest.VoteFixture(unittest.WithVoteBlockID(s.proposal.Block.BlockID)))
	require.Error(s.T(), err)
	s.stakingAggregator.AssertNotCalled(s.T(), "Verify")
}

// TestProcess_InvalidSignature tests that StakingVoteProcessor doesn't collect signatures for votes with invalid signature.
// Checks are made for cases where both staking and threshold signatures were submitted.
func (s *StakingVoteProcessorTestSuite) TestProcess_InvalidSignature() {
	invalidVoteException := errors.New("invalid-vote-exception")
	stakingVote := unittest.VoteForBlockFixture(s.proposal.Block)

	s.stakingAggregator.On("Verify", stakingVote.SignerID, mock.Anything).Return(msig.ErrInvalidFormat).Once()

	// expect sentinel error in case Verify returns ErrInvalidFormat
	err := s.processor.Process(stakingVote)
	require.Error(s.T(), err)
	require.True(s.T(), model.IsInvalidVoteError(err))
	require.ErrorIs(s.T(), err.(model.InvalidVoteError).Err, msig.ErrInvalidFormat)

	s.stakingAggregator.On("Verify", stakingVote.SignerID, mock.Anything).Return(invalidVoteException)

	// except exception
	err = s.processor.Process(stakingVote)
	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, invalidVoteException)

	s.stakingAggregator.AssertNotCalled(s.T(), "TrustedAdd")
}

// TestProcess_TrustedAddError tests a case where we were able to successfully verify signature but failed to collect it.
func (s *StakingVoteProcessorTestSuite) TestProcess_TrustedAddError() {
	trustedAddException := errors.New("trusted-add-exception")
	stakingVote := unittest.VoteForBlockFixture(s.proposal.Block)
	*s.stakingAggregator = mockhotstuff.WeightedSignatureAggregator{}
	s.stakingAggregator.On("Verify", stakingVote.SignerID, mock.Anything).Return(nil).Once()
	s.stakingAggregator.On("TrustedAdd", stakingVote.SignerID, mock.Anything).Return(uint64(0), trustedAddException).Once()
	err := s.processor.Process(stakingVote)
	require.ErrorIs(s.T(), err, trustedAddException)
}

// TestProcess_BuildQCError tests all error paths during process of building QC.
// Building QC is a one time operation, we need to make sure that failing in one of the steps leads to exception.
// Since it's a one time operation we need a complicated test to test all conditions.
func (s *StakingVoteProcessorTestSuite) TestProcess_BuildQCError() {

	// In this test we will mock all dependencies for happy path, and replace some branches with unhappy path
	// to simulate errors along the branches.

	vote := unittest.VoteForBlockFixture(s.proposal.Block)

	// in this test case we aren't able to aggregate staking signature
	exception := errors.New("staking-aggregate-exception")
	stakingSigAggregator := &mockhotstuff.WeightedSignatureAggregator{}
	stakingSigAggregator.On("Verify", mock.Anything, mock.Anything).Return(nil)
	stakingSigAggregator.On("TrustedAdd", mock.Anything, mock.Anything).Return(s.minRequiredStake, nil)
	stakingSigAggregator.On("TotalWeight").Return(s.minRequiredStake)
	stakingSigAggregator.On("Aggregate").Return(nil, nil, exception)

	processor := newStakingVoteProcessor(unittest.Logger(), s.proposal.Block, stakingSigAggregator, s.onQCCreated, s.minRequiredStake)
	err := processor.Process(vote)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcess_NotEnoughStakingWeight tests a scenario where we first don't have enough stake,
// then we iteratively increase it to the point where we have enough staking weight. No QC should be created.
func (s *StakingVoteProcessorTestSuite) TestProcess_NotEnoughStakingWeight() {
	for i := uint64(0); ; i += s.sigWeight {
		if s.minRequiredStake >= i {
			break
		}

		vote := unittest.VoteForBlockFixture(s.proposal.Block)
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.False(s.T(), s.processor.done.Load())
	s.onQCCreatedState.AssertExpectations(s.T())
}

// TestProcess_CreatingQC tests a scenario when we have collected enough staking weight and random beacon shares
// and proceed to build QC. Created QC has to have all signatures and identities aggregated by
// aggregators and packed with consensus packer.
func (s *StakingVoteProcessorTestSuite) TestProcess_CreatingQC() {

	// prepare for aggregation, as soon as we will collect enough shares we will try to create QC
	stakingSigners := unittest.IdentifierListFixture(10)
	expectedSig := unittest.SignatureFixture()
	s.stakingAggregator.On("Aggregate").Return(stakingSigners, []byte(expectedSig), nil)

	s.onQCCreatedState.On("onQCCreated", mock.Anything).Run(func(args mock.Arguments) {
		qc := args.Get(0).(*flow.QuorumCertificate)
		// ensure that QC contains correct field
		expectedQC := &flow.QuorumCertificate{
			View:      s.proposal.Block.View,
			BlockID:   s.proposal.Block.BlockID,
			SignerIDs: stakingSigners,
			SigData:   expectedSig,
		}
		require.Equal(s.T(), expectedQC, qc)
	}).Return(nil).Once()

	// generate staking signatures till we reach enough stake
	for i := uint64(0); i < s.minRequiredStake; i += s.sigWeight {
		vote := unittest.VoteForBlockFixture(s.proposal.Block)
		s.stakingAggregator.On("Verify", vote.SignerID, mock.Anything).Return(nil)
		err := s.processor.Process(vote)
		require.NoError(s.T(), err)
	}

	require.True(s.T(), s.processor.done.Load())
	s.onQCCreatedState.AssertExpectations(s.T())

	// processing extra votes shouldn't result in creating new QCs
	vote := unittest.VoteForBlockFixture(s.proposal.Block)
	err := s.processor.Process(vote)
	require.NoError(s.T(), err)

	s.onQCCreatedState.AssertExpectations(s.T())
}

// TestProcess_ConcurrentCreatingQC tests a scenario where multiple goroutines process vote at same time,
// we expect only one QC created in this scenario.
func (s *StakingVoteProcessorTestSuite) TestProcess_ConcurrentCreatingQC() {
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

	// at this point sending any vote should result in creating QC.
	s.onQCCreatedState.On("onQCCreated", mock.Anything).Return(nil).Once()

	var startupWg, shutdownWg sync.WaitGroup

	vote := unittest.VoteForBlockFixture(s.proposal.Block)
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
