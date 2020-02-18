package hotstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// SigVerifier verifies signatures
type SigVerifier interface {
	// VerifySig verifies a single signature
	VerifySig(sig crypto.Signature, block *types.Block, signerKey crypto.PublicKey) (bool, error)

	// VerifyAggregatedSignature verifies an aggregated signature
	VerifyAggregatedSignature(aggsig *types.AggregatedSignature, block *types.Block, signerKeys []crypto.PublicKey) (bool, error)
}
