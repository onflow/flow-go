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
	committee    *mocks.VoterCommittee
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

	// set up the mocked hotstuff Committee state
	ps.committee = &mocks.VoterCommittee{}
	ps.committee.On("LeaderForView", ps.block.View).Return(ps.leader.NodeID, nil)
	ps.committee.On("IdentitiesByEpoch", mock.Anything, mock.Anything).Return(
		func(view uint64, selector flow.IdentityFilter) flow.IdentityList {
			return ps.participants.Filter(selector)
		},
		nil,
	)
	for _, participant := range ps.participants {
		ps.committee.On("IdentityByEpoch", mock.Anything, participant.NodeID).Return(participant, nil)
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

	// change the hotstuff.Committee to return a different leader
	*ps.committee = mocks.VoterCommittee{}
	ps.committee.On("LeaderForView", ps.block.View).Return(ps.participants[1].NodeID, nil)
	for _, participant := range ps.participants {
		ps.committee.On("IdentityByEpoch", mock.Anything, participant.NodeID).Return(participant, nil)
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
	vs.committee = &mocks.Committee{}
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
// In this case, the `hotstuff.Committee` returns a `model.InvalidSignerError`,
// which the Validator should recognize as a symptom for an invalid vote.
// Hence, we expect the validator to return a `model.InvalidVoteError`.
func (vs *VoteSuite) TestVoteInvalidSignerID() {
	*vs.committee = mocks.Committee{}
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
	committee    *mocks.Committee
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
	qs.committee = &mocks.Committee{}
	qs.committee.On("IdentitiesByEpoch", mock.Anything, mock.Anything).Return(
		func(view uint64, selector flow.IdentityFilter) flow.IdentityList {
			return qs.participants.Filter(selector)
		},
		nil,
	)

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
	// change the hotstuff.Committee to fail on retrieving participants
	*qs.committee = mocks.Committee{}
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
	committee    *mocks.Committee
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
	s.committee = &mocks.Committee{}
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

// TestTCInvalidSigners tests if correct error is returned when signers are invalid
func (s *TCSuite) TestTCInvalidSigners() {
	s.participants = s.participants[1:] // remove participant[0] from the list of valid consensus participant
	err := s.validator.ValidateTC(s.tc) // the QC should not be validated anymore
	assert.True(s.T(), model.IsInvalidTCError(err), "if some signers are invalid consensus participants, an ErrorInvalidTC error should be raised")
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
