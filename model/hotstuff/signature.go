package hotstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// SingleSignature is a signature from a single signer, which can be aggregaged
// into aggregated signature.
type SingleSignature struct {
	StakingSignature      crypto.Signature // The raw bytes for BLS signature
	RandomBeaconSignature crypto.Signature // The raw bytes for Threshold Signature for generating random beacon
	SignerID              flow.Identifier  // The identifier of the signer, which identifies signer's role, stake, etc.
}

// AggregatedSignature is the aggregated signatures signed by all the signers on the same message.
type AggregatedSignature struct {
	StakingSignatures     []crypto.Signature // The raw bytes for aggregated BLS signature
	RandomBeaconSignature crypto.Signature   // The aggregated threshold signature for generating random beacon
	SignerIDs             []flow.Identifier  // The Identifiers of all the signers
}
