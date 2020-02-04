package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type SigVerifier interface {
	// VerifySig verifies a single signature.
	VerifySig(sig *types.VoteSignatureWithPubKey, message types.VoteBytes) error

	// VerifyAggregatedSig checks if the given aggregated signature is aggregated from signatures
	// that are signed for a certain view by the given public keys over a message
	// aggsig: the aggregated signature
	// pubkeys: the public keys of whom generated each individule signature that have been aggregated.
	// view: the view of the block that was signed.
	// Note: view is needed when the aggregated signature contains threshold signature, in which case
	// the view is needed to find the group public key. The group public key is needed to verify threshold
	// signature.
	VerifyAggregatedSig(aggsig *types.AggregatedSignature, pubkeys []types.PubKey, message types.VoteBytes, blockID flow.Identifier) error
}
