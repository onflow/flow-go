package validator

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

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
	leader       *flow.Identity
	finalized    uint64
	parent       *model.Block
	block        *model.Block
	voters       flow.IdentityList
	proposal     *model.Proposal
	vote         *model.Vote
	voter        *flow.Identity
	committee    *mocks.DynamicCommittee
	forks        *mocks.Forks
	verifier     *mocks.Verifier
	validator    *Validator
}

func (ps *ProposalSuite) SetupTest() {
	// the leader is a random node for now
	rand.Seed(time.Now().UnixNano())
	ps.finalized = uint64(rand.Uint32() + 1)
	ps.participants = unittest.IdentityListFixture(8, unittest.WithRole(flow.RoleConsensus))
	ps.leader = ps.participants[0]

	// the parent is the last finalized block, followed directly by a block from the leader
	ps.parent = helper.MakeBlock(
		helper.WithBlockView(ps.finalized),
	)
	ps.block = helper.MakeBlock(
		helper.WithBlockView(ps.finalized+1),
		helper.WithBlockProposer(ps.leader.NodeID),
		helper.WithParentBlock(ps.parent),
		helper.WithParentSigners(ps.participants.NodeIDs()),
	)
	ps.voters = ps.participants.Filter(filter.HasNodeID(ps.block.QC.SignerIDs...))
	ps.proposal = &model.Proposal{Block: ps.block}
	ps.vote = ps.proposal.ProposerVote()
	ps.voter = ps.leader

	// set up the mocked hotstuff DynamicCommittee state
	ps.committee = &mocks.DynamicCommittee{}
	ps.committee.On("LeaderForView", ps.block.View).Return(ps.leader.NodeID, nil)
	ps.committee.On("WeightThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(ps.participants.TotalWeight()), nil)
	ps.committee.On("IdentitiesByEpoch", mock.Anything, mock.Anything).Return(
		func(_ uint64, selector flow.IdentityFilter) flow.IdentityList {
			return ps.participants.Filter(selector)
		},
		nil,
	)
	for _, participant := range ps.participants {
		ps.committee.On("IdentityByEpoch", mock.Anything, participant.NodeID).Return(participant, nil)
		ps.committee.On("IdentityByBlock", mock.Anything, participant.NodeID).Return(participant, nil)
	}

	// the finalized view is the one of the parent of the
	ps.forks = &mocks.Forks{}
	ps.forks.On("FinalizedView").Return(ps.finalized)
	ps.forks.On("GetBlock", ps.parent.BlockID).Return(ps.parent, true)
	ps.forks.On("GetBlock", ps.block.BlockID).Return(ps.block, true)

	// set up the mocked verifier
	ps.verifier = &mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(nil)
	ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

	// set up the validator with the mocked dependencies
	ps.validator = New(ps.committee, ps.forks, ps.verifier)
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
	assert.False(ps.T(), model.IsInvalidBlockError(err), "if signature check fails, we should not receive an ErrorInvalidBlock")
}

func (ps *ProposalSuite) TestProposalSignatureInvalidFormat() {

	// change the verifier to fail signature validation with ErrInvalidFormat error
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(nil)
	ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(fmt.Errorf("%w", model.ErrInvalidFormat))

	// check that validation now fails
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid signature should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.True(ps.T(), model.IsInvalidBlockError(err), "if signature is invalid, we should generate an invalid error")
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
	assert.True(ps.T(), model.IsInvalidBlockError(err), "if signature is invalid, we should generate an invalid error")
}

func (ps *ProposalSuite) TestProposalWrongLeader() {

	// change the hotstuff.DynamicCommittee to return a different leader
	*ps.committee = mocks.DynamicCommittee{}
	ps.committee.On("LeaderForView", ps.block.View).Return(ps.participants[1].NodeID, nil)
	for _, participant := range ps.participants {
		ps.committee.On("IdentityByEpoch", mock.Anything, participant.NodeID).Return(participant, nil)
		ps.committee.On("IdentityByBlock", mock.Anything, participant.NodeID).Return(participant, nil)
	}

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal from the wrong proposer should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.True(ps.T(), model.IsInvalidBlockError(err), "if the proposal has wrong proposer, we should generate a invalid error")
}

// TestProposalQCInvalid checks that Validator handles the verifier's error returns correctly.
// In case of `model.ErrInvalidFormat` and model.ErrInvalidSignature`, we expect the Validator
// to recognize those as an invalid QC, i.e. returns an `model.InvalidBlockError`.
// In contrast, unexpected exceptions and `model.InvalidSignerError` should _not_ be
// interpreted as a sign of an invalid QC.
func (ps *ProposalSuite) TestProposalQCInvalid() {
	ps.Run("invalid signature", func() {
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(
			fmt.Errorf("invalid qc: %w", model.ErrInvalidSignature))
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure case is recognized as an invalid block
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.True(ps.T(), model.IsInvalidBlockError(err), "if the block's QC signature is invalid, an ErrorInvalidBlock error should be raised")
	})

	ps.Run("invalid format", func() {
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(
			fmt.Errorf("invalid qc: %w", model.ErrInvalidFormat))
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure case is recognized as an invalid block
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.True(ps.T(), model.IsInvalidBlockError(err), "if the block's QC has an invalid format, an ErrorInvalidBlock error should be raised")
	})

	// Theoretically, `VerifyQC` could also return a `model.InvalidSignerError`. However,
	// for the time being, we assume that _every_ HotStuff participant is also a member of
	// the random beacon committee. Consequently, `InvalidSignerError` should not occur atm.
	// TODO: if the random beacon committee is a strict subset of the HotStuff committee,
	//       we expect `model.InvalidSignerError` here during normal operations.
	ps.Run("invalid signer", func() {
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(
			fmt.Errorf("invalid qc: %w", model.NewInvalidSignerErrorf("")))
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure case is recognized as an invalid block
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.Error(ps.T(), err)
		assert.False(ps.T(), model.IsInvalidBlockError(err))
	})

	ps.Run("unknown exception", func() {
		exception := errors.New("exception")
		*ps.verifier = mocks.Verifier{}
		ps.verifier.On("VerifyQC", ps.voters, ps.block.QC.SigData, ps.parent.View, ps.parent.BlockID).Return(exception)
		ps.verifier.On("VerifyVote", ps.voter, ps.vote.SigData, ps.block.View, ps.block.BlockID).Return(nil)

		// check that validation fails and the failure case is recognized as an invalid block
		err := ps.validator.ValidateProposal(ps.proposal)
		assert.ErrorIs(ps.T(), err, exception)
		assert.False(ps.T(), model.IsInvalidBlockError(err))
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
	assert.False(ps.T(), model.IsInvalidBlockError(err), "if we can't verify the QC, we should not generate a invalid error")
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
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.participants.NodeIDs()),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCHighestQC(ps.block.QC))),
		)
		ps.verifier.On("VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, proposal.LastViewTC.TOHighQCViews).Return(nil).Once()
		err := ps.validator.ValidateProposal(proposal)
		require.NoError(ps.T(), err)
	})
	ps.Run("no-tc", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			// in this case proposal without LastViewTC is considered invalid
		)
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidBlockError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyQC")
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("tc-for-wrong-view", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.participants.NodeIDs()),
				helper.WithTCView(ps.block.View+10), // LastViewTC.View must be equal to Block.View-1
				helper.WithTCHighestQC(ps.block.QC))),
		)
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidBlockError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyQC")
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("proposal-not-safe-to-extend", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.participants.NodeIDs()),
				helper.WithTCView(ps.block.View+1),
				// proposal is not safe to extend because included QC.View is higher that Block.View
				helper.WithTCHighestQC(helper.MakeQC(helper.WithQCView(ps.block.View+1))))),
		)
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidBlockError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyQC")
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("included-tc-invalid-structure", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.participants.NodeIDs()),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCHighestQC(ps.block.QC))),
		)
		// after this operation TC is invalid
		proposal.LastViewTC.TOHighQCViews = proposal.LastViewTC.TOHighQCViews[1:]
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidBlockError(err) && model.IsInvalidTCError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("included-tc-highest-qc-not-highest", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.participants.NodeIDs()),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCHighestQC(ps.block.QC),
			)),
		)
		// this is considered an invalid TC, because highest QC's view is not equal to max{TOHighQCViews}
		proposal.LastViewTC.TOHighQCViews[0] = proposal.LastViewTC.TOHighestQC.View + 1
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidBlockError(err) && model.IsInvalidTCError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("included-tc-threshold-not-reached", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.participants.NodeIDs()[:1]), // one signer is not enough to reach threshold
				helper.WithTCView(ps.block.View+1),
				helper.WithTCHighestQC(ps.block.QC),
			)),
		)
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidBlockError(err) && model.IsInvalidTCError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("included-tc-highest-qc-invalid", func() {
		qc := helper.MakeQC(
			helper.WithQCView(ps.block.QC.View-1),
			helper.WithQCSigners(ps.voters.NodeIDs()))

		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.participants.NodeIDs()),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCHighestQC(qc))),
		)
		ps.verifier.On("VerifyQC", ps.voters, qc.SigData,
			qc.View, qc.BlockID).Return(model.ErrInvalidSignature).Once()
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidBlockError(err) && model.IsInvalidTCError(err))
		ps.verifier.AssertNotCalled(ps.T(), "VerifyTC")
	})
	ps.Run("included-tc-invalid-sig", func() {
		proposal := helper.MakeProposal(
			helper.WithBlock(helper.MakeBlock(
				helper.WithBlockView(ps.block.View+2),
				helper.WithBlockProposer(ps.leader.NodeID),
				helper.WithParentSigners(ps.participants.NodeIDs()),
				helper.WithBlockQC(ps.block.QC)),
			),
			helper.WithLastViewTC(helper.MakeTC(
				helper.WithTCSigners(ps.participants.NodeIDs()),
				helper.WithTCView(ps.block.View+1),
				helper.WithTCHighestQC(ps.block.QC))),
		)
		ps.verifier.On("VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, proposal.LastViewTC.TOHighQCViews).Return(model.ErrInvalidSignature).Once()
		err := ps.validator.ValidateProposal(proposal)
		require.True(ps.T(), model.IsInvalidBlockError(err) && model.IsInvalidTCError(err))
		ps.verifier.AssertCalled(ps.T(), "VerifyTC", ps.voters, []byte(proposal.LastViewTC.SigData),
			proposal.LastViewTC.View, proposal.LastViewTC.TOHighQCViews)
	})
}

func TestValidateVote(t *testing.T) {
	suite.Run(t, new(VoteSuite))
}

type VoteSuite struct {
	suite.Suite
	signer    *flow.Identity
	block     *model.Block
	vote      *model.Vote
	forks     *mocks.Forks
	verifier  *mocks.Verifier
	committee *mocks.DynamicCommittee
	validator *Validator
}

func (vs *VoteSuite) SetupTest() {

	// create a random signing identity
	vs.signer = unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	// create a block that should be signed
	vs.block = helper.MakeBlock()

	// create a vote for this block
	vs.vote = &model.Vote{
		View:     vs.block.View,
		BlockID:  vs.block.BlockID,
		SignerID: vs.signer.NodeID,
		SigData:  []byte{},
	}

	// set up the mocked forks
	vs.forks = &mocks.Forks{}
	vs.forks.On("GetBlock", vs.block.BlockID).Return(vs.block, true)

	// set up the mocked verifier
	vs.verifier = &mocks.Verifier{}
	vs.verifier.On("VerifyVote", vs.signer, vs.vote.SigData, vs.block.View, vs.block.BlockID).Return(nil)

	// the leader for the block view is the correct one
	vs.committee = &mocks.DynamicCommittee{}
	vs.committee.On("IdentityByEpoch", mock.Anything, vs.signer.NodeID).Return(vs.signer, nil)

	// set up the validator with the mocked dependencies
	vs.validator = New(vs.committee, vs.forks, vs.verifier)
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

// TestVoteInvalidSignerID checks that the Validator correctly handles a vote
// with a SignerID that does not correspond to a valid consensus participant.
// In this case, the `hotstuff.DynamicCommittee` returns a `model.InvalidSignerError`,
// which the Validator should recognize as a symptom for an invalid vote.
// Hence, we expect the validator to return a `model.InvalidVoteError`.
func (vs *VoteSuite) TestVoteInvalidSignerID() {
	*vs.committee = mocks.DynamicCommittee{}
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
	participants flow.IdentityList
	signers      flow.IdentityList
	block        *model.Block
	qc           *flow.QuorumCertificate
	committee    *mocks.DynamicCommittee
	verifier     *mocks.Verifier
	validator    *Validator
}

func (qs *QCSuite) SetupTest() {

	// create a list of 10 nodes with 1-weight each
	qs.participants = unittest.IdentityListFixture(10,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithWeight(1),
	)

	// signers are a qualified majority at 7
	qs.signers = qs.participants[:7]

	// create a block that has the signers in its QC
	qs.block = helper.MakeBlock()
	qs.qc = helper.MakeQC(helper.WithQCBlock(qs.block), helper.WithQCSigners(qs.signers.NodeIDs()))

	// return the correct participants and identities from view state
	qs.committee = &mocks.DynamicCommittee{}
	qs.committee.On("IdentitiesByEpoch", mock.Anything, mock.Anything).Return(
		func(_ uint64, selector flow.IdentityFilter) flow.IdentityList {
			return qs.participants.Filter(selector)
		},
		nil,
	)
	qs.committee.On("WeightThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(qs.participants.TotalWeight()), nil)

	// set up the mocked verifier to verify the QC correctly
	qs.verifier = &mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.signers, qs.qc.SigData, qs.qc.View, qs.qc.BlockID).Return(nil)

	// set up the validator with the mocked dependencies
	qs.validator = New(qs.committee, nil, qs.verifier)
}

func (qs *QCSuite) TestQCOK() {

	// check the default happy case passes
	err := qs.validator.ValidateQC(qs.qc)
	assert.NoError(qs.T(), err, "a valid QC should be accepted")
}

// TestQCInvalidSignersError tests that a qc fails validation if:
// QC signer's Identities cannot all be retrieved (some are not valid consensus participants)
func (qs *QCSuite) TestQCInvalidSignersError() {
	qs.participants = qs.participants[1:] // remove participant[0] from the list of valid consensus participant
	err := qs.validator.ValidateQC(qs.qc) // the QC should not be validated anymore
	assert.True(qs.T(), model.IsInvalidQCError(err), "if some signers are invalid consensus participants, an ErrorInvalidQC error should be raised")
}

// TestQCRetrievingParticipantsError tests that validation errors if:
// there is an error retrieving identities of consensus participants
func (qs *QCSuite) TestQCRetrievingParticipantsError() {
	// change the hotstuff.DynamicCommittee to fail on retrieving participants
	*qs.committee = mocks.DynamicCommittee{}
	qs.committee.On("IdentitiesByEpoch", mock.Anything, mock.Anything).Return(qs.participants, errors.New("FATAL internal error"))

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
	qs.qc = helper.MakeQC(helper.WithQCBlock(qs.block), helper.WithQCSigners(qs.signers.NodeIDs()))

	// the QC should not be validated anymore
	err := qs.validator.ValidateQC(qs.qc)
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

func (qs *QCSuite) TestQCSignatureInvalid() {

	// change the verifier to fail the QC signature
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.signers, qs.qc.SigData, qs.qc.View, qs.qc.BlockID).Return(fmt.Errorf("invalid qc: %w", model.ErrInvalidSignature))

	// the QC should no longer be validation
	err := qs.validator.ValidateQC(qs.qc)
	assert.True(qs.T(), model.IsInvalidQCError(err), "if the signature is invalid an ErrorInvalidQC error should be raised")
}

func (qs *QCSuite) TestQCSignatureInvalidFormat() {

	// change the verifier to fail the QC signature
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.signers, qs.qc.SigData, qs.qc.View, qs.qc.BlockID).Return(fmt.Errorf("%w", model.ErrInvalidFormat))

	// the QC should no longer be validation
	err := qs.validator.ValidateQC(qs.qc)
	assert.True(qs.T(), model.IsInvalidQCError(err), "if the signature has an invalid format, an ErrorInvalidQC error should be raised")
}

func TestValidateTC(t *testing.T) {
	suite.Run(t, new(TCSuite))
}

type TCSuite struct {
	suite.Suite
	participants flow.IdentityList
	signers      flow.IdentityList
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
		unittest.WithWeight(1),
	)

	// signers are a qualified majority at 7
	s.signers = s.participants[:7]

	rand.Seed(time.Now().UnixNano())
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
		helper.WithParentSigners(s.signers.NodeIDs()))
	s.tc = helper.MakeTC(helper.WithTCHighestQC(s.block.QC),
		helper.WithTCView(view+1),
		helper.WithTCSigners(s.signers.NodeIDs()),
		helper.WithTCHighQCViews(highQCViews))

	// return the correct participants and identities from view state
	s.committee = &mocks.DynamicCommittee{}
	s.committee.On("IdentitiesByEpoch", mock.Anything, mock.Anything).Return(
		func(view uint64, selector flow.IdentityFilter) flow.IdentityList {
			return s.participants.Filter(selector)
		},
		nil,
	)

	s.verifier = &mocks.Verifier{}
	s.verifier.On("VerifyQC", s.signers, s.block.QC.SigData, parent.View, parent.BlockID).Return(nil)

	// set up the validator with the mocked dependencies
	s.validator = New(s.committee, nil, s.verifier)
}

// TestTCOk tests if happy-path returns correct result
func (s *TCSuite) TestTCOk() {
	s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.TOHighQCViews).Return(nil).Once()

	// check the default happy case passes
	err := s.validator.ValidateTC(s.tc)
	assert.NoError(s.T(), err, "a valid TC should be accepted")
}

// TestTCEmptySigners tests if correct error is returned when signers are empty
func (s *TCSuite) TestTCEmptySigners() {
	s.tc.SignerIDs = []flow.Identifier{}
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "tc must have at least one signer, an ErrorInvalidTC error should be raised")
}

// TestTCHighQCViews tests if correct error is returned when high qc views are invalid
func (s *TCSuite) TestTCHighQCViews() {
	s.tc.TOHighQCViews = s.tc.TOHighQCViews[1:]
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if highQCViews len is not equal to signers len, an ErrorInvalidTC error should be raised")
}

// TestTCHighestQCFromFuture tests if correct error is returned when included QC is higher than TC's view
func (s *TCSuite) TestTCHighestQCFromFuture() {
	// highest QC from future view
	s.tc.TOHighestQC.View = s.tc.View + 1
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if TOHighestQC.View > TC.View, an ErrorInvalidTC error should be raised")
}

// TestTCHighestQCIsNotHighest tests if correct error is returned when included QC is not highest
func (s *TCSuite) TestTCHighestQCIsNotHighest() {
	// highest QC view is not equal to max(TOHighestQCViews)
	s.tc.TOHighQCViews[0] = s.tc.TOHighestQC.View + 1
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if max(highQCViews) != TOHighestQC.View, an ErrorInvalidTC error should be raised")
}

// TestTCInvalidSigners tests if correct error is returned when signers are invalid
func (s *TCSuite) TestTCInvalidSigners() {
	s.participants = s.participants[1:] // remove participant[0] from the list of valid consensus participant
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if some signers are invalid consensus participants, an ErrorInvalidTC error should be raised")
}

// TestTCThresholdNotReached tests if correct error is returned when TC's singers don't have enough weight
func (s *TCSuite) TestTCThresholdNotReached() {
	s.tc.SignerIDs = s.tc.SignerIDs[:1]
	s.tc.TOHighQCViews = s.tc.TOHighQCViews[:1]
	// make sure that there is the highest view
	s.tc.TOHighQCViews[0] = s.tc.TOHighestQC.View

	// adjust signers to be less than total weight
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if signers don't have enough weight, an ErrorInvalidTC error should be raised")
}

// TestTCInvalidHighestQC tests if correct error is returned when included highest QC is invalid
func (s *TCSuite) TestTCInvalidHighestQC() {
	*s.verifier = mocks.Verifier{}
	s.verifier.On("VerifyQC", s.signers, s.tc.TOHighestQC.SigData, s.tc.TOHighestQC.View, s.tc.TOHighestQC.BlockID).Return(fmt.Errorf("invalid qc: %w", model.ErrInvalidFormat)).Once()
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if included QC is invalid, an ErrorInvalidTC error should be raised")
}

// TestTCInvalidSignature tests a few scenarios when the signature is invalid or TC signers is malformed
func (s *TCSuite) TestTCInvalidSignature() {
	s.Run("invalid-format", func() {
		*s.verifier = mocks.Verifier{}
		s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.TOHighQCViews).Return(model.ErrInvalidFormat).Once()
		err := s.validator.ValidateTC(s.tc)
		assert.True(s.T(), model.IsInvalidTCError(err), "if included TC's inputs are invalid, an ErrorInvalidTC error should be raised")
	})
	s.Run("invalid-signature", func() {
		*s.verifier = mocks.Verifier{}
		s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.TOHighQCViews).Return(model.ErrInvalidSignature).Once()
		err := s.validator.ValidateTC(s.tc)
		assert.True(s.T(), model.IsInvalidTCError(err), "if included TC's signature is invalid, an ErrorInvalidTC error should be raised")
	})
	s.Run("verify-sig-exception", func() {
		exception := errors.New("verify-sig-exception")
		*s.verifier = mocks.Verifier{}
		s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		s.verifier.On("VerifyTC", s.signers, []byte(s.tc.SigData), s.tc.View, s.tc.TOHighQCViews).Return(exception).Once()
		err := s.validator.ValidateTC(s.tc)
		assert.ErrorAs(s.T(), err, &exception, "if included TC's signature is invalid, an exception should be propagated")
		assert.False(s.T(), model.IsInvalidTCError(err))
	})
}
