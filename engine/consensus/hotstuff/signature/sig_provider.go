package signature

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// SigProvider provides symmetry functions to generate and verify signatures
type SigProvider struct {
}

// VerifySig verifies a single signature.
// sig - the signature to be verified
// signer - the signer who signed the message. It was read from signature's Signer field
// message - the message that was signed by the signer
func (s *SigProvider) VerifySig(sig crypto.Signature, block *types.Block, signerKeys ...crypto.PublicKey) (bool, error) {
	panic("TODO")
}
