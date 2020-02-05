package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// SigVerifier provides functions to verify single signature and aggregated signature.
type SigVerifier interface {
	// VerifySig verifies a single signature.
	// For valid signature, nil will be returned; for invalid signature, error will be returned.
	VerifySig(sig *types.VoteSignatureWithIndexedPubKey, message types.VoteBytes) error

	// VerifyAggregatedSig verifies the aggregated signature.
	// VerifyAggregatedSig checks if the given aggregated signature is aggregated from signatures
	// that are signed for a certain view by the given public keys over a message
	//
	// aggsig: the aggregated signature to be verified
	//
	// pubkeys: the public keys and signer indexes of whom generated each individule
	// signature that have been aggregated.
	//
	// blockID: the ID of the block that was signed.
	//
	// Note: blockID is needed when the aggregated signature contains threshold signature, in which case
	// the blockID is needed to find the group public key.
	// The group public key is needed to verify threshold signature.
	VerifyAggregatedSig(aggsig *types.AggregatedSignature, pubkeys []*types.IndexedPubKey, message types.VoteBytes, blockID flow.Identifier) error
}
