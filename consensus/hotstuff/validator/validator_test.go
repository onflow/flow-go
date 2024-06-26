package validator

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/onflow/flow-go/module/signature"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestValidateProposal(t *testing.T) {
	suite.Run(t, new(ProposalSuite))
}

type ProposalSuite struct {
	suite.Suite
	participants flow.IdentityList
	indices      []byte
	leader       *flow.IdentitySkeleton
	finalized    uint64
	parent       *model.Block
	block        *model.Block
	voters       flow.IdentitySkeletonList
	proposal     *model.Proposal
	vote         *model.Vote
	voter        *flow.IdentitySkeleton
	committee    *mocks.Replicas
	verifier     *mocks.Verifier
	validator    *Validator
}

func (ps *ProposalSuite) SetupTest() {
	// the leader is a random node for now
	ps.finalized = uint64(rand.Uint32() + 1)
	ps.participants = unittest.IdentityListFixture(8, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity])
	ps.leader = &ps.participants[0].IdentitySkeleton

	// the parent is the last finalized block, followed directly by a block from the leader
	ps.parent = helper.MakeBlock(
		helper.WithBlockView(ps.finalized),
	)

	var err error

	ps.indices, err = signature.EncodeSignersToIndices(ps.participants.NodeIDs(), ps.participants.NodeIDs())
	require.NoError(ps.T(), err)

	ps.block = helper.MakeBlock(
		helper.WithBlockView(ps.finalized+1),
		helper.WithBlockProposer(ps.leader.NodeID),
		helper.WithParentBlock(ps.parent),
		helper.WithParentSigners(ps.indices),
	)

	voterIDs, err := signature.DecodeSignerIndicesToIdentifiers(ps.participants.NodeIDs(), ps.block.QC.SignerIndices)
	require.NoError(ps.T(), err)

	ps.voters = ps.participants.Filter(filter.HasNodeID[flow.Identity](voterIDs...)).ToSkeleton()
	ps.proposal = &model.Proposal{Block: ps.block}
	ps.vote = ps.proposal.ProposerVote()
	ps.voter = ps.leader

	// set up the mocked hotstuff Replicas state
	ps.committee = &mocks.Replicas{}
	ps.committee.On("LeaderForView", ps.block.View).Return(ps.leader.NodeID, nil)
	ps.committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(ps.participants.ToSkeleton().TotalWeight()), nil)
	ps.committee.On("IdentitiesByEpoch", mock.Anything).Return(
		func(_ uint64) flow.IdentitySkeletonList {
			return ps.participants.ToSkeleton()
		},
		nil,
	)
	for _, participant := range ps.participants {
		ps.committee.On("IdentityByEpoch", mock.Anything, participant.NodeID).Return(&participant.IdentitySkeleton, nil)
	}

	// set up the mocked verifier
	ps.verifier = &mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(nil).Maybe()
	ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil).Maybe()

	// set up the validator with the mocked dependencies
	ps.validator = New(ps.committee, ps.verifier)
}

func (ps *ProposalSuite) TestProposalOK() {
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.NoError(ps.T(), err, "a valid proposal should be accepted")
}

func (ps *ProposalSuite) TestProposalSignatureError() {

	// change the verifier to error on signature validation with unspecific error
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(nil)
	ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(errors.New("dummy error"))

	// check that validation now fails
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal should be rejected if signature check fails")

	// check that the error is not one that leads to invalid
	assert.False(ps.T(), model.IsInvalidProposalError(err), "if signature check fails, we should not receive an ErrorInvalidBlock")
}

func (ps *ProposalSuite) TestProposalSignatureInvalidFormat() {

	// change the verifier to fail signature validation with InvalidFormatError error
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(nil)
	ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(model.NewInvalidFormatErrorf(""))

	// check that validation now fails
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid signature should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.True(ps.T(), model.IsInvalidProposalError(err), "if signature is invalid, we should generate an invalid error")
}

func (ps *ProposalSuite) TestProposalSignatureInvalid() {

	// change the verifier to fail signature validation
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(nil)
	ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(model.ErrInvalidSignature)

	// check that validation now fails
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid signature should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.True(ps.T(), model.IsInvalidProposalError(err), "if signature is invalid, we should generate an invalid error")
}

func (ps *ProposalSuite) TestProposalWrongLeader() {

	// change the hotstuff.Replicas to return a different leader
	*ps.committee = mocks.Replicas{}
	ps.committee.On("LeaderForView", ps.block.View).Return(ps.participants[1].NodeID, nil)
	for _, participant := range ps.participants.ToSkeleton() {
		ps.committee.On("IdentityByEpoch", mock.Anything, participant.NodeID).Return(participant, nil)
	}

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal from the wrong proposer should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.True(ps.T(), model.IsInvalidProposalError(err), "if the proposal has wrong proposer, we should generate a invalid error")
}

// TestProposalQCInvalid checks that Validator handles the verifier's error returns correctly.
// In case of `model.InvalidFormatError` and model.ErrInvalidSignature`, we expect the Validator
// to recognize those as an invalid QC, i.e. returns an `model.InvalidProposalError`.
// In contrast, unexpected exceptions and `model.InvalidSignerError` should _not_ be
// interpreted as a sign of an invalid QC.
func (ps *ProposalSuite) TestProposalQCInvalid() {
	ps.Run("invalid-signature", func() {
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(
			fmt.Errorf("invalid qc: %w", model.ErrInvalidSignature))
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure case is recognized as an invalid block
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.True(ps.T(), model.IsInvalidProposalError(err), "if the block's QC signature is invalid, an ErrorInvalidBlock error should be raised")
	})

	ps.Run("invalid-format", func() {
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(model.NewInvalidFormatErrorf("invalid qc"))
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure case is recognized as an invalid block
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.True(ps.T(), model.IsInvalidProposalError(err), "if the block's QC has an invalid format, an ErrorInvalidBlock error should be raised")
	})

	// Theoretically, `VerifyQC` could also return a `model.InvalidSignerError`. However,
	// for the time being, we assume that _every_ HotStuff participant is also a member of
	// the random beacon committee. Consequently, `InvalidSignerError` should not occur atm.
	// TODO: if the random beacon committee is a strict subset of the HotStuff committee,
	//       we expect `model.InvalidSignerError` here during normal operations.
	ps.Run("invalid-signer", func() {
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(
			fmt.Errorf("invalid qc: %w", model.NewInvalidSignerErrorf("")))
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure case is recognized as an invalid block
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.Error(ps.T(), err)
		assert.False(ps.T(), model.IsInvalidProposalError(err))
	})

	ps.Run("unknown-exception", func() {
		exception := errors.New("exception")
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(exception)
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure case is recognized as an invalid block
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.ErrorIs(ps.T(), err, exception)
		assert.False(ps.T(), model.IsInvalidProposalError(err))
	})

	ps.Run("verify-qc-err-view-for-unknown-epoch", func() {
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(model.ErrViewForUnknownEpoch)
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure is considered internal exception and NOT an InvalidProposal error
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.Error(ps.T(), err)
		assert.NotErrorIs(ps.T(), err, model.ErrViewForUnknownEpoch)
		assert.False(ps.T(), model.IsInvalidProposalError(err))
	})
}

func (ps *ProposalSuite) TestProposalQCError() {

	// change verifier to fail on QC validation
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(fmt.Errorf("some exception"))
	ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid QC should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.False(ps.T(), model.IsInvalidProposalError(err), "if we can't verify the QC, we should not generate a invalid error")
}

// TestProposalWithLastViewTC tests different scenarios where last view has ended with TC
// this requires including a valid LastViewTC.
func (ps *ProposalSuite) TestProposalWithLastViewTC() {
	// assume all proposals are created by valid leader
	ps.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ps.committee.On("LeaderForView", mock.Anything).Return(ps.leader.NodeID, nil)

	ps.Run("happy-path", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.indices),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCNewestQC(ps.block.QC))),
		)
		ps.verifier.On("VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, proposal.LastViewTC.NewestQCViews).Return(nil).Once()
		err := ps.validator.ValidateProposal(proposal)
		require.NoError(ps.T(), err)
	})
	ps.Run("no-tc", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			// in this case proposal without LastViewTC is considered invalid
		)
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidProposalError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyQC")
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("tc-for-wrong-view", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.indices),
				helper.WithTCView(ps.block.View+10), // LastViewTC.View must be equal to Block.View-1
				helper.WithTCNewestQC(ps.block.QC))),
		)
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidProposalError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyQC")
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("proposal-not-safe-to-extend", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.indices),
				helper.WithTCView(ps.block.View+1),
				// proposal is not safe to extend because included QC.View is higher that Block.QC.View
				helper.WithTCNewestQC(helper.MakeQC(helper.WithQCView(ps.block.View+1))))),
		)
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidProposalError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyQC")
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("included-tc-highest-qc-not-highest", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.indices),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCNewestQC(ps.block.QC),
			)),
		)
		ps.verifier.On("VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, mock.Anything).Return(nil).Once()

		// this is considered an invalid TC, because highest QC's view is not equal to max{NewestQCViews}
		proposal.LastViewTC.NewestQCViews[0] = proposal.LastViewTC.NewestQC.View + 1
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidProposalError(err) && model.IsInvalidTCError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("included-tc-threshold-not-reached", func() {
		// TC is signed by only one signer - insufficient to reach weight threshold
		insufficientSignerIndices, err := signature.EncodeSignersToIndices(ps.participants.NodeIDs(), ps.participants.NodeIDs()[:1])
		require.NoError(ps.T(), err)
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(insufficientSignerIndices), // one signer is not enough to reach threshold
				helper.WithTCView(ps.block.View+1),
				helper.WithTCNewestQC(ps.block.QC),
			)),
		)
		err = ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidProposalError(err) && model.IsInvalidTCError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("included-tc-highest-qc-invalid", func() {
		// QC included in TC has view below QC included in proposal
		qc := helper.MakeQC(
			helper.WithQCView(ps.block.QC.View-1),
			helper.WithQCSigners(ps.indices))

		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.indices),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCNewestQC(qc))),
		)
		ps.verifier.On("VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, proposal.LastViewTC.NewestQCViews).Return(nil).Once()
		ps.verifier.On("VerifyQC", ps.voters, qc.SigData,
			qc.View, qc.BlockID).Return(model.ErrInvalidSignature).Once()
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidProposalError(err) && model.IsInvalidTCError(err))
	})
	ps.Run("verify-qc-err-view-for-unknown-epoch", func() {
		newestQC := helper.MakeQC(
			helper.WithQCView(ps.block.QC.View-2),
			helper.WithQCSigners(ps.indices))

		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.indices),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCNewestQC(newestQC))),
		)
		ps.verifier.On("VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, proposal.LastViewTC.NewestQCViews).Return(nil).Once()
		// Validating QC included in TC returns ErrViewForUnknownEpoch
		ps.verifier.On("VerifyQC", ps.voters, newestQC.SigData,
			newestQC.View, newestQC.BlockID).Return(model.ErrViewForUnknownEpoch).Once()
		err := ps.validator.ValidateProposal(proposal)
		require.Error(ps.T(), err)
		require.False(ps.T(), model.IsInvalidProposalError(err))
		require.False(ps.T(), model.IsInvalidTCError(err))
		require.NotErrorIs(ps.T(), err, model.ErrViewForUnknownEpoch)
	})
	ps.Run("included-tc-invalid-sig", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.indices),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCNewestQC(ps.block.QC))),
		)
		ps.verifier.On("VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, proposal.LastViewTC.NewestQCViews).Return(model.ErrInvalidSignature).Once()
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidProposalError(err) && model.IsInvalidTCError(err))
		ps.verifier.AssertCalled(ps.T(), "VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, proposal.LastViewTC.NewestQCViews)
	})
	ps.Run("last-view-successful-but-includes-tc", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.finalized+1),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.indices),
				helper.WithParentBlock(ps.parent)),
			),
			helper.WithLastViewTC(helper.MakeTC()),
		)
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidProposalError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.verifier.AssertExpectations(ps.T())
}

func TestValidateVote(t *testing.T) {
	suite.Run(t, new(VoteSuite))
}

type VoteSuite struct {
	suite.Suite
	signer    *flow.IdentitySkeleton
	block     *model.Block
	vote      *model.Vote
	verifier  *mocks.Verifier
	committee *mocks.Replicas
	validator *Validator
}

func (vs *VoteSuite) SetupTest() {

	// create a random signing identity
	vs.signer = &unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus)).IdentitySkeleton

	// create a block that should be signed
	vs.block = helper.MakeBlock()

	// create a vote for this block
	vs.vote = &model.Vote{
		View:     vs.block.View,
		BlockID:  vs.block.BlockID,
		SignerID: vs.signer.NodeID,
		SigData:  []byte{},
	}

	// set up the mocked verifier
	vs.verifier = &mocks.Verifier{}
	vs.verifier.On("VerifyVote", vs.signer, vs.vote.SigData, vs.block.View, vs.block.BlockID).Return(nil)

	// the leader for the block view is the correct one
	vs.committee = &mocks.Replicas{}
	vs.committee.On("IdentityByEpoch", mock.Anything, vs.signer.NodeID).Return(vs.signer, nil)

	// set up the validator with the mocked dependencies
	vs.validator = New(vs.committee, vs.verifier)
}

// TestVoteOK checks the happy case, which is the default for the suite
func (vs *VoteSuite) TestVoteOK() {
	_, err := vs.validator.ValidateVote(vs.vote)
	assert.NoError(vs.T(), err, "a valid vote should be accepted")
}

// TestVoteSignatureError checks that the Validator does not misinterpret
// unexpected exceptions for invalid votes.
func (vs *VoteSuite) TestVoteSignatureError() {
	*vs.verifier = mocks.Verifier{}
	vs.verifier.On("VerifyVote", vs.signer, vs.vote.SigData, vs.block.View, vs.block.BlockID).Return(fmt.Errorf("some exception"))

	// check that the vote is no longer validated
	_, err := vs.validator.ValidateVote(vs.vote)
	assert.Error(vs.T(), err, "a vote with error on signature validation should be rejected")
	assert.False(vs.T(), model.IsInvalidVoteError(err), "internal exception should not be interpreted as invalid vote")
}

// TestVoteVerifyVote_ErrViewForUnknownEpoch tests if ValidateVote correctly handles VerifyVote's ErrViewForUnknownEpoch sentinel error
// Validator shouldn't return a sentinel error here because this behavior is a symptom of internal bug, this behavior is not expected.
func (vs *VoteSuite) TestVoteVerifyVote_ErrViewForUnknownEpoch() {
	*vs.verifier = mocks.Verifier{}
	vs.verifier.On("VerifyVote", vs.signer, vs.vote.SigData, vs.block.View, vs.block.BlockID).Return(model.ErrViewForUnknownEpoch)

	// check that the vote is no longer validated
	_, err := vs.validator.ValidateVote(vs.vote)
	assert.Error(vs.T(), err)
	assert.False(vs.T(), model.IsInvalidVoteError(err), "internal exception should not be interpreted as invalid vote")
	assert.NotErrorIs(vs.T(), err, model.ErrViewForUnknownEpoch, "we don't expect a sentinel error here")
}

// TestVoteInvalidSignerID checks that the Validator correctly handles a vote
// with a SignerID that does not correspond to a valid consensus participant.
// In this case, the `hotstuff.DynamicCommittee` returns a `model.InvalidSignerError`,
// which the Validator should recognize as a symptom for an invalid vote.
// Hence, we expect the validator to return a `model.InvalidVoteError`.
func (vs *VoteSuite) TestVoteInvalidSignerID() {
	*vs.committee = mocks.Replicas{}
	vs.committee.On("IdentityByEpoch", vs.block.View, vs.vote.SignerID).Return(nil, model.NewInvalidSignerErrorf(""))

	// A `model.InvalidSignerError` from the committee should be interpreted as
	// the Vote being invalid, i.e. we expect an InvalidVoteError to be returned
	_, err := vs.validator.ValidateVote(vs.vote)
	assert.Error(vs.T(), err, "a vote with unknown SignerID should be rejected")
	assert.True(vs.T(), model.IsInvalidVoteError(err), "a vote with unknown SignerID should be rejected")
}

// TestVoteSignatureInvalid checks that the Validator correctly handles votes
// with cryptographically invalid signature. In this case, the `hotstuff.Verifier`
// returns a `model.ErrInvalidSignature`, which the Validator should recognize as
// a symptom for an invalid vote.
// Hence, we expect the validator to return a `model.InvalidVoteError`.
func (vs *VoteSuite) TestVoteSignatureInvalid() {
	*vs.verifier = mocks.Verifier{}
	vs.verifier.On("VerifyVote", vs.signer, vs.vote.SigData, vs.block.View, vs.block.BlockID).Return(fmt.Errorf("staking sig is invalid: %w", model.ErrInvalidSignature))

	// A `model.ErrInvalidSignature` from the `hotstuff.Verifier` should be interpreted as
	// the Vote being invalid, i.e. we expect an InvalidVoteError to be returned
	_, err := vs.validator.ValidateVote(vs.vote)
	assert.Error(vs.T(), err, "a vote with an invalid signature should be rejected")
	assert.True(vs.T(), model.IsInvalidVoteError(err), "a vote with an invalid signature should be rejected")
}

func TestValidateQC(t *testing.T) {
	suite.Run(t, new(QCSuite))
}

type QCSuite struct {
	suite.Suite
	participants flow.IdentitySkeletonList
	signers      flow.IdentitySkeletonList
	block        *model.Block
	qc           *flow.QuorumCertificate
	committee    *mocks.Replicas
	verifier     *mocks.Verifier
	validator    *Validator
}

func (qs *QCSuite) SetupTest() {
	// create a list of 10 nodes with 1-weight each
	qs.participants = unittest.IdentityListFixture(10,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithInitialWeight(1),
	).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

	// signers are a qualified majority at 7
	qs.signers = qs.participants[:7]

	// create a block that has the signers in its QC
	qs.block = helper.MakeBlock()
	indices, err := signature.EncodeSignersToIndices(qs.participants.NodeIDs(), qs.signers.NodeIDs())
	require.NoError(qs.T(), err)

	qs.qc = helper.MakeQC(helper.WithQCBlock(qs.block), helper.WithQCSigners(indices))

	// return the correct participants and identities from view state
	qs.committee = &mocks.Replicas{}
	qs.committee.On("IdentitiesByEpoch", mock.Anything).Return(
		func(_ uint64) flow.IdentitySkeletonList {
			return qs.participants
		},
		nil,
	)
	qs.committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(qs.participants.TotalWeight()), nil)

	// set up the mocked verifier to verify the QC correctly
	qs.verifier = &mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.signers, qs.qc.SigData, qs.qc.View, qs.qc.BlockID).Return(nil)

	// set up the validator with the mocked dependencies
	qs.validator = New(qs.committee, qs.verifier)
}

// TestQCOK verifies the default happy case
func (qs *QCSuite) TestQCOK() {

	// check the default happy case passes
	err := qs.validator.ValidateQC(qs.qc)
	assert.NoError(qs.T(), err, "a valid QC should be accepted")
}

// TestQCRetrievingParticipantsError tests that validation errors if:
// there is an error retrieving identities of consensus participants
func (qs *QCSuite) TestQCRetrievingParticipantsError() {
	// change the hotstuff.DynamicCommittee to fail on retrieving participants
	*qs.committee = mocks.Replicas{}
	qs.committee.On("IdentitiesByEpoch", mock.Anything).Return(qs.participants, errors.New("FATAL internal error"))

	// verifier should escalate unspecific internal error to surrounding logic, but NOT as ErrorInvalidQC
	err := qs.validator.ValidateQC(qs.qc)
	assert.Error(qs.T(), err, "unspecific error when retrieving consensus participants should be escalated to surrounding logic")
	assert.False(qs.T(), model.IsInvalidQCError(err), "unspecific internal errors should not result in ErrorInvalidQC error")
}

// TestQCSignersError tests that a qc fails validation if:
// QC signer's have insufficient weight (but are all valid consensus participants otherwise)
func (qs *QCSuite) TestQCInsufficientWeight() {
	// signers only have weight 6 out of 10 total (NOT have a supermajority)
	qs.signers = qs.participants[:6]
	indices, err := signature.EncodeSignersToIndices(qs.participants.NodeIDs(), qs.signers.NodeIDs())
	require.NoError(qs.T(), err)

	qs.qc = helper.MakeQC(helper.WithQCBlock(qs.block), helper.WithQCSigners(indices))

	// the QC should not be validated anymore
	err = qs.validator.ValidateQC(qs.qc)
	assert.Error(qs.T(), err, "a QC should be rejected if it has insufficient voted weight")

	// we should get a threshold error to bubble up for extra info
	assert.True(qs.T(), model.IsInvalidQCError(err), "if there is insufficient voted weight, an invalid block error should be raised")
}

// TestQCSignatureError tests that validation errors if:
// there is an unspecific internal error while validating the signature
func (qs *QCSuite) TestQCSignatureError() {

	// set up the verifier to fail QC verification
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.signers, qs.qc.SigData, qs.qc.View, qs.qc.BlockID).Return(errors.New("dummy error"))

	// verifier should escalate unspecific internal error to surrounding logic, but NOT as ErrorInvalidQC
	err := qs.validator.ValidateQC(qs.qc)
	assert.Error(qs.T(), err, "unspecific sig verification error should be escalated to surrounding logic")
	assert.False(qs.T(), model.IsInvalidQCError(err), "unspecific internal errors should not result in ErrorInvalidQC error")
}

// TestQCSignatureInvalid verifies that the Validator correctly handles the model.ErrInvalidSignature.
// This error return from `Verifier.VerifyQC` is an expected failure case in case of a byzantine input, where
// one of the signatures in the QC is broken. Hence, the Validator should wrap it as InvalidProposalError.
func (qs *QCSuite) TestQCSignatureInvalid() {
	// change the verifier to fail the QC signature
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.signers, qs.qc.SigData, qs.qc.View, qs.qc.BlockID).Return(fmt.Errorf("invalid qc: %w", model.ErrInvalidSignature))

	// the QC should no longer pass validation
	err := qs.validator.ValidateQC(qs.qc)
	assert.True(qs.T(), model.IsInvalidQCError(err), "if the signature is invalid an ErrorInvalidQC error should be raised")
}

// TestQCVerifyQC_ErrViewForUnknownEpoch tests if ValidateQC correctly handles VerifyQC's ErrViewForUnknownEpoch sentinel error
// Validator shouldn't return a sentinel error here because this behavior is a symptom of internal bug, this behavior is not expected.
func (qs *QCSuite) TestQCVerifyQC_ErrViewForUnknownEpoch() {
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.signers, qs.qc.SigData, qs.qc.View, qs.qc.BlockID).Return(model.ErrViewForUnknownEpoch)
	err := qs.validator.ValidateQC(qs.qc)
	assert.Error(qs.T(), err)
	assert.False(qs.T(), model.IsInvalidQCError(err), "we don't expect a sentinel error here")
	assert.NotErrorIs(qs.T(), err, model.ErrViewForUnknownEpoch, "we don't expect a sentinel error here")
}

// TestQCSignatureInvalidFormat verifies that the Validator correctly handles the model.InvalidFormatError.
// This error return from `Verifier.VerifyQC` is an expected failure case in case of a byzantine input, where
// some binary vector (e.g. `sigData`) is broken. Hence, the Validator should wrap it as InvalidProposalError.
func (qs *QCSuite) TestQCSignatureInvalidFormat() {
	// change the verifier to fail the QC signature
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.signers, qs.qc.SigData, qs.qc.View, qs.qc.BlockID).Return(model.NewInvalidFormatErrorf("invalid sigType"))

	// the QC should no longer pass validation
	err := qs.validator.ValidateQC(qs.qc)
	assert.True(qs.T(), model.IsInvalidQCError(err), "if the signature has an invalid format, an ErrorInvalidQC error should be raised")
}

// TestQCEmptySigners verifies that the Validator correctly handles the model.InsufficientSignaturesError:
// In the validator, we previously checked the total weight of all signers meets the supermajority threshold,
// which is a _positive_ number. Hence, there must be at least one signer. Hence, `Verifier.VerifyQC`
// returning this error would be a symptom of a fatal internal bug. The Validator should _not_ interpret
// this error as an invalid QC / invalid block, i.e. it should _not_ return an `InvalidProposalError`.
func (qs *QCSuite) TestQCEmptySigners() {
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", mock.Anything, qs.qc.SigData, qs.block.View, qs.block.BlockID).Return(
		fmt.Errorf("%w", model.NewInsufficientSignaturesErrorf("")))

	// the Validator should _not_ interpret this as a invalid QC, but as an internal error
	err := qs.validator.ValidateQC(qs.qc)
	assert.True(qs.T(), model.IsInsufficientSignaturesError(err)) // unexpected error should be wrapped and propagated upwards
	assert.False(qs.T(), model.IsInvalidProposalError(err), err, "should _not_ interpret this as a invalid QC, but as an internal error")
}

func TestValidateTC(t *testing.T) {
	suite.Run(t, new(TCSuite))
}

type TCSuite struct {
	suite.Suite
	participants flow.IdentitySkeletonList
	signers      flow.IdentitySkeletonList
	indices      []byte
	block        *model.Block
	tc           *flow.TimeoutCertificate
	committee    *mocks.DynamicCommittee
	verifier     *mocks.Verifier
	validator    *Validator
}

func (s *TCSuite) SetupTest() {

	// create a list of 10 nodes with 1-weight each
	s.participants = unittest.IdentityListFixture(10,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithInitialWeight(1),
	).Sort(flow.Canonical[flow.Identity]).ToSkeleton()

	// signers are a qualified majority at 7
	s.signers = s.participants[:7]

	var err error
	s.indices, err = signature.EncodeSignersToIndices(s.participants.NodeIDs(), s.signers.NodeIDs())
	require.NoError(s.T(), err)

	view := uint64(int(rand.Uint32()) + len(s.participants))

	highQCViews := make([]uint64, 0, len(s.signers))
	for i := range s.signers {
		highQCViews = append(highQCViews, view-uint64(i)-1)
	}

	rand.Shuffle(len(highQCViews), func(i, j int) {
		highQCViews[i], highQCViews[j] = highQCViews[j], highQCViews[i]
	})

	// create a block that has the signers in its QC
	parent := helper.MakeBlock(helper.WithBlockView(view - 1))
	s.block = helper.MakeBlock(helper.WithBlockView(view),
		helper.WithParentBlock(parent),
		helper.WithParentSigners(s.indices))
	s.tc = helper.MakeTC(helper.WithTCNewestQC(s.block.QC),
		helper.WithTCView(view+1),
		helper.WithTCSigners(s.indices),
		helper.WithTCHighQCViews(highQCViews))

	// return the correct participants and identities from view state
	s.committee = &mocks.DynamicCommittee{}
	s.committee.On("IdentitiesByEpoch", mock.Anything, mock.Anything).Return(
		func(view uint64) flow.IdentitySkeletonList {
			return s.participants
		},
		nil,
	)
	s.committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(s.participants.TotalWeight()), nil)

	s.verifier = &mocks.Verifier{}
	s.verifier.On("VerifyQC", s.signers, s.block.QC.SigData, parent.View, parent.BlockID).Return(nil)

	// set up the validator with the mocked dependencies
	s.validator = New(s.committee, s.verifier)
}

// TestTCOk tests if happy-path returns correct result
func (s *TCSuite) TestTCOk() {
	s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.NewestQCViews).Return(nil).Once()

	// check the default happy case passes
	err := s.validator.ValidateTC(s.tc)
	assert.NoError(s.T(), err, "a valid TC should be accepted")
}

// TestTCNewestQCFromFuture tests if correct error is returned when included QC is higher than TC's view
func (s *TCSuite) TestTCNewestQCFromFuture() {
	// highest QC from future view
	s.tc.NewestQC.View = s.tc.View + 1
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if NewestQC.View > TC.View, an ErrorInvalidTC error should be raised")
}

// TestTCNewestQCIsNotHighest tests if correct error is returned when included QC is not highest
func (s *TCSuite) TestTCNewestQCIsNotHighest() {
	s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData),
		s.tc.View, s.tc.NewestQCViews).Return(nil).Once()

	// highest QC view is not equal to max(TONewestQCViews)
	s.tc.NewestQCViews[0] = s.tc.NewestQC.View + 1
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if max(highQCViews) != NewestQC.View, an ErrorInvalidTC error should be raised")
}

// TestTCInvalidSigners tests if correct error is returned when signers are invalid
func (s *TCSuite) TestTCInvalidSigners() {
	s.participants = s.participants[1:] // remove participant[0] from the list of valid consensus participant
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if some signers are invalid consensus participants, an ErrorInvalidTC error should be raised")
}

// TestTCThresholdNotReached tests if correct error is returned when TC's singers don't have enough weight
func (s *TCSuite) TestTCThresholdNotReached() {
	// signers only have weight 1 out of 10 total (NOT have a supermajority)
	s.signers = s.participants[:1]
	indices, err := signature.EncodeSignersToIndices(s.participants.NodeIDs(), s.signers.NodeIDs())
	require.NoError(s.T(), err)

	s.tc.SignerIndices = indices

	// adjust signers to be less than total weight
	err = s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if signers don't have enough weight, an ErrorInvalidTC error should be raised")
}

// TestTCInvalidNewestQC tests if correct error is returned when included highest QC is invalid
func (s *TCSuite) TestTCInvalidNewestQC() {
	*s.verifier = mocks.Verifier{}
	s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.NewestQCViews).Return(nil).Once()
	s.verifier.On("VerifyQC", s.signers, s.tc.NewestQC.SigData, s.tc.NewestQC.View, s.tc.NewestQC.BlockID).Return(model.NewInvalidFormatErrorf("invalid qc")).Once()
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if included QC is invalid, an ErrorInvalidTC error should be raised")
}

// TestTCVerifyQC_ErrViewForUnknownEpoch tests if ValidateTC correctly handles VerifyQC's ErrViewForUnknownEpoch sentinel error
// Validator shouldn't return a sentinel error here because this behavior is a symptom of internal bug, this behavior is not expected.
func (s *TCSuite) TestTCVerifyQC_ErrViewForUnknownEpoch() {
	*s.verifier = mocks.Verifier{}
	s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.NewestQCViews).Return(nil).Once()
	s.verifier.On("VerifyQC", s.signers, s.tc.NewestQC.SigData, s.tc.NewestQC.View, s.tc.NewestQC.BlockID).Return(model.ErrViewForUnknownEpoch).Once()
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.Error(s.T(), err)
	assert.False(s.T(), model.IsInvalidTCError(err), "we don't expect a sentinel error here")
	assert.NotErrorIs(s.T(), err, model.ErrViewForUnknownEpoch, "we don't expect a sentinel error here")
}

// TestTCInvalidSignature tests a few scenarios when the signature is invalid or TC signers is malformed
func (s *TCSuite) TestTCInvalidSignature() {
	s.Run("insufficient-signatures", func() {
		*s.verifier = mocks.Verifier{}
		s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		s.verifier.On("VerifyTC", mock.Anything, []byte(s.tc.SigData), s.tc.View, s.tc.NewestQCViews).Return(model.NewInsufficientSignaturesErrorf("")).Once()

		// the Validator should _not_ interpret this as an invalid TC, but as an internal error
		err := s.validator.ValidateTC(s.tc)
		assert.True(s.T(), model.IsInsufficientSignaturesError(err)) // unexpected error should be wrapped and propagated upwards
		assert.False(s.T(), model.IsInvalidTCError(err), err, "should _not_ interpret this as a invalid TC, but as an internal error")
	})
	s.Run("invalid-format", func() {
		*s.verifier = mocks.Verifier{}
		s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.NewestQCViews).Return(model.NewInvalidFormatErrorf("")).Once()
		err := s.validator.ValidateTC(s.tc)
		assert.True(s.T(), model.IsInvalidTCError(err), "if included TC's inputs are invalid, an ErrorInvalidTC error should be raised")
	})
	s.Run("invalid-signature", func() {
		*s.verifier = mocks.Verifier{}
		s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.NewestQCViews).Return(model.ErrInvalidSignature).Once()
		err := s.validator.ValidateTC(s.tc)
		assert.True(s.T(), model.IsInvalidTCError(err), "if included TC's signature is invalid, an ErrorInvalidTC error should be raised")
	})
	s.Run("verify-sig-exception", func() {
		exception := errors.New("verify-sig-exception")
		*s.verifier = mocks.Verifier{}
		s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.NewestQCViews).Return(exception).Once()
		err := s.validator.ValidateTC(s.tc)
		assert.ErrorIs(s.T(), err, exception, "if included TC's signature is invalid, an exception should be propagated")
		assert.False(s.T(), model.IsInvalidTCError(err))
	})
}
