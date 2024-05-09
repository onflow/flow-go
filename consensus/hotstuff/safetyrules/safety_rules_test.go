package safetyrules

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSafetyRules(t *testing.T) {
	suite.Run(t, new(SafetyRulesTestSuite))
}

// SafetyRulesTestSuite is a test suite for testing SafetyRules related functionality.
// SafetyRulesTestSuite setups mocks for injected modules and creates hotstuff.SafetyData
// based on next configuration:
// R <- B[QC_R] <- P[QC_B]
// B.View = S.View + 1
// B - bootstrapped block, we are creating SafetyRules at block B
// Based on this HighestAcknowledgedView = B.View and
type SafetyRulesTestSuite struct {
	suite.Suite

	bootstrapBlock   *model.Block
	proposal         *model.Proposal
	proposerIdentity *flow.Identity
	ourIdentity      *flow.Identity
	signer           *mocks.Signer
	persister        *mocks.Persister
	committee        *mocks.DynamicCommittee
	safetyData       *hotstuff.SafetyData
	safety           *SafetyRules
}

func (s *SafetyRulesTestSuite) SetupTest() {
	s.ourIdentity = unittest.IdentityFixture()
	s.signer = &mocks.Signer{}
	s.persister = &mocks.Persister{}
	s.committee = &mocks.DynamicCommittee{}
	s.proposerIdentity = unittest.IdentityFixture()

	// bootstrap at random bootstrapBlock
	s.bootstrapBlock = helper.MakeBlock(helper.WithBlockView(100))
	s.proposal = helper.MakeProposal(
		helper.WithBlock(
			helper.MakeBlock(
				helper.WithParentBlock(s.bootstrapBlock),
				helper.WithBlockView(s.bootstrapBlock.View+1),
				helper.WithBlockProposer(s.proposerIdentity.NodeID)),
		))

	s.committee.On("Self").Return(s.ourIdentity.NodeID).Maybe()
	s.committee.On("IdentityByBlock", mock.Anything, s.ourIdentity.NodeID).Return(s.ourIdentity, nil).Maybe()
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.proposal.Block.ProposerID).Return(s.proposerIdentity, nil).Maybe()
	s.committee.On("IdentityByEpoch", mock.Anything, s.ourIdentity.NodeID).Return(&s.ourIdentity.IdentitySkeleton, nil).Maybe()

	s.safetyData = &hotstuff.SafetyData{
		LockedOneChainView:      s.bootstrapBlock.View,
		HighestAcknowledgedView: s.bootstrapBlock.View,
	}

	s.persister.On("GetSafetyData").Return(s.safetyData, nil).Once()
	var err error
	s.safety, err = New(s.signer, s.persister, s.committee)
	require.NoError(s.T(), err)
}

// TestProduceVote_ShouldVote test basic happy path scenario where we vote for first block after bootstrap
// and next view ended with TC
func (s *SafetyRulesTestSuite) TestProduceVote_ShouldVote() {
	expectedSafetyData := &hotstuff.SafetyData{
		LockedOneChainView:      s.proposal.Block.QC.View,
		HighestAcknowledgedView: s.proposal.Block.View,
	}

	expectedVote := makeVote(s.proposal.Block)
	s.signer.On("CreateVote", s.proposal.Block).Return(expectedVote, nil).Once()
	s.persister.On("PutSafetyData", expectedSafetyData).Return(nil).Once()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), vote)
	require.Equal(s.T(), expectedVote, vote)

	s.persister.AssertCalled(s.T(), "PutSafetyData", expectedSafetyData)

	// producing vote for same view yields an error since we have voted already for this view
	otherVote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.True(s.T(), model.IsNoVoteError(err))
	require.Nil(s.T(), otherVote)

	lastViewTC := helper.MakeTC(
		helper.WithTCView(s.proposal.Block.View+1),
		helper.WithTCNewestQC(s.proposal.Block.QC))

	// voting on proposal where last view ended with TC
	proposalWithTC := helper.MakeProposal(
		helper.WithBlock(
			helper.MakeBlock(
				helper.WithParentBlock(s.bootstrapBlock),
				helper.WithBlockView(s.proposal.Block.View+2),
				helper.WithBlockProposer(s.proposerIdentity.NodeID))),
		helper.WithLastViewTC(lastViewTC))

	expectedSafetyData = &hotstuff.SafetyData{
		LockedOneChainView:      s.proposal.Block.QC.View,
		HighestAcknowledgedView: proposalWithTC.Block.View,
	}

	expectedVote = makeVote(proposalWithTC.Block)
	s.signer.On("CreateVote", proposalWithTC.Block).Return(expectedVote, nil).Once()
	s.persister.On("PutSafetyData", expectedSafetyData).Return(nil).Once()
	s.committee.On("IdentityByBlock", proposalWithTC.Block.BlockID, proposalWithTC.Block.ProposerID).Return(s.proposerIdentity, nil).Maybe()

	vote, err = s.safety.ProduceVote(proposalWithTC, proposalWithTC.Block.View)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), vote)
	require.Equal(s.T(), expectedVote, vote)
	s.signer.AssertExpectations(s.T())
	s.persister.AssertCalled(s.T(), "PutSafetyData", expectedSafetyData)
}

// TestProduceVote_IncludedQCHigherThanTCsQC checks specific scenario where previous round resulted in TC and leader
// knows about QC which is not part of TC and qc.View > tc.NewestQC.View. We want to allow this, in this case leader
// includes his QC into proposal satisfies next condition: Block.QC.View > lastViewTC.NewestQC.View
func (s *SafetyRulesTestSuite) TestProduceVote_IncludedQCHigherThanTCsQC() {
	lastViewTC := helper.MakeTC(
		helper.WithTCView(s.proposal.Block.View+1),
		helper.WithTCNewestQC(s.proposal.Block.QC))

	// voting on proposal where last view ended with TC
	proposalWithTC := helper.MakeProposal(
		helper.WithBlock(
			helper.MakeBlock(
				helper.WithParentBlock(s.proposal.Block),
				helper.WithBlockView(s.proposal.Block.View+2),
				helper.WithBlockProposer(s.proposerIdentity.NodeID))),
		helper.WithLastViewTC(lastViewTC))

	expectedSafetyData := &hotstuff.SafetyData{
		LockedOneChainView:      proposalWithTC.Block.QC.View,
		HighestAcknowledgedView: proposalWithTC.Block.View,
	}

	require.Greater(s.T(), proposalWithTC.Block.QC.View, proposalWithTC.LastViewTC.NewestQC.View,
		"for this test case we specifically require that qc.View > lastViewTC.NewestQC.View")

	expectedVote := makeVote(proposalWithTC.Block)
	s.signer.On("CreateVote", proposalWithTC.Block).Return(expectedVote, nil).Once()
	s.persister.On("PutSafetyData", expectedSafetyData).Return(nil).Once()
	s.committee.On("IdentityByBlock", proposalWithTC.Block.BlockID, proposalWithTC.Block.ProposerID).Return(s.proposerIdentity, nil).Maybe()

	vote, err := s.safety.ProduceVote(proposalWithTC, proposalWithTC.Block.View)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), vote)
	require.Equal(s.T(), expectedVote, vote)
	s.signer.AssertExpectations(s.T())
	s.persister.AssertCalled(s.T(), "PutSafetyData", expectedSafetyData)
}

// TestProduceVote_UpdateLockedOneChainView tests that LockedOneChainView is updated when sees a higher QC.
// Note: `LockedOneChainView` is only updated when the replica votes.
func (s *SafetyRulesTestSuite) TestProduceVote_UpdateLockedOneChainView() {
	s.safety.safetyData.LockedOneChainView = 0

	require.NotEqual(s.T(), s.safety.safetyData.LockedOneChainView, s.proposal.Block.QC.View,
		"in this test LockedOneChainView is lower so it needs to be updated")

	expectedSafetyData := &hotstuff.SafetyData{
		LockedOneChainView:      s.proposal.Block.QC.View,
		HighestAcknowledgedView: s.proposal.Block.View,
	}

	expectedVote := makeVote(s.proposal.Block)
	s.signer.On("CreateVote", s.proposal.Block).Return(expectedVote, nil).Once()
	s.persister.On("PutSafetyData", expectedSafetyData).Return(nil).Once()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), vote)
	require.Equal(s.T(), expectedVote, vote)
	s.signer.AssertExpectations(s.T())
	s.persister.AssertCalled(s.T(), "PutSafetyData", expectedSafetyData)
}

// TestProduceVote_InvalidCurrentView tests that no vote is created if `curView` has invalid values.
// In particular, `SafetyRules` requires that:
//   - the block's view matches `curView`
//   - that values for `curView` are monotonously increasing
//
// Failing any of these conditions is a symptom of an internal bug; hence `SafetyRules` should
// _not_ return a `NoVoteError`.
func (s *SafetyRulesTestSuite) TestProduceVote_InvalidCurrentView() {

	s.Run("block-view-does-not-match", func() {
		vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View+1)
		require.Nil(s.T(), vote)
		require.Error(s.T(), err)
		require.False(s.T(), model.IsNoVoteError(err))
	})
	s.Run("view-not-monotonously-increasing", func() {
		// create block with view < HighestAcknowledgedView
		proposal := helper.MakeProposal(
			helper.WithBlock(
				helper.MakeBlock(
					func(block *model.Block) {
						block.QC = helper.MakeQC(helper.WithQCView(s.safetyData.HighestAcknowledgedView - 2))
					},
					helper.WithBlockView(s.safetyData.HighestAcknowledgedView-1))))
		vote, err := s.safety.ProduceVote(proposal, proposal.Block.View)
		require.Nil(s.T(), vote)
		require.Error(s.T(), err)
		require.False(s.T(), model.IsNoVoteError(err))
	})

	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_NodeEjected tests that no vote is created if block proposer is ejected
func (s *SafetyRulesTestSuite) TestProduceVote_ProposerEjected() {
	*s.committee = mocks.DynamicCommittee{}
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.proposal.Block.ProposerID).Return(nil, model.NewInvalidSignerErrorf("node-ejected")).Once()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.True(s.T(), model.IsNoVoteError(err))
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_InvalidProposerIdentity tests that no vote is created if there was an exception retrieving proposer identity
// We are specifically testing that unexpected errors are handled correctly, i.e.
// that SafetyRules does not erroneously wrap unexpected exceptions into the expected NoVoteError.
func (s *SafetyRulesTestSuite) TestProduceVote_InvalidProposerIdentity() {
	*s.committee = mocks.DynamicCommittee{}
	exception := errors.New("invalid-signer-identity")
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.proposal.Block.ProposerID).Return(nil, exception).Once()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), model.IsNoVoteError(err))
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_NodeNotAuthorizedToVote tests that no vote is created if the voter is not authorized to vote.
// Nodes have zero weight in the grace periods around the epochs where they are authorized to participate.
// We don't want zero-weight nodes to vote in the first place, to avoid unnecessary traffic.
// Note: this also covers ejected nodes. In both cases, the committee will return an `InvalidSignerError`.
func (s *SafetyRulesTestSuite) TestProduceVote_NodeEjected() {
	*s.committee = mocks.DynamicCommittee{}
	s.committee.On("Self").Return(s.ourIdentity.NodeID)
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.ourIdentity.NodeID).Return(nil, model.NewInvalidSignerErrorf("node-ejected")).Once()
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.proposal.Block.ProposerID).Return(s.proposerIdentity, nil).Maybe()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.True(s.T(), model.IsNoVoteError(err))
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_InvalidVoterIdentity tests that no vote is created if there was an exception retrieving voter identity
// We are specifically testing that unexpected errors are handled correctly, i.e.
// that SafetyRules does not erroneously wrap unexpected exceptions into the expected NoVoteError.
func (s *SafetyRulesTestSuite) TestProduceVote_InvalidVoterIdentity() {
	*s.committee = mocks.DynamicCommittee{}
	s.committee.On("Self").Return(s.ourIdentity.NodeID)
	exception := errors.New("invalid-signer-identity")
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.proposal.Block.ProposerID).Return(s.proposerIdentity, nil).Maybe()
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.ourIdentity.NodeID).Return(nil, exception).Once()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), model.IsNoVoteError(err))
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_CreateVoteException tests that no vote is created if vote creation raised an exception
func (s *SafetyRulesTestSuite) TestProduceVote_CreateVoteException() {
	exception := errors.New("create-vote-exception")
	s.signer.On("CreateVote", s.proposal.Block).Return(nil, exception).Once()
	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), model.IsNoVoteError(err))
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_PersistStateException tests that no vote is created if persisting state failed
func (s *SafetyRulesTestSuite) TestProduceVote_PersistStateException() {
	exception := errors.New("persister-exception")
	s.persister.On("PutSafetyData", mock.Anything).Return(exception)

	vote := makeVote(s.proposal.Block)
	s.signer.On("CreateVote", s.proposal.Block).Return(vote, nil).Once()
	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
}

// TestProduceVote_VotingOnInvalidProposals tests different scenarios where we try to vote on unsafe blocks
// SafetyRules contain a variety of checks to confirm that QC and TC have the desired relationship to each other.
// In particular, we test:
//
//	  (i) A TC should be included in a proposal, if and only of the QC is not the prior view.
//	 (ii) When the proposal includes a TC (i.e. the QC not being for the prior view), the TC must be for the prior view.
//	(iii) The QC in the block must have a smaller view than the block.
//	 (iv) If the block contains a TC, the TC cannot contain a newer QC than the block itself.
//
// Conditions (i) - (iv) are validity requirements for the block and all blocks that SafetyRules processes
// are supposed to be pre-validated. Hence, failing any of those conditions means we have an internal bug.
// Consequently, we expect SafetyRules to return exceptions but _not_ `NoVoteError`, because the latter
// indicates that the input block was valid, but we didn't want to vote.
func (s *SafetyRulesTestSuite) TestProduceVote_VotingOnInvalidProposals() {

	// a proposal which includes a QC for the previous round should not contain a TC
	s.Run("proposal-includes-last-view-qc-and-tc", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(
				helper.MakeBlock(
					helper.WithParentBlock(s.bootstrapBlock),
					helper.WithBlockView(s.bootstrapBlock.View+1))),
			helper.WithLastViewTC(helper.MakeTC()))
		s.committee.On("IdentityByBlock", proposal.Block.BlockID, proposal.Block.ProposerID).Return(s.proposerIdentity, nil).Maybe()
		vote, err := s.safety.ProduceVote(proposal, proposal.Block.View)
		require.Error(s.T(), err)
		require.False(s.T(), model.IsNoVoteError(err))
		require.Nil(s.T(), vote)
	})
	s.Run("no-last-view-tc", func() {
		// create block where Block.View != Block.QC.View+1 and LastViewTC = nil
		proposal := helper.MakeProposal(
			helper.WithBlock(
				helper.MakeBlock(
					helper.WithParentBlock(s.bootstrapBlock),
					helper.WithBlockView(s.bootstrapBlock.View+2))))
		vote, err := s.safety.ProduceVote(proposal, proposal.Block.View)
		require.Error(s.T(), err)
		require.False(s.T(), model.IsNoVoteError(err))
		require.Nil(s.T(), vote)
	})
	s.Run("last-view-tc-invalid-view", func() {
		// create block where Block.View != Block.QC.View+1 and
		// Block.View != LastViewTC.View+1
		proposal := helper.MakeProposal(
			helper.WithBlock(
				helper.MakeBlock(
					helper.WithParentBlock(s.bootstrapBlock),
					helper.WithBlockView(s.bootstrapBlock.View+2))),
			helper.WithLastViewTC(
				helper.MakeTC(
					helper.WithTCView(s.bootstrapBlock.View))))
		vote, err := s.safety.ProduceVote(proposal, proposal.Block.View)
		require.Error(s.T(), err)
		require.False(s.T(), model.IsNoVoteError(err))
		require.Nil(s.T(), vote)
	})
	s.Run("proposal-includes-QC-for-higher-view", func() {
		// create block where Block.View != Block.QC.View+1 and
		// Block.View == LastViewTC.View+1 and Block.QC.View >= Block.View
		// in this case block is not safe to extend since proposal includes QC which is newer than the proposal itself.
		proposal := helper.MakeProposal(
			helper.WithBlock(
				helper.MakeBlock(
					helper.WithParentBlock(s.bootstrapBlock),
					helper.WithBlockView(s.bootstrapBlock.View+2),
					func(block *model.Block) {
						block.QC = helper.MakeQC(helper.WithQCView(s.bootstrapBlock.View + 10))
					})),
			helper.WithLastViewTC(
				helper.MakeTC(
					helper.WithTCView(s.bootstrapBlock.View+1))))
		vote, err := s.safety.ProduceVote(proposal, proposal.Block.View)
		require.Error(s.T(), err)
		require.False(s.T(), model.IsNoVoteError(err))
		require.Nil(s.T(), vote)
	})
	s.Run("last-view-tc-invalid-highest-qc", func() {
		// create block where Block.View != Block.QC.View+1 and
		// Block.View == LastViewTC.View+1 and Block.QC.View < LastViewTC.NewestQC.View
		// in this case block is not safe to extend since proposal is built on top of QC, which is lower
		// than QC presented in LastViewTC.
		TONewestQC := helper.MakeQC(helper.WithQCView(s.bootstrapBlock.View + 1))
		proposal := helper.MakeProposal(
			helper.WithBlock(
				helper.MakeBlock(
					helper.WithParentBlock(s.bootstrapBlock),
					helper.WithBlockView(s.bootstrapBlock.View+2))),
			helper.WithLastViewTC(
				helper.MakeTC(
					helper.WithTCView(s.bootstrapBlock.View+1),
					helper.WithTCNewestQC(TONewestQC))))
		vote, err := s.safety.ProduceVote(proposal, proposal.Block.View)
		require.Error(s.T(), err)
		require.False(s.T(), model.IsNoVoteError(err))
		require.Nil(s.T(), vote)
	})

	s.signer.AssertNotCalled(s.T(), "CreateVote")
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_VoteEquivocation tests scenario when we try to vote twice in same view. We require that replica
// follows next rules:
//   - replica votes once per view
//   - replica votes in monotonously increasing views
//
// Voting twice per round on equivocating proposals is considered a byzantine behavior.
// Expect a `model.NoVoteError` sentinel in such scenario.
func (s *SafetyRulesTestSuite) TestProduceVote_VoteEquivocation() {
	expectedVote := makeVote(s.proposal.Block)
	s.signer.On("CreateVote", s.proposal.Block).Return(expectedVote, nil).Once()
	s.persister.On("PutSafetyData", mock.Anything).Return(nil).Once()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), vote)
	require.Equal(s.T(), expectedVote, vote)

	equivocatingProposal := helper.MakeProposal(
		helper.WithBlock(
			helper.MakeBlock(
				helper.WithParentBlock(s.bootstrapBlock),
				helper.WithBlockView(s.bootstrapBlock.View+1),
				helper.WithBlockProposer(s.proposerIdentity.NodeID)),
		))

	// voting at same view(event different proposal) should result in NoVoteError
	vote, err = s.safety.ProduceVote(equivocatingProposal, s.proposal.Block.View)
	require.True(s.T(), model.IsNoVoteError(err))
	require.Nil(s.T(), vote)
}

// TestProduceVote_AfterTimeout tests a scenario where we first timeout for view and then try to produce a vote for
// same view, this should result in error since producing a timeout means that we have given up on this view
// and are in process of moving forward, no vote should be created.
func (s *SafetyRulesTestSuite) TestProduceVote_AfterTimeout() {
	view := s.proposal.Block.View
	newestQC := helper.MakeQC(helper.WithQCView(view - 1))
	expectedTimeout := &model.TimeoutObject{
		View:     view,
		NewestQC: newestQC,
	}
	s.signer.On("CreateTimeout", view, newestQC, (*flow.TimeoutCertificate)(nil)).Return(expectedTimeout, nil).Once()
	s.persister.On("PutSafetyData", mock.Anything).Return(nil).Once()

	// first timeout, then try to vote
	timeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), timeout)

	// voting in same view after producing timeout is not allowed
	vote, err := s.safety.ProduceVote(s.proposal, view)
	require.True(s.T(), model.IsNoVoteError(err))
	require.Nil(s.T(), vote)

	s.signer.AssertExpectations(s.T())
	s.persister.AssertExpectations(s.T())
}

// TestProduceTimeout_ShouldTimeout tests that we can produce timeout in cases where
// last view was successful or not. Also tests last timeout caching.
func (s *SafetyRulesTestSuite) TestProduceTimeout_ShouldTimeout() {
	view := s.proposal.Block.View
	newestQC := helper.MakeQC(helper.WithQCView(view - 1))
	expectedTimeout := &model.TimeoutObject{
		View:     view,
		NewestQC: newestQC,
	}

	expectedSafetyData := &hotstuff.SafetyData{
		LockedOneChainView:      s.safetyData.LockedOneChainView,
		HighestAcknowledgedView: view,
		LastTimeout:             expectedTimeout,
	}
	s.signer.On("CreateTimeout", view, newestQC, (*flow.TimeoutCertificate)(nil)).Return(expectedTimeout, nil).Once()
	s.persister.On("PutSafetyData", expectedSafetyData).Return(nil).Once()
	timeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedTimeout, timeout)

	s.persister.AssertCalled(s.T(), "PutSafetyData", expectedSafetyData)

	// producing timeout with same arguments should return cached version but with incremented timeout tick
	expectedSafetyData.LastTimeout = &model.TimeoutObject{}
	*expectedSafetyData.LastTimeout = *expectedTimeout
	expectedSafetyData.LastTimeout.TimeoutTick++
	s.persister.On("PutSafetyData", expectedSafetyData).Return(nil).Once()

	otherTimeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.NoError(s.T(), err)
	require.Equal(s.T(), timeout.ID(), otherTimeout.ID())
	require.Equal(s.T(), timeout.TimeoutTick+1, otherTimeout.TimeoutTick)

	// to create new TO we need to provide a TC
	lastViewTC := helper.MakeTC(helper.WithTCView(view),
		helper.WithTCNewestQC(newestQC))

	expectedTimeout = &model.TimeoutObject{
		View:       view + 1,
		NewestQC:   newestQC,
		LastViewTC: lastViewTC,
	}
	s.signer.On("CreateTimeout", view+1, newestQC, lastViewTC).Return(expectedTimeout, nil).Once()
	expectedSafetyData = &hotstuff.SafetyData{
		LockedOneChainView:      s.safetyData.LockedOneChainView,
		HighestAcknowledgedView: view + 1,
		LastTimeout:             expectedTimeout,
	}
	s.persister.On("PutSafetyData", expectedSafetyData).Return(nil).Once()

	// creating new timeout should invalidate cache
	otherTimeout, err = s.safety.ProduceTimeout(view+1, newestQC, lastViewTC)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), otherTimeout)
}

// TestProduceTimeout_NotSafeToTimeout tests that we don't produce a timeout when it's not safe
// We expect that the EventHandler to feed only request timeouts for the current view, providing valid set of inputs.
// Hence, the cases tested here would be symptoms of an internal bugs, and therefore should not result in an NoVoteError.
func (s *SafetyRulesTestSuite) TestProduceTimeout_NotSafeToTimeout() {

	s.Run("newest-qc-nil", func() {
		// newestQC cannot be nil
		timeout, err := s.safety.ProduceTimeout(s.safetyData.LockedOneChainView, nil, nil)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	// if a QC for the previous view is provided, a last view TC is unnecessary and must not be provided
	s.Run("includes-last-view-qc-and-tc", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))

		// tc not needed but included
		timeout, err := s.safety.ProduceTimeout(newestQC.View+1, newestQC, helper.MakeTC())
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("last-view-tc-nil", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))

		// tc needed but not included
		timeout, err := s.safety.ProduceTimeout(newestQC.View+2, newestQC, nil)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("last-view-tc-for-wrong-view", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))
		// lastViewTC should be for newestQC.View+1
		lastViewTC := helper.MakeTC(helper.WithTCView(newestQC.View))

		timeout, err := s.safety.ProduceTimeout(newestQC.View+2, newestQC, lastViewTC)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("cur-view-equal-to-highest-QC", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))
		lastViewTC := helper.MakeTC(helper.WithTCView(s.safetyData.LockedOneChainView - 1))

		timeout, err := s.safety.ProduceTimeout(s.safetyData.LockedOneChainView, newestQC, lastViewTC)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("cur-view-below-highest-QC", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))
		lastViewTC := helper.MakeTC(helper.WithTCView(newestQC.View - 2))

		timeout, err := s.safety.ProduceTimeout(newestQC.View-1, newestQC, lastViewTC)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("last-view-tc-is-newer", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))
		// newest QC included in TC cannot be higher than the newest QC known to replica
		lastViewTC := helper.MakeTC(helper.WithTCView(newestQC.View+1),
			helper.WithTCNewestQC(helper.MakeQC(helper.WithQCView(newestQC.View+1))))

		timeout, err := s.safety.ProduceTimeout(newestQC.View+2, newestQC, lastViewTC)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("highest-qc-below-locked-round", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView - 1))

		timeout, err := s.safety.ProduceTimeout(newestQC.View+1, newestQC, nil)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("cur-view-below-highest-acknowledged-view", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))
		// modify highest acknowledged view in a way that it's definitely bigger than the newest QC view
		s.safetyData.HighestAcknowledgedView = newestQC.View + 10

		timeout, err := s.safety.ProduceTimeout(newestQC.View+1, newestQC, nil)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})

	s.signer.AssertNotCalled(s.T(), "CreateTimeout")
	s.signer.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceTimeout_CreateTimeoutException tests that no timeout is created if timeout creation raised an exception
func (s *SafetyRulesTestSuite) TestProduceTimeout_CreateTimeoutException() {
	view := s.proposal.Block.View
	newestQC := helper.MakeQC(helper.WithQCView(view - 1))

	exception := errors.New("create-timeout-exception")
	s.signer.On("CreateTimeout", view, newestQC, (*flow.TimeoutCertificate)(nil)).Return(nil, exception).Once()
	vote, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), model.IsNoVoteError(err))
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceTimeout_PersistStateException tests that no timeout is created if persisting state failed
func (s *SafetyRulesTestSuite) TestProduceTimeout_PersistStateException() {
	exception := errors.New("persister-exception")
	s.persister.On("PutSafetyData", mock.Anything).Return(exception)

	view := s.proposal.Block.View
	newestQC := helper.MakeQC(helper.WithQCView(view - 1))
	expectedTimeout := &model.TimeoutObject{
		View:     view,
		NewestQC: newestQC,
	}

	s.signer.On("CreateTimeout", view, newestQC, (*flow.TimeoutCertificate)(nil)).Return(expectedTimeout, nil).Once()
	timeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.Nil(s.T(), timeout)
	require.ErrorIs(s.T(), err, exception)
}

// TestProduceTimeout_AfterVote tests a case where we first produce a vote and then try to timeout
// for same view. This behavior is expected and should result in valid timeout without any errors.
func (s *SafetyRulesTestSuite) TestProduceTimeout_AfterVote() {
	expectedVote := makeVote(s.proposal.Block)
	s.signer.On("CreateVote", s.proposal.Block).Return(expectedVote, nil).Once()
	s.persister.On("PutSafetyData", mock.Anything).Return(nil).Times(2)

	view := s.proposal.Block.View

	// first produce vote, then try to timeout
	vote, err := s.safety.ProduceVote(s.proposal, view)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), vote)

	newestQC := helper.MakeQC(helper.WithQCView(view - 1))

	expectedTimeout := &model.TimeoutObject{
		View:     view,
		NewestQC: newestQC,
	}

	s.signer.On("CreateTimeout", view, newestQC, (*flow.TimeoutCertificate)(nil)).Return(expectedTimeout, nil).Once()

	// timing out for same view should be possible
	timeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), timeout)

	s.persister.AssertExpectations(s.T())
	s.signer.AssertExpectations(s.T())
}

// TestProduceTimeout_InvalidProposerIdentity tests that no timeout is created if there was an exception retrieving proposer identity
// We are specifically testing that unexpected errors are handled correctly, i.e.
// that SafetyRules does not erroneously wrap unexpected exceptions into the expected model.NoTimeoutError.
func (s *SafetyRulesTestSuite) TestProduceTimeout_InvalidProposerIdentity() {
	view := s.proposal.Block.View
	newestQC := helper.MakeQC(helper.WithQCView(view - 1))
	*s.committee = mocks.DynamicCommittee{}
	exception := errors.New("invalid-signer-identity")
	s.committee.On("IdentityByEpoch", view, s.ourIdentity.NodeID).Return(nil, exception).Once()
	s.committee.On("Self").Return(s.ourIdentity.NodeID)

	timeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.Nil(s.T(), timeout)
	require.ErrorIs(s.T(), err, exception)
	require.False(s.T(), model.IsNoTimeoutError(err))
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceTimeout_NodeEjected tests that no timeout is created if the replica is not authorized to create timeout.
// Nodes have zero weight in the grace periods around the epochs where they are authorized to participate.
// We don't want zero-weight nodes to participate in the first place, to avoid unnecessary traffic.
// Note: this also covers ejected nodes. In both cases, the committee will return an `InvalidSignerError`.
func (s *SafetyRulesTestSuite) TestProduceTimeout_NodeEjected() {
	view := s.proposal.Block.View
	newestQC := helper.MakeQC(helper.WithQCView(view - 1))
	*s.committee = mocks.DynamicCommittee{}
	s.committee.On("Self").Return(s.ourIdentity.NodeID)
	s.committee.On("IdentityByEpoch", view, s.ourIdentity.NodeID).Return(nil, model.NewInvalidSignerErrorf("")).Maybe()

	timeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.Nil(s.T(), timeout)
	require.True(s.T(), model.IsNoTimeoutError(err))
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

func makeVote(block *model.Block) *model.Vote {
	return &model.Vote{
		BlockID: block.BlockID,
		View:    block.View,
		SigData: nil, // signature doesn't matter in this test case
	}
}
