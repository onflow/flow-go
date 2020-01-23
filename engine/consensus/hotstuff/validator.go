package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Validator struct {
	viewState ViewState
}

func (v *Validator) ValidateQC(qc *types.QuorumCertificate) bool {
	panic("TODO")
}

func (v *Validator) ValidateBlock(bp *types.BlockProposal) bool {
	panic("TODO")
}

func (v *Validator) ValidateIncorporatedVote(vote *types.Vote, bp *types.BlockProposal, identities flow.IdentityList) error {
	err := v.checkVoteSig(vote)
	if err != nil {
		return fmt.Errorf("could not validate the signature: %w", err)
	}

	if vote.View != bp.View() {
		return fmt.Errorf("could not validate the view: %w", types.ErrInvalidView{vote})
	}

	return nil
}

func (v *Validator) ValidatePendingVote(vote *types.Vote) error {
	err := v.checkVoteSig(vote)
	if err != nil {
		return fmt.Errorf("could not validate the signature: %w", err)
	}

	return nil
}

func (v *Validator) checkVoteSig(vote *types.Vote) error {
	return nil
}
