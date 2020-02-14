package signature

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// SigProvider provides symmetry functions to generate and verify signatures
type SigProvider struct {
	hasher crypto.Hasher
}

// VerifySig verifies a single signature for a block using the given public key
// sig - the signature to be verified
// block - the block that the signature was signed for.
// signerKey - the public key of the signer who signed the block.
func (s *SigProvider) VerifySig(sig crypto.Signature, block *types.Block, signerKey crypto.PublicKey) (bool, error) {
	msg := BlockToBytesForSign(block)
	return signerKey.Verify(sig, msg[:], s.hasher)
}

// VerifyAggregatedSignature verifies an aggregated signature.
// aggsig - the aggregated signature to be verified
// block - the block that the signature was signed for.
// signerKeys - the public keys of all the signers who signed the block.
func (s *SigProvider) VerifyAggregatedSignature(aggsig *types.AggregatedSignature, block *types.Block, signerKeys []crypto.PublicKey) (bool, error) {

	// for now the aggregated signature for BLS signatures is implemented as a slice of all the signatures.
	// to verifiy it, we basically verify every single signature

	// check that the number of keys and signatures should match
	sigs := aggsig.Raw
	if len(sigs) != len(signerKeys) {
		return false, nil
	}

	// check each signature
	for i, sig := range sigs {
		signerKey := signerKeys[i]
		valid, err := s.VerifySig(sig, block, signerKey)
		if err != nil {
			return false, err
		}
		if !valid {
			return false, nil
		}
	}
	return true, nil
}

// BlockToBytesForSign generates the bytes that was signed for a block
// Note: this function should be reused when signing a block or a vote
func BlockToBytesForSign(block *types.Block) flow.Identifier {
	return flow.MakeID(struct {
		BlockID flow.Identifier
		View    uint64
	}{
		BlockID: block.BlockID,
		View:    block.View,
	})
}