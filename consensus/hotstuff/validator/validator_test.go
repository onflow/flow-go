package validator

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/verification"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
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
	proposal     *model.Proposal
	vote         *model.Vote
	committee    *mocks.Committee
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

	// set up the mocked hotstuff Committee state
	ps.committee = &mocks.Committee{}
	ps.committee.On("LeaderForView", ps.block.View).Return(ps.leader.NodeID, nil)
	ps.committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return ps.participants.Filter(selector)
		},
		nil,
	)
	for _, participant := range ps.participants {
		ps.committee.On("Identity", mock.Anything, participant.NodeID).Return(participant, nil)
	}

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
	ps.validator = New(ps.committee, ps.forks, ps.verifier)
}

func (ps *ProposalSuite) TestProposalOK() {

	err := ps.validator.ValidateProposal(ps.proposal)
	assert.NoError(ps.T(), err, "a valid proposal should be accepted")
}

func (ps *ProposalSuite) TestProposalSignatureError() {

	// change the verifier to error on signature validation with unspecific error
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.block.QC.SignerIDs, ps.block.QC.SigData, ps.parent).Return(true, nil)
	ps.verifier.On("VerifyVote", ps.vote.SignerID, ps.vote.SigData, ps.block).Return(true, errors.New("dummy error"))

	// check that validation now fails
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal should be rejected if signature check fails")

	// check that the error is not one that leads to invalid
	assert.False(ps.T(), model.IsInvalidBlockError(err), "if signature check fails, we should not receive an ErrorInvalidBlock")
}

func (ps *ProposalSuite) TestProposalSignatureInvalidFormat() {

	// change the verifier to fail signature validation with ErrInvalidFormat error
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.block.QC.SignerIDs, ps.block.QC.SigData, ps.parent).Return(true, nil)
	ps.verifier.On("VerifyVote", ps.vote.SignerID, ps.vote.SigData, ps.block).Return(true, fmt.Errorf("%w", verification.ErrInvalidFormat))

	// check that validation now fails
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid signature should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.True(ps.T(), model.IsInvalidBlockError(err), "if signature is invalid, we should generate an invalid error")
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
	assert.True(ps.T(), model.IsInvalidBlockError(err), "if signature is invalid, we should generate an invalid error")
}

func (ps *ProposalSuite) TestProposalWrongLeader() {

	// change the hotstuff.Committee to return a different leader
	*ps.committee = mocks.Committee{}
	ps.committee.On("LeaderForView", ps.block.View).Return(ps.participants[1].NodeID, nil)
	for _, participant := range ps.participants {
		ps.committee.On("Identity", mock.Anything, participant.NodeID).Return(participant, nil)
	}

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal from the wrong proposer should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.True(ps.T(), model.IsInvalidBlockError(err), "if the proposal has wrong proposer, we should generate a invalid error")
}

func (ps *ProposalSuite) TestProposalMismatchingView() {

	// change the QC's view to be different from the parent
	ps.proposal.Block.QC.View++

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with a mismatching QC view should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.True(ps.T(), model.IsInvalidBlockError(err), "if the QC has a mismatching view, we should generate a invalid error")
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
	assert.True(ps.T(), model.IsMissingBlockError(err), "if we don't have the proposal parent for a QC above or equal finalized view, we should generate an missing block error")
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
	assert.True(ps.T(), model.IsInvalidBlockError(err), "if the block's QC signature is invalid, an ErrorInvalidBlock error should be raised")
}

func (ps *ProposalSuite) TestProposalQCError() {

	// change verifier to fail on QC validation
	*ps.verifier = mocks.Verifier{}
	ps.verifier.On("VerifyQC", ps.block.QC.SignerIDs, ps.block.QC.SigData, ps.parent).Return(true, fmt.Errorf("Some error"))
	ps.verifier.On("VerifyVote", ps.vote.SignerID, ps.vote.SigData, ps.block).Return(true, nil)

	// check that validation fails now
	err := ps.validator.ValidateProposal(ps.proposal)
	assert.Error(ps.T(), err, "a proposal with an invalid QC should be rejected")

	// check that the error is an invalid proposal error to allow creating slashing challenge
	assert.False(ps.T(), model.IsInvalidBlockError(err), "if we can't verify the QC, we should not generate a invalid error")
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
	committee *mocks.Committee
	validator *Validator
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
	vs.committee = &mocks.Committee{}
	vs.committee.On("Identity", mock.Anything, vs.signer.NodeID).Return(vs.signer, nil)

	// set up the validator with the mocked dependencies
	vs.validator = New(vs.committee, vs.forks, vs.verifier)
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
	assert.True(vs.T(), model.IsInvalidVoteError(err), "a mismatching view should create a invalid vote error")
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
	participants flow.IdentityList
	signers      flow.IdentityList
	block        *model.Block
	qc           *flow.QuorumCertificate
	committee    *mocks.Committee
	verifier     *mocks.Verifier
	validator    *Validator
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
	qs.committee = &mocks.Committee{}
	qs.committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return qs.participants.Filter(selector)
		},
		nil,
	)

	// set up the mocked verifier to verify the QC correctly
	qs.verifier = &mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.qc.SignerIDs, qs.qc.SigData, qs.block).Return(true, nil)

	// set up the validator with the mocked dependencies
	qs.validator = New(qs.committee, nil, qs.verifier)
}

func (qs *QCSuite) TestQCOK() {

	// check the default happy case passes
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.NoError(qs.T(), err, "a valid QC should be accepted")
}

// TestQCInvalidSignersError tests that a qc fails validation if:
// QC signer's Identities cannot all be retrieved (some are not valid consensus participants)
func (qs *QCSuite) TestQCInvalidSignersError() {
	qs.participants = qs.participants[1:]           // remove participant[0] from the list of valid consensus participant
	err := qs.validator.ValidateQC(qs.qc, qs.block) // the QC should not be validated anymore
	assert.True(qs.T(), model.IsInvalidBlockError(err), "if some signers are invalid consensus participants, an ErrorInvalidBlock error should be raised")
}

// TestQCRetrievingParticipantsError tests that validation errors if:
// there is an error retrieving identities of consensus participants
func (qs *QCSuite) TestQCRetrievingParticipantsError() {
	// change the hotstuff.Committee to fail on retrieving participants
	*qs.committee = mocks.Committee{}
	qs.committee.On("Identities", mock.Anything, mock.Anything).Return(qs.participants, errors.New("FATAL internal error"))

	// verifier should escalate unspecific internal error to surrounding logic, but NOT as ErrorInvalidBlock
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.Error(qs.T(), err, "unspecific error when retrieving consensus participants should be escalated to surrounding logic")
	assert.False(qs.T(), model.IsInvalidBlockError(err), "unspecific internal errors should not result in ErrorInvalidBlock error")
}

// TestQCSignersError tests that a qc fails validation if:
// QC signer's have insufficient stake (but are all valid consensus participants otherwise)
func (qs *QCSuite) TestQCInsufficientStake() {
	// signers only have stake 6 out of 10 total (NOT have a supermajority)
	qs.signers = qs.participants[:6]
	qs.qc = helper.MakeQC(qs.T(), helper.WithQCBlock(qs.block), helper.WithQCSigners(qs.signers.NodeIDs()))

	// the QC should not be validated anymore
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.Error(qs.T(), err, "a QC should be rejected if it has insufficient voted stake")

	// we should get a threshold error to bubble up for extra info
	assert.True(qs.T(), model.IsInvalidBlockError(err), "if there is insufficient voted stake, an invalid block error should be raised")
}

// TestQCSignatureError tests that validation errors if:
// there is an unspecific internal error while validating the signature
func (qs *QCSuite) TestQCSignatureError() {

	// set up the verifier to fail QC verification
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.qc.SignerIDs, qs.qc.SigData, qs.block).Return(true, errors.New("dummy error"))

	// verifier should escalate unspecific internal error to surrounding logic, but NOT as ErrorInvalidBlock
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.Error(qs.T(), err, "unspecific sig verification error should be escalated to surrounding logic")
	assert.False(qs.T(), model.IsInvalidBlockError(err), "unspecific internal errors should not result in ErrorInvalidBlock error")
}

func (qs *QCSuite) TestQCSignatureInvalid() {

	// change the verifier to fail the QC signature
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.qc.SignerIDs, qs.qc.SigData, qs.block).Return(false, nil)

	// the QC should no longer be validation
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.True(qs.T(), model.IsInvalidBlockError(err), "if the signature is invalid an ErrorInvalidBlock error should be raised")
}

func (qs *QCSuite) TestQCSignatureInvalidFormat() {

	// change the verifier to fail the QC signature
	*qs.verifier = mocks.Verifier{}
	qs.verifier.On("VerifyQC", qs.qc.SignerIDs, qs.qc.SigData, qs.block).Return(true, fmt.Errorf("%w", verification.ErrInvalidFormat))

	// the QC should no longer be validation
	err := qs.validator.ValidateQC(qs.qc, qs.block)
	assert.True(qs.T(), model.IsInvalidBlockError(err), "if the signature has an invalid format, an ErrorInvalidBlock error should be raised")
}
