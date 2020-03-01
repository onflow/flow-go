package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// Validator is responsible for validating QC, Block and Vote
type Validator struct {
	viewState   *ViewState
	forks       ForksReader
	sigVerifier SigVerifier
}

// NewValidator creates a new Validator instance
func NewValidator(viewState *ViewState, forks ForksReader, sigVerifier SigVerifier) *Validator {
	return &Validator{
		viewState:   viewState,
		forks:       forks,
		sigVerifier: sigVerifier,
	}
}

// ValidateQC validates the QC
// qc - the qc to be validated
// block - the block that the qc is pointing to
func (v *Validator) ValidateQC(qc *hotstuff.QuorumCertificate, block *hotstuff.Block) error {
	if qc.BlockID != block.BlockID { // check block ID
		return newInvalidBlockError(block, fmt.Sprintf("qc.BlockID (%s) doesn't match with block's ID (%s)", qc.BlockID, block.BlockID))
	}
	if qc.View != block.View { // check view
		return newInvalidBlockError(block, fmt.Sprintf("qc.View (%d) doesn't match with block's View (%d)", qc.View, block.View))
	}

	stakedSigners, err := v.viewState.IdentitiesForConsensusParticipants(qc.BlockID, qc.AggregatedSignature.SignerIDs...)
	if err != nil {
		return fmt.Errorf("invalid signer identities in qc of blockID %s: %w", qc.BlockID, err)
	}
	// method IdentitiesForConsensusParticipants guarantees that there are no duplicated signers
	// and all signers are valid, staked consensus nodes

	// determine whether stakedSigners reach minimally required stake threshold for consensus:
	allConsensusParticipants, err := v.viewState.AllConsensusParticipants(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot get identities at blockID (%s) to validate QC, %w", qc.BlockID, err)
	}
	threshold := ComputeStakeThresholdForBuildingQC(allConsensusParticipants.TotalStake()) // compute required stake threshold
	totalStakes := stakedSigners.TotalStake()                                              // total stakes of all signers
	if totalStakes < threshold {                                                           // if stake is below minimally required threshold: qc is invalid
		return newInvalidBlockError(block, fmt.Sprintf("insufficient stake (required=%d, got=%d) before signatures are verified", threshold, totalStakes))
	}

	// validate signature:
	// collect staking keys for for nodes that contributed to qc's aggregated sig:
	stakingPubKeys := make([]crypto.PublicKey, 0, len(stakedSigners))
	for _, signer := range stakedSigners {
		stakingPubKeys = append(stakingPubKeys, signer.StakingPubKey)
	}
	// validate qc's aggregated staking signature using staking keys
	valid, err := v.sigVerifier.VerifyAggregatedStakingSignature(qc.AggregatedSignature.StakingSignatures, block, stakingPubKeys)
	if err != nil {
		return fmt.Errorf("cannot verify qc's aggregated signature, qc.BlockID: %s", qc.BlockID)
	}
	if !valid {
		return newInvalidBlockError(block, "aggregated staking signature in QC is invalid")
	}

	// validate qc's random beacon signature (reconstructed threshold signature)
	valid, err = v.sigVerifier.VerifyAggregatedRandomBeaconSignature(qc.AggregatedSignature.RandomBeaconSignature, block)
	if err != nil {
		return fmt.Errorf("cannot verify reconstructed random beacon sig from qc: %w", err)
	}
	if !valid {
		return newInvalidBlockError(block, "reconstructed random beacon signature in QC is invalid")
	}

	return nil
}

// ValidateProposal validates the block proposal
// A block is considered as valid if it's a valid extension of existing forks.
// Note it doesn't check if it's conflicting with finalized block
func (v *Validator) ValidateProposal(proposal *hotstuff.Proposal) error {
	qc := proposal.Block.QC
	block := proposal.Block
	blockID := proposal.Block.BlockID

	// get signer's Identity (will error if signer is not a staked consensus participant at block)
	signer, err := v.viewState.IdentityForConsensusParticipant(proposal.Block.BlockID, proposal.Block.ProposerID)
	if err != nil {
		return newInvalidBlockError(block, fmt.Sprintf("invalid proposed identity for block %s: %w", blockID, err))
	}
	// check the signer is the leader for that block
	leader := v.viewState.LeaderForView(proposal.Block.View)
	if leader.ID() != signer.ID() {
		return newInvalidBlockError(block, fmt.Sprintf("proposed by from wrong leader (%s), expected leader: (%s)", signer.ID(), leader.ID()))
	}

	// check signer's staking signature
	valid, err := v.sigVerifier.VerifyStakingSig(proposal.StakingSignature, proposal.Block, signer.StakingPubKey)
	if err != nil {
		return fmt.Errorf("cannot verify block %s 's staking signature: %w", blockID, err)
	}
	if !valid {
		return newInvalidBlockError(block, "block proposer's staking signature is invalid")
	}

	// check signer's random beacon signature
	valid, err = v.sigVerifier.VerifyRandomBeaconSig(proposal.RandomBeaconSignature, proposal.Block, signer.RandomBeaconPubKey)
	if err != nil {
		return fmt.Errorf("cannot verify block %s 's random beacon signature: %w", blockID, err)
	}
	if !valid {
		return newInvalidBlockError(block, "block proposer's random beacon signature is invalid")
	}

	// check parent
	if proposal.Block.View <= qc.View { // block's view must be larger than block.qc's view
		return newInvalidBlockError(block, fmt.Sprintf("block's view (%d) must be higher than QC's view (%d)", proposal.Block.View, qc.View))
	}
	if qc.View < v.forks.FinalizedView() {
		// It Forks has already pruned the parent block. I.e., we can't validate that the qc matches
		// a known (and valid) parent. Nevertheless, we just store this block, because there migth already exists
		// children of this block, we will receive later. However, we know for sure that the block's fork
		// cannot keep growing anymore because it conflicts with a finalized block.

		// TODO: note other components will expect Validator has validated, and might re-validate it,
		return hotstuff.ErrUnverifiableBlock
	}

	// if the parent block is equal or above the finalized view, then Forks should have it
	parent, found := v.forks.GetBlock(qc.BlockID) // get parent block
	if !found {                                   // parent is missing
		return &hotstuff.ErrorMissingBlock{qc.View, qc.BlockID}
	}
	return v.ValidateQC(qc, parent) // validate QC; very expensive crypto check; kept to the last
}

// ValidateVote validates the vote and returns the signer identity who signed the vote
// vote - the vote to be validated
// block - the voting block. Assuming the block has been validated.
func (v *Validator) ValidateVote(vote *hotstuff.Vote, block *hotstuff.Block) (*flow.Identity, error) {
	// view must match with the block's view
	if vote.View != block.View {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("wrong view number. expected (%d), got (%d)", block.View, vote.View))
	}
	// block hash must match
	if vote.BlockID != block.BlockID {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("wrong block ID. expected (%s), got (%d)", block.BlockID, vote.BlockID))
	}

	// get voter's Identity (will error if voter is not a staked consensus participant at block)
	voter, err := v.viewState.IdentityForConsensusParticipant(block.BlockID, vote.Signature.SignerID)
	if err != nil {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("invalid voter identity: %s", err.Error()))
	}

	// check staking signature
	valid, err := v.sigVerifier.VerifyStakingSig(vote.Signature.StakingSignature, block, voter.StakingPubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot verify signature for vote (%s): %w", vote.ID(), err)
	}
	if !valid {
		return nil, newInvalidVoteError(vote, "invalid staking signature")
	}

	// check random beacon signature
	valid, err = v.sigVerifier.VerifyRandomBeaconSig(vote.Signature.RandomBeaconSignature, block, voter.RandomBeaconPubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot verify vote %s 's random beacon signature: %w", vote.ID(), err)
	}
	if !valid {
		return nil, newInvalidVoteError(vote, "invalid random beacon signature")
	}

	return voter, nil
}

func newInvalidBlockError(block *hotstuff.Block, msg string) error {
	return &hotstuff.ErrorInvalidBlock{
		BlockID: block.BlockID,
		View:    block.View,
		Msg:     msg,
	}
}

func newInvalidVoteError(vote *hotstuff.Vote, msg string) error {
	return &hotstuff.ErrorInvalidVote{
		VoteID: vote.ID(),
		View:   vote.View,
		Msg:    msg,
	}
}
