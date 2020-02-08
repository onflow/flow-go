package signature

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// SigVerifier provides functions to verify single signature and aggregated signature.
type SigVerifier interface {
	// VerifySig verifies a single signature.
	// It reads the public key from the signature's identity field, and verifies the raw sig against
	// the given message.
	VerifySig(sig *types.Signature, message types.VoteBytes) (bool, error)

	// VerifyAggregatedSig verifies the aggregated signature.
	// aggsig - the aggregated signature to be verified
	// message - the message that each signer was signed.
	VerifyAggregatedSig(aggsig *types.AggregatedSignature, message types.VoteBytes) (bool, error)
}
