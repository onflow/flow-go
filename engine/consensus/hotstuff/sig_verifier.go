package hotstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// SigVerifier verifies signatures
type SigVerifier interface {
	// VerifySig verifies a single signature
	VerifySig(sig crypto.Signature, block *hotstuff.Block, signerKey crypto.PublicKey) (bool, error)

	// VerifyAggregatedSignature verifies an aggregated signature
	VerifyAggregatedSignature(aggsig *hotstuff.AggregatedSignature, block *hotstuff.Block, signerKeys []crypto.PublicKey) (bool, error)
}
