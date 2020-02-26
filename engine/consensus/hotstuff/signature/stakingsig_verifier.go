// +build relic

package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
)

// SigVerifier provides functions for verifying consensus nodes' staking signatures (i.e. votes)
type StakingSigVerifier struct {
	stakingHasher crypto.Hasher // the hasher for staking signature
}

func NewStakingSigVerifier(stakingSigTag string) StakingSigVerifier {
	return StakingSigVerifier{
		stakingHasher: crypto.NewBLS_KMAC(stakingSigTag),
	}
}

// VerifyStakingSig verifies a single BLS signature for a block using the given public key
// sig - the signature to be verified
// block - the block that the signature was signed for.
// signerKey - the public key of the signer who signed the block.
func (s *StakingSigVerifier) VerifyStakingSig(sig crypto.Signature, block *model.Block, signerKey crypto.PublicKey) (bool, error) {
	// convert into message bytes
	msg := BlockToBytesForSign(block)

	// validate the staking signature
	valid, err := signerKey.Verify(sig, msg, s.stakingHasher)
	if err != nil {
		return false, fmt.Errorf("cannot verify staking sig: %w", err)
	}
	return valid, nil
}

// VerifyAggregatedStakingSignature verifies an aggregated signature.
// aggsig - the aggregated signature to be verified
// block - the block that the signature was signed for.
// signerKeys - the public keys of all the signers who signed the block.
func (s *StakingSigVerifier) VerifyAggregatedStakingSignature(aggsig []crypto.Signature, block *model.Block, signerKeys []crypto.PublicKey) (bool, error) {
	// for now the aggregated staking signature for BLS signatures is implemented as a slice of all the signatures.
	// to verify it, we basically verify every single signature

	// check that the number of keys and signatures should match
	if len(aggsig) != len(signerKeys) {
		return false, nil
	}

	msg := BlockToBytesForSign(block)

	// check each signature
	for i, sig := range aggsig {
		signerKey := signerKeys[i]

		// validate the staking signature
		valid, err := signerKey.Verify(sig, msg, s.stakingHasher)
		if err != nil {
			return false, fmt.Errorf("cannot verify aggregated staking sig for (%d)-th sig: %w", i, err)
		}
		if !valid {
			return false, nil
		}
	}

	return true, nil
}
