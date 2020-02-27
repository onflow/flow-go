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
		forks: forks,
		sigVerifier: sigVerifier,
	}
}

// ValidateQC validates the QC
// qc - the qc to be validated
// block - the block that the qc is pointing to
func (v *Validator) ValidateQC(qc *hotstuff.QuorumCertificate, block *hotstuff.Block) error {
	// check block ID
	if qc.BlockID != block.BlockID {
		return newInvalidBlockError(block, fmt.Sprintf("qc.BlockID (%x) doesn't match with block's ID (%x)", qc.BlockID, block.BlockID))
	}

	// check view
	if qc.View != block.View {
		return newInvalidBlockError(block, fmt.Sprintf("qc.View (%d) doesn't match with block's View (%d)", qc.View, block.View))
	}

	// check if there is duplicates in QC.AggregatedSignature.Signers
	signerMap := make(map[flow.Identifier]struct{}, len(qc.AggregatedSignature.SignerIDs))
	for _, signerID := range qc.AggregatedSignature.SignerIDs {
		_, found := signerMap[signerID]
		if found {
			return newInvalidBlockError(block, "duplicated signers in QC")
		}
		signerMap[signerID] = struct{}{}
	}

	stakedSigners, err := v.viewState.ConsensusIdentities(qc.BlockID, qc.AggregatedSignature.SignerIDs...)
	if err != nil {
		return fmt.Errorf("cannot get signer identities at blockID (%x) to validate QC, %w", qc.BlockID, err)
	}

	// Given the fact we've checked there is no duplication in QC.AggregatedSignature.Signers,
	// in other words, signers are all unique, then:
	// if the counts are equal, then the aggregated signature contains all staked signers' signatures.
	// if the counts are not equal, then some signer must be unstaked or not among the consensus members
	if int(stakedSigners.Count()) != len(qc.AggregatedSignature.SignerIDs) {
		return newInvalidBlockError(block, fmt.Sprintf("QC has signatures from unstaked signers or non-consensus nodes"))
	}

	// get all staked nodes at qc's Block
	allStakedNodes, err := v.viewState.ConsensusIdentities(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot get identities at blockID (%x) to validate QC, %w", qc.BlockID, err)
	}

	// compute the threshold of stake required for a valid QC
	threshold := ComputeStakeThresholdForBuildingQC(allStakedNodes.TotalStake())

	// compute total stakes of all signers from the QC
	totalStakes := stakedSigners.TotalStake()

	// check if there are enough stake for building QC
	if totalStakes < threshold {
		return newInvalidBlockError(block, fmt.Sprintf("insufficient stake (required=%d, got=%d) before signatures are verified", threshold, totalStakes))
	}

	// convert to public keys
	stakingPubKeys := make([]crypto.PublicKey, 0, len(stakedSigners))
	for _, signer := range stakedSigners {
		stakingPubKeys = append(stakingPubKeys, signer.StakingPubKey)
	}

	// validate qc's aggregated staking signatures.
	valid, err := v.sigVerifier.VerifyAggregatedStakingSignature(qc.AggregatedSignature.StakingSignatures, block, stakingPubKeys)
	if err != nil {
		return fmt.Errorf("cannot verify qc's aggregated signature, qc.BlockID: %x", qc.BlockID)
	}

	if !valid {
		return newInvalidBlockError(block, "aggregated staking signature in QC is invalid")
	}

	// validate qc's reconstructed random beacon signatures.
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

	// get claimed signer
	signers, err := v.viewState.ConsensusIdentities(proposal.Block.BlockID, proposal.Block.ProposerID)
	if err != nil {
		return fmt.Errorf("cannot get signer for block: %x", blockID)
	}

	if len(signers) != 1 {
		return newInvalidBlockError(block, fmt.Sprintf("signer is not staked. signerID: %x", proposal.Block.ProposerID))
	}

	// the only signer
	signer := signers[0]

	// check the signer is the leader for that block
	leader := v.viewState.LeaderForView(proposal.Block.View)
	if leader.ID() != signer.ID() {
		return newInvalidBlockError(block, fmt.Sprintf("proposed by from wrong leader (%x), expected leader: (%x)", signer.ID(), leader.ID()))
	}

	// check staking signature
	valid, err := v.sigVerifier.VerifyStakingSig(proposal.StakingSignature, proposal.Block, signer.StakingPubKey)
	if err != nil {
		return fmt.Errorf("cannot verify block %x 's staking signature: %w", blockID, err)
	}

	if !valid {
		return newInvalidBlockError(block, "block proposer's staking signature is invalid")
	}

	// check random beacon signature
	valid, err = v.sigVerifier.VerifyRandomBeaconSig(proposal.RandomBeaconSignature, proposal.Block, signer.RandomBeaconPubKey)
	if err != nil {
		return fmt.Errorf("cannot verify block %x 's random beacon signature: %w", blockID, err)
	}

	if !valid {
		return newInvalidBlockError(block, "block proposer's random beacon signature is invalid")
	}

	// check view
	if proposal.Block.View <= qc.View {
		return newInvalidBlockError(block, fmt.Sprintf("block's view (%d) must be higher than QC's view (%d)", proposal.Block.View, qc.View))
	}

	// get parent block
	parent, found := v.forks.GetBlock(qc.BlockID)
	if !found {
		// parent is missing

		// if the parent block is equal or above the finalized view, then Forks should have it
		if qc.View >= v.forks.FinalizedView() {
			// if Forks doesn't have it, then it's an invalid block
			return newInvalidBlockError(block, fmt.Sprintf("qc is pointing to unknown block (%x)", qc.BlockID))
		}

		// It could be the case where Forks has pruned the parent block.
		// At this point, we can't verify the QC, because Forks doesn't the parent block, and we can't tell
		// for sure if the parent block exists. But we know for sure is even if it's invalid we won't vote
		// for it, because the QC.View is below finalized block. So, we could simply consider this block
		// as "valid", and we won't vote for it or build a block on top of it.

		// TODO: note other components will expect Validator has validated, and might re-validate it,
		// if the validation failed there, it will cause crash.
		// should we return a new error here to ignore?
		return hotstuff.ErrUnverifiableBlock
	}

	// validate QC - keep the most expensive the last to check
	return v.ValidateQC(qc, parent)
}

// ValidateVote validates the vote and returns the signer identity who signed the vote
// vote - the vote to be validated
// block - the voting block. Assuming the block has been validated.
func (v *Validator) ValidateVote(vote *hotstuff.Vote, block *hotstuff.Block) (*flow.Identity, error) {
	blockID := block.BlockID
	voteID := vote.ID()

	// view must match with the block's view
	if vote.View != block.View {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("wrong view number. expected (%d), got (%d)", block.View, vote.View))
	}

	// block hash must match
	if vote.BlockID != blockID {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("wrong block ID. expected (%x), got (%d)", blockID, vote.BlockID))
	}

	// get claimed signer
	signerID := vote.Signature.SignerID
	signers, err := v.viewState.ConsensusIdentities(blockID, signerID)
	if err != nil {
		return nil, fmt.Errorf("cannot get signer (%x) to validate vote (%x): %w", signerID, voteID, err)
	}

	if len(signers) != 1 {
		return nil, newInvalidVoteError(vote, fmt.Sprintf("signer (%x) is not staked", signerID))
	}

	// the only signer
	signer := signers[0]

	// check staking signature
	valid, err := v.sigVerifier.VerifyStakingSig(vote.Signature.StakingSignature, block, signer.StakingPubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot verify signature for vote (%x): %w", voteID, err)
	}

	if !valid {
		return nil, newInvalidVoteError(vote, "vote's staking signature is invalid")
	}

	// check random beacon signature
	valid, err = v.sigVerifier.VerifyRandomBeaconSig(vote.Signature.RandomBeaconSignature, block, signer.RandomBeaconPubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot verify vote %x 's random beacon signature: %w", blockID, err)
	}

	if !valid {
		return nil, newInvalidVoteError(vote, "vote's random beacon signature is invalid")
	}

	return signer, nil
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
