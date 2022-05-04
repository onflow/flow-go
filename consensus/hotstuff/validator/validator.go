package validator

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// Validator is responsible for validating QC, Block and Vote
type Validator struct {
	committee hotstuff.VoterCommittee
	forks     hotstuff.ForksReader
	verifier  hotstuff.Verifier
}

var _ hotstuff.Validator = (*Validator)(nil)

// New creates a new Validator instance
func New(
	committee hotstuff.VoterCommittee,
	forks hotstuff.ForksReader,
	verifier hotstuff.Verifier,
) *Validator {
	return &Validator{
		committee: committee,
		forks:     forks,
		verifier:  verifier,
	}
}

// ValidateTC validates the TC
// tc - the tc to be validated
// During normal operations, the following error returns are expected:
//  * model.InvalidTCError if the TC is invalid
// Any other error should be threated as exception
func (v *Validator) ValidateTC(tc *flow.TimeoutCertificate) error {
	highestQC := tc.TOHighestQC

	// consistency checks
	if len(tc.TOHighQCViews) != len(tc.SignerIDs) {
		return newInvalidTCError(tc, fmt.Errorf("invalid TC structure expected %x messages, got %x", len(tc.SignerIDs), len(tc.TOHighQCViews)))
	}

	if tc.View < highestQC.View {
		return newInvalidTCError(tc, fmt.Errorf("TC's QC cannot be newer than the TC's view"))
	}

	highestQCView := tc.TOHighQCViews[0]
	for _, view := range tc.TOHighQCViews {
		if highestQCView < view {
			highestQCView = view
		}
	}

	if highestQCView != tc.TOHighestQC.View {
		return newInvalidTCError(tc, fmt.Errorf("included QC should be equal to highest contributed view"))
	}

	// 1. Check if there is super-majority of votes
	allParticipants, err := v.committee.IdentitiesByEpoch(tc.View, filter.Any)
	if err != nil {
		return fmt.Errorf("could not get consensus participants at view %d: %w", tc.View, err)
	}

	signers := allParticipants.Filter(filter.HasNodeID(tc.SignerIDs...)) // resulting IdentityList contains no duplicates
	if len(signers) != len(tc.SignerIDs) {
		return newInvalidTCError(tc, model.NewInvalidSignerErrorf("some tc signers are duplicated or invalid consensus participants at view %x", tc.View))
	}

	// determine whether signers reach minimally required weight threshold for consensus
	threshold := hotstuff.ComputeWeightThresholdForBuildingQC(allParticipants.TotalWeight()) // compute required weight threshold
	if signers.TotalWeight() < threshold {
		return newInvalidTCError(tc, fmt.Errorf("tc signers have insufficient weight of %d (required=%d)", signers.TotalWeight(), threshold))
	}

	// Validate QC
	err = v.ValidateQC(highestQC)
	if err != nil {
		return newInvalidTCError(tc, fmt.Errorf("invalid QC included in TC: %w", err))
	}

	// Verify multi-message BLS sig of TC, by far the most expensive check
	err = v.verifier.VerifyTC(signers, tc.SigData, tc.View, tc.TOHighQCViews)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidFormat):
			return newInvalidTCError(tc, fmt.Errorf("TC's signature data has an invalid structure: %w", err))
		case errors.Is(err, model.ErrInvalidSignature):
			return newInvalidTCError(tc, fmt.Errorf("TC contains invalid signature(s): %w", err))
		default:
			return fmt.Errorf("cannot verify tc's aggregated signature (tc.View: %d): %w", tc.View, err)
		}
	}
	return nil
}

// ValidateQC validates the QC
// qc - the qc to be validated
func (v *Validator) ValidateQC(qc *flow.QuorumCertificate) error {
	// Retrieve full Identities of all legitimate consensus participants and the Identities of the qc's signers
	// IdentityList returned by hotstuff.VoterCommittee contains only legitimate consensus participants for the specified view (must have positive weight)
	allParticipants, err := v.committee.IdentitiesByEpoch(qc.View, filter.Any)
	if err != nil {
		return fmt.Errorf("could not get consensus participants at view %d: %w", qc.View, err)
	}
	signers := allParticipants.Filter(filter.HasNodeID(qc.SignerIDs...)) // resulting IdentityList contains no duplicates
	if len(signers) != len(qc.SignerIDs) {
		return newInvalidQCError(qc, model.NewInvalidSignerErrorf("some qc signers are duplicated or invalid consensus participants at view %x", qc.View))
	}

	// determine whether signers reach minimally required weight threshold for consensus
	threshold := hotstuff.ComputeWeightThresholdForBuildingQC(allParticipants.TotalWeight()) // compute required weight threshold
	if signers.TotalWeight() < threshold {
		return newInvalidQCError(qc, fmt.Errorf("qc signers have insufficient weight of %d (required=%d)", signers.TotalWeight(), threshold))
	}

	// verify whether the signature bytes are valid for the QC in the context of the protocol state
	err = v.verifier.VerifyQC(signers, qc.SigData, qc.View, qc.BlockID)
	if err != nil {
		// Theoretically, `VerifyQC` could also return a `model.InvalidSignerError`. However,
		// for the time being, we assume that _every_ HotStuff participant is also a member of
		// the random beacon committee. Consequently, `InvalidSignerError` should not occur atm.
		// TODO: if the random beacon committee is a strict subset of the HotStuff committee,
		//       we expect `model.InvalidSignerError` here during normal operations.
		switch {
		case errors.Is(err, model.ErrInvalidFormat):
			return newInvalidQCError(qc, fmt.Errorf("QC's  signature data has an invalid structure: %w", err))
		case errors.Is(err, model.ErrInvalidSignature):
			return newInvalidQCError(qc, fmt.Errorf("QC contains invalid signature(s): %w", err))
		default:
			return fmt.Errorf("cannot verify qc's aggregated signature (qc.BlockID: %x): %w", qc.BlockID, err)
		}
	}

	return nil
}

// ValidateProposal validates the block proposal
// A block is considered as valid if it's a valid extension of existing forks.
// Note it doesn't check if it's conflicting with finalized block
func (v *Validator) ValidateProposal(proposal *model.Proposal) error {
	qc := proposal.Block.QC
	block := proposal.Block

	// validate the proposer's vote and get his identity
	_, err := v.ValidateVote(proposal.ProposerVote())
	if model.IsInvalidVoteError(err) {
		return newInvalidBlockError(block, fmt.Errorf("invalid proposer signature: %w", err))
	}
	if err != nil {
		return fmt.Errorf("error verifying leader signature for block %x: %w", block.BlockID, err)
	}

	// check the proposer is the leader for the proposed block's view
	leader, err := v.committee.LeaderForView(block.View)
	if err != nil {
		return fmt.Errorf("error determining leader for block %x: %w", block.BlockID, err)
	}
	if leader != block.ProposerID {
		return newInvalidBlockError(block, fmt.Errorf("proposer %s is not leader (%s) for view %d", block.ProposerID, leader, block.View))
	}

	// validate QC - keep the most expensive the last to check
	err = v.ValidateQC(qc)
	if err != nil {
		if model.IsInvalidQCError(err) {
			return newInvalidBlockError(block, fmt.Errorf("invalid qc included: %w", err))
		}
		return fmt.Errorf("unexpected error verifying qc: %w", err)
	}

	// The Block must contain a proof that the primary legitimately entered the respective view.
	// Transitioning to proposal.Block.View is possible either by observing a QC or a TC for the
	// previous round. If and only if the QC is _not_ for the previous round we require a TC for
	// the previous view to be present.
	lastViewSuccessful := proposal.Block.View == proposal.Block.QC.View+1
	if !lastViewSuccessful {
		// check if proposal is correctly structured
		if proposal.LastViewTC == nil {
			return newInvalidBlockError(block, fmt.Errorf("last view has ended with timeout but proposal doesn't include LastViewTC"))
		}

		if proposal.Block.View != proposal.LastViewTC.View+1 {
			return newInvalidBlockError(block, fmt.Errorf("TC missing"))
		}

		// check if proposal extends the highest QC from TC.
		if proposal.Block.QC.View < proposal.LastViewTC.TOHighestQC.View {
			return newInvalidBlockError(block, fmt.Errorf("proposal's QC is lower than locked QC"))
		}

		// check if included TC is valid
		err = v.ValidateTC(proposal.LastViewTC)
		if err != nil {
			return newInvalidBlockError(block, fmt.Errorf(""))
		}
	}

	return nil
}

// ValidateVote validates the vote and returns the identity of the voter who signed
// vote - the vote to be validated
func (v *Validator) ValidateVote(vote *model.Vote) (*flow.Identity, error) {
	voter, err := v.committee.IdentityByEpoch(vote.View, vote.SignerID)
	if model.IsInvalidSignerError(err) {
		return nil, newInvalidVoteError(vote, err)
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving voter Identity at view %x: %w", vote.View, err)
	}

	// check whether the signature data is valid for the vote in the hotstuff context
	err = v.verifier.VerifyVote(voter, vote.SigData, vote.View, vote.BlockID)
	if err != nil {
		// Theoretically, `VerifyVote` could also return a `model.InvalidSignerError`. However,
		// for the time being, we assume that _every_ HotStuff participant is also a member of
		// the random beacon committee. Consequently, `InvalidSignerError` should not occur atm.
		// TODO: if the random beacon committee is a strict subset of the HotStuff committee,
		//       we expect `model.InvalidSignerError` here during normal operations.
		if errors.Is(err, model.ErrInvalidFormat) || errors.Is(err, model.ErrInvalidSignature) {
			return nil, newInvalidVoteError(vote, err)
		}
		return nil, fmt.Errorf("cannot verify signature for vote (%x): %w", vote.ID(), err)
	}

	return voter, nil
}

func newInvalidBlockError(block *model.Block, err error) error {
	return model.InvalidBlockError{
		BlockID: block.BlockID,
		View:    block.View,
		Err:     err,
	}
}

func newInvalidQCError(qc *flow.QuorumCertificate, err error) error {
	return model.InvalidQCError{
		BlockID: qc.BlockID,
		View:    qc.View,
		Err:     err,
	}
}

func newInvalidTCError(tc *flow.TimeoutCertificate, err error) error {
	return model.InvalidTCError{
		View: tc.View,
		Err:  err,
	}
}

func newInvalidVoteError(vote *model.Vote, err error) error {
	return model.InvalidVoteError{
		VoteID: vote.ID(),
		View:   vote.View,
		Err:    err,
	}
}
