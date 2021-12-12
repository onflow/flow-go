//go:build relic
// +build relic

package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
)

// StakingVerifier is a verifier capable of verifying staking signature for each
// verifying operation. It's used primarily with collection cluster where hotstuff without beacon signers is used.
type StakingVerifier struct {
	stakingHasher hash.Hasher
}

// NewStakingVerifier creates a new single verifier with the given dependencies.
func NewStakingVerifier() *StakingVerifier {
	return &StakingVerifier{
		stakingHasher: crypto.NewBLSKMAC(encoding.CollectorVoteTag),
	}
}

// VerifyVote verifies the validity of a single signature from a vote.
// Usually this method is only used to verify the proposer's vote, which is
// the vote included in a block proposal.
// The implementation returns the following sentinel errors:
// * signature.ErrInvalidFormat if the signature has an incompatible format.
// * model.ErrInvalidSignature is the signature is invalid
// * unexpected errors should be treated as symptoms of bugs or uncovered
//   edge cases in the logic (i.e. as fatal)
func (v *StakingVerifier) VerifyVote(signer *flow.Identity, sigData []byte, block *model.Block) error {

	// create the to-be-signed message
	msg := MakeVoteMessage(block.View, block.BlockID)

	// verify each signature against the message
	stakingValid, err := signer.StakingPubKey.Verify(sigData, msg, v.stakingHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	if !stakingValid {
		return fmt.Errorf("invalid sig for block %v: %w", block.BlockID, model.ErrInvalidSignature)
	}

	return nil
}

// VerifyQC verifies the validity of a single signature on a quorum certificate.
//
// In the single verification case, `sigData` represents a single signature (`crypto.Signature`).
func (v *StakingVerifier) VerifyQC(signers flow.IdentityList, sigData []byte, block *model.Block) (bool, error) {
	// verify the aggregated staking signature
	msg := MakeVoteMessage(block.View, block.BlockID)

	pks := make([]crypto.PublicKey, 0, len(signers))
	for _, identity := range signers {
		pks = append(pks, identity.StakingPubKey)
	}

	// TODO: to be replaced by module/signature.PublicKeyAggregator in V2
	aggregatedKey, err := crypto.AggregateBLSPublicKeys(pks)
	if err != nil {
		return false, fmt.Errorf("could not compute aggregated key: %w", err)
	}
	stakingValid, err := aggregatedKey.Verify(sigData, msg, v.stakingHasher)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	return stakingValid, nil
}
