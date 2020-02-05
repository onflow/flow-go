package signature

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// SigVerifier provides functions to verify single signature and aggregated signature.
type SigVerifier interface {
	// VerifySig verifies a single signature.
	// sig - the signature to be verified
	// message - the message that was signed
	// blockID - the blockID of the data (vote) to be signed.
	// The signer index in the signature and the blockID determines the public key, which will
	// be used to verify the signature.
	// blockID also determines the group public key to verify threshold signature.
	VerifySig(sig *flow.PartialSignature, message types.VoteBytes, blockID flow.Identifier) (bool, error)

	// VerifyAggregatedSig verifies the aggregated signature.
	// VerifyAggregatedSig checks if the given aggregated signature is aggregated from signatures
	// that are signed for a certain view by the given public keys over a message
	//
	// aggsig: the aggregated signature to be verified
	// blockID: the ID of the block that was signed.
	// Note: blockID is needed when the aggregated signature contains threshold signature, in which case
	// the blockID is needed to find the group public key.
	// The group public key is needed to verify threshold signature.
	VerifyAggregatedSig(aggsig *types.AggregatedSignature, message types.VoteBytes, blockID flow.Identifier) (bool, error)
}
