package sigvalidator

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// SingleSigValidator implements the SigValidator interface and owns the knowledge of how to
// decode the signer and signature fields. It converts the vote/block into general message and
// signatures for the underneath stateful verifier to do pure signature verify.
// It's used by collection cluster, where each signer only signs one staking sig with their staking key.
// Since threshold signature is not used, the signature validation can be implemented in a stateless manner.
type SingleSigValidator struct {
	identity2SignerIndex map[flow.Identifier]int // lookup signer index by identity
}

func (v *SingleSigValidator) ValidateVote(vote *model.Vote) error {
	panic("TO IMPLEMENT")
}

func (v *SingleSigValidator) ValidateBlock(block *model.Proposal) error {
	panic("TO IMPLEMENT")
}
