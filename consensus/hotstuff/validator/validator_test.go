package validator

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow/filter"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestValidateProposal(t *testing.T) {
	suite.Run(t, new(ProposalSuite))
}

type ProposalSuite struct {
	suite.Suite
	participants    flow.IdentityList
	leader          *flow.Identity
	finalized       uint64
	parent          *model.Block
	block           *model.Block
	proposal        *model.Proposal
	vote            *model.Vote
	membersState    *mocks.MembersState
	membersSnapshot *mocks.MembersSnapshot
	forks           *mocks.Forks
	verifier        *mocks.Verifier
	validator       *Validator
}

func (ps *ProposalSuite) SetupTest() {

	// the leader is a random node for now
	ps.finalized = uint64(rand.Uint32() + 1)
	ps.participants = unittest.IdentityListFixture(8, unittest.WithRole(flow.RoleConsensus))
	ps.leader = ps.participants[0]

	// the parent is the last finalized block, followed directly by a block from the leader
	ps.parent = helper.MakeBlock(ps.T(),
		helper.WithBlockView(ps.finalized),
	)
	ps.block = helper.MakeBlock(ps.T(),
		helper.WithBlockView(ps.finalized+1),
		helper.WithBlockProposer(ps.leader.NodeID),
		helper.WithParentBlock(ps.parent),
		helper.WithParentSigners(ps.participants.NodeIDs()),
	)
	ps.proposal = &model.Proposal{Block: ps.block}
	ps.vote = ps.proposal.ProposerVote()

	// the member state snapshot
	ps.membersSnapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return ps.participants.Filter(selector)
		},
		nil,
	)
	for _, participant := range ps.participants {
		ps.membersSnapshot.On("Identity", participant.NodeID).Return(participant, nil)
	}

	// the leader for the block view is the correct one
	ps.membersState = &mocks.MembersState{}
	ps.membersState.On("AtBlockID", mock.Anything).Return(ps.membersSnapshot)
	ps.membersState.On("LeaderForView", ps.block.View).Return(ps.leader)

	// the finalized view is the one of the parent of the
	ps.forks = &mocks.Forks{}
	ps.forks.On("FinalizedView").Return(ps.finalized)
	ps.forks.On("GetBlock", ps.parent.BlockID).Return(ps.parent, true)
	ps.forks.On("GetBlock", ps.block.BlockID).Return(ps.block, true)

	// set up the mocked verifier
	ps.verifier = &mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.block.QC.SignerIDs, ps.block.QC.SigData, ps.parent).Return(true, nil)
	ps.verifier.On("VerifyVote", ps.vote.SignerID, ps.vote.SigData, ps.block).Return(true, nil)

	// set up the validator with the mocked dependencies
	ps.validator = New(ps.membersState, ps.forks, ps.verifier)
}

func (ps *ProposalSuite) TestProposalOK() {

	err := ps.validator.ValidateProposal(ps.proposal)
	assert.NoError(ps.T(), err, "a valid proposal should be accepted")
}

func (ps *ProposalSuite) TestProposalSignatureError() {

	// change the verifier to error on signature validation
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.block.QC.SignerIDs, ps.block.QC.SigData, ps.parent).Return(true, nil)
	ps.verifier.On("VerifyVote", ps.vote.SignerID, ps.vote.SigData, ps.block).Return(true, errors.New("dummy error"))

	// check that validation now fails
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal should be rejected if signature check fails")

	// check that the error is not one that leads to invalid
	var invalid *model.ErrorInvalidBlock
	assert.True(ps.T(), errors.As(err, &invalid), "if signature check fails, we should not generate a invalid error")
}

func (ps *ProposalSuite) TestProposalSignatureInvalid() {

	// change the verifier to fail signature validation
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.block.QC.SignerIDs, ps.block.QC.SigData, ps.parent).Return(true, nil)
	ps.verifier.On("VerifyVote", ps.vote.SignerID, ps.vote.SigData, ps.block).Return(false, nil)

	// check that validation now fails
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid signature should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	var invalid *model.ErrorInvalidBlock
	assert.True(ps.T(), errors.As(err, &invalid), "if signature is invalid, we should generate an invalid error")
}

func (ps *ProposalSuite) TestProposalWrongLeader() {

	// change the MembersState to return a different leader
	ps.membersState = &mocks.MembersState{}
	ps.membersState.On("AtBlockID", mock.Anything).Return(ps.membersSnapshot)
	ps.membersState.On("LeaderForView", ps.block.View).Return(ps.participants[1])

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal from the wrong proposer should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	var invalid *model.ErrorInvalidBlock
	assert.True(ps.T(), errors.As(err, &invalid), "if the proposal has wrong proposer, we should generate a invalid error")
}

func (ps *ProposalSuite) TestProposalMismatchingView() {

	// change the QC's view to be different from the parent
	ps.proposal.Block.QC.View++

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with a mismatching QC view should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	var invalid *model.ErrorInvalidBlock
	assert.True(ps.T(), errors.As(err, &invalid), "if the QC has a mismatching view, we should generate a invalid error")
}

func (ps *ProposalSuite) TestProposalMissingParentHigher() {

	// change forks to not find the parent
	ps.block.QC.View = ps.finalized
	*ps.forks = mocks.Forks{}
	ps.forks.On("FinalizedView").Return(ps.finalized)
	ps.forks.On("GetBlock", ps.block.QC.BlockID).Return(nil, false)

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with a missing parent should be rejected")

	// check that the error is a missing block error because we should have the block but we don't
	var missing *model.ErrorMissingBlock
	assert.True(ps.T(), errors.As(err, &missing), "if we don't have the proposal parent for a QC above or equal finalized view, we should generate an missing block error")
}

func (ps *ProposalSuite) TestProposalMissingParentLower() {

	// change forks to not find the parent
	ps.block.QC.View = ps.finalized - 1
	*ps.forks = mocks.Forks{}
	ps.forks.On("FinalizedView").Return(ps.finalized)
	ps.forks.On("GetBlock", ps.block.QC.BlockID).Return(nil, false)

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with a missing parent should be rejected")

	// check that the error is an unverifiable block because we can't verify the block
	assert.True(ps.T(), errors.Is(err, model.ErrUnverifiableBlock), "if we don't have the proposal parent for a QC below finalized view, we should generate an unverifiable block error")
}

func (ps *ProposalSuite) TestProposalQCInvalid() {

	// change verifier to fail on QC validation
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.block.QC.SignerIDs, ps.block.QC.SigData, ps.parent).Return(false, nil)
	ps.verifier.On("VerifyVote", ps.vote.SignerID, ps.vote.SigData, ps.block).Return(true, nil)

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid QC should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	var invalid *model.ErrorInvalidBlock
	assert.True(ps.T(), errors.As(err, &invalid), "if the QC is invalid, we should generate a invalid error")
}

func (ps *ProposalSuite) TestProposalQCError() {

	// change Members State Snapshot to error when getting stuff
	ps.membersSnapshot = &mocks.MembersSnapshot{}
	ps.membersSnapshot.On("Identities", mock.Anything).Return(ps.participants, errors.New("dummy error"))
	ps.membersSnapshot.On("Identity", mock.Anything).Return(ps.participants[0], errors.New("dummy error"))

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid QC should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	var invalid *model.ErrorInvalidBlock
	assert.False(ps.T(), errors.As(err, &invalid), "if we can't verify the QC, we should not generate a invalid error")
}

func TestValidateVote(t *testing.T) {
	suite.Run(t, new(VoteSuite))
}

type VoteSuite struct {
	suite.Suite
	signer          *flow.Identity
	block           *model.Block
	vote            *model.Vote
	forks           *mocks.Forks
	verifier        *mocks.Verifier
	membersState    *mocks.MembersState
	membersSnapshot *mocks.MembersSnapshot
	validator       *Validator
}

func (vs *VoteSuite) SetupTest() {

	// create a random signing identity
	vs.signer = unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	// create a block that should be signed
	vs.block = helper.MakeBlock(vs.T())

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
	vs.verifier.On("VerifyVote", vs.vote.SignerID, vs.vote.SigData, vs.block).Return(true, nil)

	// the leader for the block view is the correct one
	vs.membersSnapshot = &mocks.MembersSnapshot{}
	vs.membersSnapshot.On("Identity", vs.signer.NodeID).Return(vs.signer, nil)
	vs.membersState = &mocks.MembersState{}
	vs.membersState.On("AtBlockID", vs.block.BlockID).Return(vs.membersSnapshot, nil)

	// set up the validator with the mocked dependencies
	vs.validator = New(vs.membersState, vs.forks, vs.verifier)
}

func (vs *VoteSuite) TestVoteOK() {

	// check the happy case, which is the default for the suite
	_, err := vs.validator.ValidateVote(vs.vote, vs.block)
	assert.NoError(vs.T(), err, "a valid vote should be accepted")
}

func (vs *VoteSuite) TestVoteMismatchingView() {

	// make the view on the vote different
	vs.vote.View++

	// check that the vote is no longer validated
	_, err := vs.validator.ValidateVote(vs.vote, vs.block)
	assert.Error(vs.T(), err, "a vote with a mismatching view should be rejected")

	// TODO: this should raise an error that allows a slashing challenge
	var malformedVote *model.ErrorInvalidVote
	assert.True(vs.T(), errors.As(err, &malformedVote), "a mismatching view should create a invalid vote error")
}

func (vs *VoteSuite) TestVoteSignatureError() {

	// make the verification fail on signature
	*vs.verifier = mocks.Verifier{}
	vs.verifier.On("VerifyVote", vs.vote.SignerID, vs.vote.SigData, vs.block).Return(true, errors.New("dummy error"))

	// check that the vote is no longer validated
	_, err := vs.validator.ValidateVote(vs.vote, vs.block)
	assert.Error(vs.T(), err, "a vote with error on signature validation should be rejected")
}

func (vs *VoteSuite) TestVoteSignatureInvalid() {

	// make sure the signature is treated as invalid
	*vs.verifier = mocks.Verifier{}
	vs.verifier.On("VerifyVote", vs.vote.SignerID, vs.vote.SigData, vs.block).Return(false, nil)

	// check that the vote is no longer validated
	_, err := vs.validator.ValidateVote(vs.vote, vs.block)
	assert.Error(vs.T(), err, "a vote with an invalid signature should be rejected")
}

func TestValidateQC(t *testing.T) {
	suite.Run(t, new(QCSuite))
}

type QCSuite struct {
	suite.Suite
	participants    flow.IdentityList
	signers         flow.IdentityList
	block           *model.Block
	qc              *model.QuorumCertificate
	membersState    *mocks.MembersState
	membersSnapshot *mocks.MembersSnapshot
	verifier        *mocks.Verifier
	validator       *Validator
}

func (qs *QCSuite) SetupTest() {

	// create a list of 10 nodes with one stake each
	qs.participants = unittest.IdentityListFixture(10,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithStake(1),
	)

	// signers are a qualified majority at 7
	qs.signers = qs.participants[:7]

	// create a block that has the signers in its QC
	qs.block = helper.MakeBlock(qs.T())
	qs.qc = helper.MakeQC(qs.T(), helper.WithQCBlock(qs.block), helper.WithQCSigners(qs.signers.NodeIDs()))

	// return the correct participants and identities from view state
	qs.membersSnapshot = &mocks.MembersSnapshot{}
	qs.membersSnapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return qs.participants.Filter(selector)
		},
		nil,
	)
	qs.membersState = &mocks.MembersState{}
	qs.membersState.On("AtBlockID", qs.qc.BlockID).Return(qs.membersSnapshot, nil)

	// set up the mocked verifier to verify the QC correctly
	qs.verifier = &mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.qc.SignerIDs, qs.qc.SigData, qs.block).Return(true, nil)

	// set up the validator with the mocked dependencies
	qs.validator = New(qs.membersState, nil, qs.verifier)
}

func (qs *QCSuite) TestQCOK() {

	// check the default happy case passes
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.NoError(qs.T(), err, "a valid QC should be accepted")
}

// TestQCSignersError tests that a qc fails validation if:
// QC signer's Identities cannot all be retrieved
// (some are not valid consensus participants or retrieval fails with unspecific error)
func (qs *QCSuite) TestQCSignersError() {
	// change the MembersState to fail at retrieving participants
	qs.membersSnapshot = &mocks.MembersSnapshot{}
	qs.membersSnapshot.On("Identities", filter.Any).Return(qs.participants, nil)
	qs.membersSnapshot.On("Identities", qs.signers.NodeIDs()).Return(qs.signers[1:], nil)
	err := qs.validator.ValidateQC(qs.qc, qs.block) // the QC should not be validated anymore
	assert.Error(qs.T(), err, "a QC should be rejected if we can't retrieve signer identities")
	assert.True(qs.T(), errors.Is(err, model.ErrorInvalidBlock{}))

	// change the MembersState to fail at retrieving participants
	qs.membersSnapshot = &mocks.MembersSnapshot{}
	qs.membersSnapshot.On("Identities", filter.Any).Return(qs.participants, nil)
	qs.membersSnapshot.On("Identities", qs.signers.NodeIDs()).Return(qs.signers[1:], errors.New("FATAL internal error"))
	err = qs.validator.ValidateQC(qs.qc, qs.block) // the QC should not be validated anymore
	assert.Error(qs.T(), err, "expecting fatal internal error to be escalated to surrounding logic")
	assert.False(qs.T(), errors.Is(err, model.ErrorInvalidBlock{}), "unspecific internal errors should not result in ErrorInvalidBlock error")
}

// TestQCParticipantsError tests that validation errors if:
// there is an error retrieving identities of all consensus participants
func (qs *QCSuite) TestQCParticipantsError() {
	// change the MembersState to fail on participants
	qs.membersSnapshot = &mocks.MembersSnapshot{}
	qs.membersSnapshot.On("Identities", filter.Any).Return(qs.participants, errors.New("dummy error"))
	qs.membersSnapshot.On("Identities", qs.signers.NodeIDs()).Return(qs.signers, nil)

	// the QC should not be validated anymore
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.Error(qs.T(), err, "expecting fatal internal error to be escalated to surrounding logic")
	assert.False(qs.T(), errors.Is(err, model.ErrorInvalidBlock{}), "unspecific internal errors should not result in ErrorInvalidBlock error")
}

// TestQCSignersError tests that a qc fails validation if:
// QC signer's have insufficient stake (but are all valid consensus participants otherwise)
func (qs *QCSuite) TestQCInsufficientStake() {
	// signers only have stake 6 out of 10 total (NOT have a supermajority)
	qs.signers = qs.participants[:6]

	// the QC should not be validated anymore
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.Error(qs.T(), err, "a QC should be rejected if it has insufficient voted stake")

	// we should get a threshold error to bubble up for extra info
	var invalid *model.ErrorInvalidBlock
	assert.True(qs.T(), errors.As(err, &invalid), "if there is insufficient voted stake, an invalid block error should be raised")
}

func (qs *QCSuite) TestQCSignatureError() {

	// set up the verifier to fail QC verification
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.qc.SignerIDs, qs.qc.SigData, qs.block).Return(true, errors.New("dummy error"))

	// the QC should not be validated anymore
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.Error(qs.T(), err, "a QC should be rejected if we fail to verify it")
}

func (qs *QCSuite) TestQCSignatureInvalid() {

	// change the verifier to fail the QC signature
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.qc.SignerIDs, qs.qc.SigData, qs.block).Return(false, nil)

	// the QC should no longer be validation
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.Error(qs.T(), err, "a QC should be rejected if the signature is invalid")

}
