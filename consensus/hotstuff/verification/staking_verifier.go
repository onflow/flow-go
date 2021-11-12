//go:build relic
// +build relic

package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	modulesig "github.com/onflow/flow-go/module/signature"
)

// StakingVerifier is a verifier capable of verifying staking signature for each
// verifying operation. It's used primarily with collection cluster where hotstuff without beacon signers is used.
type StakingVerifier struct {
	committee     hotstuff.Committee
	stakingHasher hash.Hasher
	// TODO: to be replaced by module/signature.PublicKeyAggregator in V2
	keysAggregator *stakingKeysAggregator
}

// NewStakingVerifier creates a new single verifier with the given dependencies.
// - the hotstuff committee's state is used to retrieve the public keys for the staking signature;
func NewStakingVerifier(committee hotstuff.Committee) *StakingVerifier {
	return &StakingVerifier{
		committee:      committee,
		stakingHasher:  crypto.NewBLSKMAC(encoding.CollectorVoteTag),
		keysAggregator: newStakingKeysAggregator(),
	}
}

// VerifyVote verifies the validity of a single signature from a vote.
// Usually this method is only used to verify the proposer's vote, which is
// the vote included in a block proposal.
// TODO: return error only, because when the sig is invalid, the returned bool
func (v *StakingVerifier) VerifyVote(signer *flow.Identity, sigData []byte, block *model.Block) (bool, error) {

	// create the to-be-signed message
	msg := MakeVoteMessage(block.View, block.BlockID)

	sigType, stakingSig, err := signature.DecodeSingleSig(sigData)
	if err != nil {
		return false, fmt.Errorf("could not decode signature: %w", modulesig.ErrInvalidFormat)
	}

	if sigType != hotstuff.SigTypeStaking {
		return false, fmt.Errorf("expected staking signature: %w", modulesig.ErrInvalidFormat)
	}

	// verify each signature against the message
	stakingValid, err := signer.StakingPubKey.Verify(stakingSig, msg, v.stakingHasher)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	if !stakingValid {
		return false, fmt.Errorf("invalid staking sig")
	}

	return true, nil
}

// VerifyQC verifies the validity of a single signature on a quorum certificate.
//
// In the single verification case, `sigData` represents a single signature (`crypto.Signature`).
func (v *StakingVerifier) VerifyQC(signers flow.IdentityList, sigData []byte, block *model.Block) (bool, error) {
	// verify the aggregated staking signature

	msg := MakeVoteMessage(block.View, block.BlockID)

	aggregatedKey, err := v.keysAggregator.aggregatedStakingKey(signers)
	if err != nil {
		return false, fmt.Errorf("could not compute aggregated key: %w", err)
	}
	stakingValid, err := aggregatedKey.Verify(sigData, msg, v.stakingHasher)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	return stakingValid, nil
}
