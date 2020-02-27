package hotstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// SigVerifier verifies signatures
type SigVerifier interface {
	// VerifyStakingSig verifies a single staking signature
	VerifyStakingSig(stakingSig crypto.Signature, block *hotstuff.Block, signerKey crypto.PublicKey) (bool, error)

	// VerifyRandomBeaconSig verifies a single random beacon signature
	VerifyRandomBeaconSig(randomBeaconSig crypto.Signature, block *hotstuff.Block, signerKey crypto.PublicKey) (bool, error)

	// VerifyAggregatedStakingSignature verifies an aggregated staking signature
	VerifyAggregatedStakingSignature(aggsig []crypto.Signature, block *hotstuff.Block, signerKeys []crypto.PublicKey) (bool, error)

	// VerifyAggregatedRandomBeaconSignature verifies an aggregated random beacon signature
	VerifyAggregatedRandomBeaconSignature(sig crypto.Signature, block *hotstuff.Block) (bool, error)
}
