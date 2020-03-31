package validator

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Validator is responsible for validating QC, Block and Vote
type Validator struct {
	viewState hotstuff.ViewState
	forks     hotstuff.ForksReader
	verifier  hotstuff.Verifier
}

// New creates a new Validator instance
func New(viewState hotstuff.ViewState, forks hotstuff.ForksReader, verifier hotstuff.Verifier) *Validator {
	return &Validator{
		viewState: viewState,
		forks:     forks,
		verifier:  verifier,
	}
}

// ValidateQC validates the QC
// qc - the qc to be validated
// block - the block that the qc is pointing to
func (v *Validator) ValidateQC(qc *model.QuorumCertificate, block *model.Block) error {
	if qc.BlockID != block.BlockID { // check block ID
		return newInvalidBlockError(block, fmt.Sprintf("qc.BlockID (%s) doesn't match with block's ID (%s)", qc.BlockID, block.BlockID))
	}
	if qc.View != block.View { // check view
		return newInvalidBlockError(block, fmt.Sprintf("qc.View (%d) doesn't match with block's View (%d)", qc.View, block.View))
	}

	stakedSigners, err := v.viewState.IdentitiesForConsensusParticipants(qc.BlockID, qc.SignerIDs)
	if err != nil {
		return fmt.Errorf("invalid verifier identities in qc of blockID %s: %w", qc.BlockID, err)
	}
	// method IdentitiesForConsensusParticipants guarantees that there are no duplicated signers
	// and all signers are valid, staked consensus nodes

	// determine whether stakedSigners reach minimally required stake threshold for consensus:
	allConsensusParticipants, err := v.viewState.AllConsensusParticipants(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot get identities at blockID (%s) to validate QC, %w", qc.BlockID, err)
	}
	threshold := hotstuff.ComputeStakeThresholdForBuildingQC(allConsensusParticipants.TotalStake()) // compute required stake threshold
	totalStakes := stakedSigners.TotalStake()                                                       // total stakes of all signers
	if totalStakes < threshold {                                                                    // if stake is below minimally required threshold: qc is invalid
		return newInvalidBlockError(block, fmt.Sprintf("insufficient stake (required=%d, got=%d) before signatures are verified", threshold, totalStakes))
	}

	// validate signature:
	// validate qc's aggregated staking signature using the internal protocol state
	valid, err := v.verifier.VerifyQC(qc)
	if err != nil {
		return fmt.Errorf("cannot verify qc's aggregated signature, qc.BlockID: %s", qc.BlockID)
	}
	if !valid {
		return newInvalidBlockError(block, "aggregated staking signature in QC is invalid")
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

	// Validate Proposer's signatures and ensure that proposer is leader for respective view
	verifier, err := v.ValidateVote(proposal.ProposerVote(), proposal.Block)
	if err != nil {
		return newInvalidBlockError(block, fmt.Sprintf("invalid proposer for block %s: %s", blockID, err.Error()))
	}
	// check the verifier is the leader for that block
	leader := v.viewState.LeaderForView(proposal.Block.View)
	if leader.ID() != verifier.ID() {
		return newInvalidBlockError(block, fmt.Sprintf("proposed by from wrong leader (%s), expected leader: (%s)", verifier.ID(), leader.ID()))
	}

	// check proposal's parent
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

// ValidateVote validates the vote and returns the verifier identity who signed the vote
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

	// get voter's Identity (will error if voter is not a staked consensus participant at block)
	voter, err := v.viewState.IdentityForConsensusParticipant(block.BlockID, vote.SignerID)
	if err != nil {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("invalid voter identity: %s", err.Error()))
	}

	// check staking signature
	valid, err := v.verifier.VerifyVote(vote)
	if err != nil {
		return nil, fmt.Errorf("cannot verify signature for vote (%s): %w", vote.ID(), err)
	}
	if !valid {
		return nil, newInvalidVoteError(vote, "invalid staking signature")
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
