package sigvalidator

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// SingleSigValidator implements the SigValidator interface and knows how to
// decode the signers and signature fields. It can extract into general message and
// signatures from the vote/block for the underneath stateful verifier, in order to perform the cryptographic verification.
// It's used by collection cluster, where each signer only signs one staking signature with their staking key.
// Since threshold signatures are not used, the signature validation can be implemented in a stateless manner.
type SingleSigValidator struct {
	identity2SignerIndex map[flow.Identifier]int // lookup signer index by identity
}

func NewSingleSigValidator() *SingleSigValidator {
	return &SingleSigValidator{
		identity2SignerIndex: nil,
	}
}

func (v *SingleSigValidator) ValidateVote(vote *model.Vote) error {
	panic("TO IMPLEMENT")
}

func (v *SingleSigValidator) ValidateBlock(block *model.Proposal) error {
	panic("TO IMPLEMENT")
}
