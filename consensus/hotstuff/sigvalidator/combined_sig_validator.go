package sigvalidator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// CombinedSigValidator implements the SigValidator interface and owns the knowledge of how to
// decode the signer and signature fields. It converts the vote/block into general message and
// signatures for the underneath stateful verifier to do pure signature verify.
// It's used by consensus cluster, each signer signs two sigs: staking sig and threshold sig,
// and combine them together.
// The validation will first split the combined sig into two parts, then validate each sig part.
// A stateful sigVerifier is needed to verify and accumulate the threshold sig shares.
type CombinedSigValidator struct {
	identity2SignerIndex map[flow.Identifier]int     // lookup signer index by identity
	sigVerifier          hotstuff.RandomBeaconSigner // verifies the signature, but has no concept of vote
}

func NewCombinedSigValidator() *CombinedSigValidator {
	// TODO: replace with real input
	return &CombinedSigValidator{
		identity2SignerIndex: nil,
		sigVerifier:          nil,
	}
}

func (v *CombinedSigValidator) ValidateVote(vote *model.Vote) error {
	panic("TO IMPLEMENT")
}

func (v *CombinedSigValidator) ValidateBlock(block *model.Proposal) error {
	panic("TO IMPLEMENT")
}
