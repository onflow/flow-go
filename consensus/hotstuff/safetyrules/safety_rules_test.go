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

	// producing vote for same view yields in error since we have voted already for this view
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
// knows about QC which is not part of TC and qc.View > tc.NewestQC.View. We want to allow this, leader
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

// TestProduceVote_UpdateLockedOneChainView tests that LockedOneChainView is updated when sees a higher QC
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

// TestProduceVote_InvalidCurrentView tests that no vote is created if proposal is for invalid view
func (s *SafetyRulesTestSuite) TestProduceVote_InvalidCurrentView() {
	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View+1)
	require.Nil(s.T(), vote)
	require.Error(s.T(), err)
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
func (s *SafetyRulesTestSuite) TestProduceVote_InvalidProposerIdentity() {
	*s.committee = mocks.DynamicCommittee{}
	exception := errors.New("invalid-signer-identity")
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.proposal.Block.ProposerID).Return(nil, exception).Once()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_NodeEjected tests that no vote is created if voter is ejected
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
func (s *SafetyRulesTestSuite) TestProduceVote_InvalidVoterIdentity() {
	*s.committee = mocks.DynamicCommittee{}
	s.committee.On("Self").Return(s.ourIdentity.NodeID)
	exception := errors.New("invalid-signer-identity")
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.proposal.Block.ProposerID).Return(s.proposerIdentity, nil).Maybe()
	s.committee.On("IdentityByBlock", s.proposal.Block.BlockID, s.ourIdentity.NodeID).Return(nil, exception).Once()

	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_CreateVoteException tests that no vote is created if vote creation raised an exception
func (s *SafetyRulesTestSuite) TestProduceVote_CreateVoteException() {
	exception := errors.New("create-vote-exception")
	s.signer.On("CreateVote", s.proposal.Block).Return(nil, exception).Once()
	vote, err := s.safety.ProduceVote(s.proposal, s.proposal.Block.View)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
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

// TestProduceVote_VotingOnUnsafeProposal tests different scenarios where we try to vote on unsafe blocks
func (s *SafetyRulesTestSuite) TestProduceVote_VotingOnUnsafeProposal() {
	s.Run("invalid-block-view", func() {
		// create block with block.View == block.QC.View
		proposal := helper.MakeProposal(
			helper.WithBlock(
				helper.MakeBlock(
					helper.WithParentBlock(s.bootstrapBlock),
					helper.WithBlockView(s.bootstrapBlock.View))))
		vote, err := s.safety.ProduceVote(proposal, proposal.Block.View)
		require.Error(s.T(), err)
		require.Nil(s.T(), vote)
	})
	s.Run("view-already-acknowledged", func() {
		// create block with view <= HighestAcknowledgedView
		proposal := helper.MakeProposal(
			helper.WithBlock(
				helper.MakeBlock(
					func(block *model.Block) {
						block.QC = helper.MakeQC(helper.WithQCView(s.safetyData.HighestAcknowledgedView - 1))
					},
					helper.WithBlockView(s.safetyData.HighestAcknowledgedView))))
		vote, err := s.safety.ProduceVote(proposal, proposal.Block.View)
		require.True(s.T(), model.IsNoVoteError(err))
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
		require.Nil(s.T(), vote)
	})

	s.signer.AssertNotCalled(s.T(), "CreateVote")
	s.persister.AssertNotCalled(s.T(), "PutSafetyData")
}

// TestProduceVote_AfterTimeout tests a scenario where we first vote for view and then produce a timeout for
// same view, this case is possible and should result in no error.
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

	// producing timeout with same arguments should return cached version
	otherTimeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.NoError(s.T(), err)
	require.Equal(s.T(), timeout, otherTimeout)

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

	// creating timeout for previous view(that was already cached) should result in error
	timeout, err = s.safety.ProduceTimeout(view, newestQC, nil)
	require.Error(s.T(), err)
	require.Nil(s.T(), timeout)
}

// TestProduceTimeout_NotSafeToTimeout tests that we don't produce a timeout when it's not safe
func (s *SafetyRulesTestSuite) TestProduceTimeout_NotSafeToTimeout() {

	s.Run("highest-qc-below-locked-round", func() {
		view := s.proposal.Block.View
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView - 1))

		timeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("cur-view-below-highest-acknowledged-view", func() {
		view := s.safetyData.HighestAcknowledgedView
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))

		timeout, err := s.safety.ProduceTimeout(view-2, newestQC, nil)
		require.Error(s.T(), err)
		require.Nil(s.T(), timeout)
	})
	s.Run("cur-view-below-highest-QC", func() {
		newestQC := helper.MakeQC(helper.WithQCView(s.safetyData.LockedOneChainView))
		view := newestQC.View

		timeout, err := s.safety.ProduceTimeout(view, newestQC, nil)
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

	exception := errors.New("create-vote-exception")
	s.signer.On("CreateTimeout", view, newestQC, (*flow.TimeoutCertificate)(nil)).Return(nil, exception).Once()
	vote, err := s.safety.ProduceTimeout(view, newestQC, nil)
	require.Nil(s.T(), vote)
	require.ErrorIs(s.T(), err, exception)
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

// TestProduceTimeout_AfterVote tests a case where we first produce a timeout and then try to vote
// for same view, this should result in error since producing a timeout means that we have given up on this view
// and are in process of moving forward, no vote should be created.
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

func makeVote(block *model.Block) *model.Vote {
	return &model.Vote{
		BlockID: block.BlockID,
		View:    block.View,
		SigData: nil, // signature doesn't matter in this test case
	}
}
