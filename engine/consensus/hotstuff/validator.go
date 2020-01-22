package hotstuff

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Validator struct {
	viewState ViewState
}

// QC is valid (signature is valid and has enough stake)
func (v *Validator) ValidateQC(qc *types.QuorumCertificate) error {
	panic("TODO")
}

// ValidateBlock validates the block proposal
// bp is the block proposal to be validated.
// parent is the parent of the block proposal.
func (v *Validator) ValidateBlock(bp *types.BlockProposal, parent *types.BlockProposal) error {
	// validate siangure
	err := v.validateBlockSignature(bp)
	if err != nil {
		return err
	}

	qc := bp.QC()

	// validate QC
	err = v.ValidateQC(qc)
	if err != nil {
		return err
	}

	// validate block hash
	if !bytes.Equal(qc.BlockMRH, parent.BlockMRH()) {
		return fmt.Errorf("invalid block. block must link to its parent. (qc, parent): (%v, %v)", qc.BlockMRH, parent.BlockMRH())
	}

	// validate height
	if bp.Height() != parent.Height()+1 {
		return fmt.Errorf("invalid block. block height must be 1 block higher than its parent. (block, parent): (%v, %v)", bp.Height(), parent.Height())
	}

	// validate view
	if bp.View() <= qc.View {
		return fmt.Errorf("invalid block. block's view must be higher than QC's view. (block, qc): (%v, %v)", bp.View(), qc.View)
	}
	return nil
}

func (v *Validator) ValidateVote(vote *types.Vote) error {
	panic("TODO")
}

func (v *Validator) validateBlockSignature(bp *types.BlockProposal) error {
	// getting all consensus identities
	identities, err := v.viewState.GetIdentitiesForView(bp.View())
	if err != nil {
		return fmt.Errorf("cannot get identities to validate sig on block:(view, blockMRH):(%v, %v), %w", bp.View(), bp.BlockMRH(), err)
	}

	signerPubKey, err := findSignerPubKeyByIndex(identities, bp.Signature.SignerIdx)
	if err != nil {
		return fmt.Errorf("can not find signer for block: %v, because %w", bp.BlockMRH(), err)
	}

	err := verifySignature(bp.Signature.RawSignature, signerPubKey)
	return fmt.Errorf("invalid sig for block: %v, because %w", bp.BlockMRH(), err)
}

func findSignerPubKeyByIndex(identities flow.IdentityList, idx uint32) ([]byte, error) {
	// signer must exist
	if idx > identities.Count() {
		return nil, fmt.Errorf("signer not found by signerIdx: %v", idx)
	}

	signer := identities.Get(idx)
	panic("TODO")
}

// verify signature with the pub key
func verifySignature(signature [32]byte, pubkey []byte) error {
	panic("TODO")
}
