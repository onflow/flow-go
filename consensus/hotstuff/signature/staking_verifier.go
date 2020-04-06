// +build relic

package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
)

// StakingSigVerifier verifies signatures generated with staking keys. Specifically, it verifies
// individual signatures (e.g. from a vote) and aggregated signatures (e.g. from a Quorum Certificate).
type StakingSigVerifier struct {
	stakingHasher hash.Hasher // the hasher for staking signature
}

// NewStakingSigVerifier constructs a new StakingSigVerifier
// The tag used for identifying the vote is different between collector and consensus nodes.
func NewStakingSigVerifier(stakingSigTag string) StakingSigVerifier {
	return StakingSigVerifier{
		stakingHasher: crypto.NewBLS_KMAC(stakingSigTag),
	}
}

// VerifyStakingSig verifies a single BLS staking signature for a block using signer's public key
// sig - the signature to be verified
// block - the block that the signature was signed for.
// signerKey - the public key of the signer who signed the block.
//
// Note: we are specifically choosing safety over performance here.
//   * The vote itself contains all the information for verifying the signature: the blockID and the block's view
//   * We could use the vote to verify that the signature is valid for the information contained in the vote's message
//   * However, for security, we are explicitly verifying that the vote matches the full block.
//     We do this by converting the block to the byte-sequence which we expect an honest voter to have signed
//     and then check the provided signature against this self-computed byte-sequence.
func (v *StakingSigVerifier) VerifyStakingSig(sig crypto.Signature, block *model.Block, signerKey crypto.PublicKey) (bool, error) {
	msg := BlockToBytesForSign(block)
	valid, err := signerKey.Verify(sig, msg, v.stakingHasher)
	if err != nil {
		return false, fmt.Errorf("cannot verify staking sig: %w", err)
	}
	return valid, nil
}

// VerifyStakingAggregatedSig verifies an aggregated BLS signature.
// Inputs:
//    aggStakingSig - the aggregated staking signature to be verified
//    block - the block that the signature was signed for.
//    signerKeys - the signer's public staking key
//
// Note: we are specifically choosing safety over performance here.
//   * The vote itself contains all the information for verifying the signature: the blockID and the block's view
//   * We could use the vote to verify that the signature is valid for the information contained in the vote's message
//   * However, for security, we are explicitly verifying that the vote matches the full block.
//     We do this by converting the block to the byte-sequence which we expect an honest voter to have signed
//     and then check the provided signature against this self-computed byte-sequence.
//
// Note: the aggregated BLS staking signature is currently implemented as a slice of individual signatures.
// The implementation (and method signature) will later be updated, once full BLS signature aggregation is implemented.
// To verify it, we just verify every single signature. The implementation (and method signature)
func (v *StakingSigVerifier) VerifyStakingAggregatedSig(aggStakingSig []crypto.Signature, block *model.Block, signerKeys []crypto.PublicKey) (bool, error) {
	// check that the number of keys and signatures should match
	if len(aggStakingSig) != len(signerKeys) {
		return false, nil
	}

	msg := BlockToBytesForSign(block)

	// check each signature
	for i, sig := range aggStakingSig {
		signerKey := signerKeys[i]

		// validate the staking signature
		valid, err := signerKey.Verify(sig, msg, v.stakingHasher)
		if err != nil {
			return false, fmt.Errorf("cannot verify aggregated staking sig for (%d)-th sig: %w", i, err)
		}
		if !valid {
			return false, nil
		}
	}
	return true, nil
}
