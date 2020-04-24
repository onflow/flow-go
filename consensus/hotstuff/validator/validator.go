package validator

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

// Validator is responsible for validating QC, Block and Vote
type Validator struct {
	membersState hotstuff.MembersState
	forks        hotstuff.ForksReader
	verifier     hotstuff.Verifier
}

// New creates a new Validator instance
func New(membersState hotstuff.MembersState, forks hotstuff.ForksReader, verifier hotstuff.Verifier) *Validator {
	return &Validator{
		membersState: membersState,
		forks:        forks,
		verifier:     verifier,
	}
}

// ValidateQC validates the QC
// qc - the qc to be validated
// block - the block that the qc is pointing to
func (v *Validator) ValidateQC(qc *model.QuorumCertificate, block *model.Block) error {
	if qc.BlockID != block.BlockID { // check block ID
		return newInvalidBlockError(block, fmt.Sprintf("qc.BlockID (%s) doesn't match block's ID (%s)", qc.BlockID, block.BlockID))
	}
	if qc.View != block.View { // check view
		return newInvalidBlockError(block, fmt.Sprintf("qc.View (%d) doesn't match block's View (%d)", qc.View, block.View))
	}

	// Retrieve full Identities of all legitimate consensus participants and the Identities of the qc's signers
	// IdentityList returned by MembersState contains only legitimate consensus participants for the specified block (must have positive stake)
	allConsensusParticipants, err := v.membersState.AtBlockID(block.BlockID).Identities(filter.Any)
	if err != nil {
		return fmt.Errorf("could not get consensus participants for blcok %s: %w", block.BlockID, err)
	}
	signers := allConsensusParticipants.Filter(filter.HasNodeID(qc.SignerIDs...)) // resulting IdentityList contains no duplicates
	if len(signers) < len(qc.SignerIDs) {
		return newInvalidBlockError(block, fmt.Sprintf("some signers are not valid consensus participants at block %x: %w", block.BlockID, model.ErrInvalidConsensusParticipant))
	}

	// determine whether signers reach minimally required stake threshold for consensus
	threshold := hotstuff.ComputeStakeThresholdForBuildingQC(allConsensusParticipants.TotalStake()) // compute required stake threshold
	if signers.TotalStake() < threshold {
		return newInvalidBlockError(block, fmt.Sprintf("qc signers have insufficient stake of %d (required=%d)", threshold, signers.TotalStake()))
	}

	// verify whether the signature bytes are valid for the QC in the context of the protocol state
	valid, err := v.verifier.VerifyQC(qc.SignerIDs, qc.SigData, block)
	if errors.Is(err, verification.ErrInvalidFormat) {
		return newInvalidBlockError(block, fmt.Sprintf("QC signature has bad format (%s)", err))
	}
	if err != nil {
		return fmt.Errorf("cannot verify qc's aggregated signature, qc.BlockID: %s", qc.BlockID)
	}
	if !valid {
		return newInvalidBlockError(block, "QC signature is invalid")
	}

	return nil
}

// ValidateProposal validates the block proposal
// A block is considered as valid if it's a valid extension of existing forks.
// Note it doesn't check if it's conflicting with finalized block
func (v *Validator) ValidateProposal(proposal *model.Proposal) error {
	qc := proposal.Block.QC
	block := proposal.Block
	blockID := proposal.Block.BlockID

	// validate the proposer's vote and get his identity
	_, err := v.ValidateVote(proposal.ProposerVote(), proposal.Block)
	if err != nil {
		return newInvalidBlockError(block, fmt.Sprintf("invalid proposer for block %x: %s", blockID, err.Error()))
	}

	// check the proposer is the leader for the proposed block's view
	leader, err := v.membersState.LeaderForView(proposal.Block.View)
	if err != nil {
		return fmt.Errorf("error determining primary for block %x: %w", block.BlockID, err)
	}
	if leader != proposal.Block.ProposerID {
		return newInvalidBlockError(block, fmt.Sprintf("proposer %s is not primary for view %d", proposal.Block.ProposerID, proposal.Block.View))
	}

	// check that we have the parent for the proposal
	parent, found := v.forks.GetBlock(qc.BlockID)
	if !found {
		// Forks is _allowed_ to (but obliged to) prune blocks whose view is below the newest finalized block.
		if qc.View >= v.forks.FinalizedView() {
			// If the parent block is equal or above the finalized view, then Forks should have it. Otherwise, we are missing a block!
			return &model.ErrorMissingBlock{View: qc.View, BlockID: qc.BlockID}
		}

		// Forks has already pruned the parent block. I.e., we can't validate that the qc matches
		// a known (and valid) parent. Nevertheless, we just store this block, because there might already
		// exists children of this block, which we could receive later. However, we know for sure that the
		// block's fork cannot keep growing anymore because it conflicts with a finalized block.
		// TODO: note other components will expect Validator has validated, and might re-validate it,
		return model.ErrUnverifiableBlock
	}

	// validate QC - keep the most expensive the last to check
	return v.ValidateQC(qc, parent)
}

// ValidateVote validates the vote and returns the identity of the voter who signed
// vote - the vote to be validated
// block - the voting block. Assuming the block has been validated.
func (v *Validator) ValidateVote(vote *model.Vote, block *model.Block) (*flow.Identity, error) {
	// view must match with the block's view
	if vote.View != block.View {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("wrong view number. expected (%d), got (%d)", block.View, vote.View))
	}
	// block hash must match
	if vote.BlockID != block.BlockID {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("wrong block ID. expected (%s), got (%d)", block.BlockID, vote.BlockID))
	}

	// TODO: this lookup is duplicated in Verifier
	voter, err := v.membersState.AtBlockID(block.BlockID).Identity(vote.SignerID)
	if errors.Is(err, model.ErrInvalidConsensusParticipant) {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("vote is not from a staked consensus participant: %s", err.Error()))
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants for block %x: %w", block.BlockID, err)
	}

	// check whether the signature data is valid for the vote in the hotstuff context
	valid, err := v.verifier.VerifyVote(vote.SignerID, vote.SigData, block)
	if err != nil {
		switch {
		case errors.Is(err, verification.ErrInvalidFormat):
			return nil, newInvalidVoteError(vote, fmt.Sprintf("vote signature has bad format: %s", err.Error()))
		case errors.Is(err, model.ErrInvalidConsensusParticipant):
			return nil, newInvalidVoteError(vote, fmt.Sprintf("vote is not from a staked consensus participant: %s", err.Error()))
		default:
			return nil, fmt.Errorf("cannot verify signature for vote (%s): %w", vote.ID(), err.Error())
		}
	}
	if !valid {
		return nil, newInvalidVoteError(vote, "vote signature is invalid")
	}

	return voter, nil
}

func newInvalidBlockError(block *model.Block, msg string) error {
	return &model.ErrorInvalidBlock{
		BlockID: block.BlockID,
		View:    block.View,
		Msg:     msg,
	}
}

func newInvalidVoteError(vote *model.Vote, msg string) error {
	return &model.ErrorInvalidVote{
		VoteID: vote.ID(),
		View:   vote.View,
		Msg:    msg,
	}
}
