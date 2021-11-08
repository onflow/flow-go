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
	"github.com/onflow/flow-go/model/flow"
	modulesig "github.com/onflow/flow-go/module/signature"
)

// SingleVerifierV2 is a verifier capable of verifying single staking signature for each
// verifying operation. It's used primarily with collection cluster where hotstuff without beacon signers is used.
type SingleVerifierV2 struct {
	committee      hotstuff.Committee
	staking        hash.Hasher
	keysAggregator *stakingKeysAggregator
}

// NewSingleVerifierV2 creates a new single verifier with the given dependencies.
// - the hotstuff committee's state is used to retrieve the public keys for the staking signature;
// - the staking tag is used to create hasher to verify staking signatures;
func NewSingleVerifierV2(committee hotstuff.Committee, stakingTag string) *SingleVerifierV2 {
	return &SingleVerifierV2{
		committee:      committee,
		staking:        crypto.NewBLSKMAC(stakingTag),
		keysAggregator: newStakingKeysAggregator(),
	}
}

// VerifyVote verifies the validity of a combined signature from a vote.
// Usually this method is only used to verify the proposer's vote, which is
// the vote included in a block proposal.
// TODO: return error only, because when the sig is invalid, the returned bool
// can't indicate whether it's staking sig was invalid, or beacon sig was invalid.
func (v *SingleVerifierV2) VerifyVote(signer *flow.Identity, sigData []byte, block *model.Block) (bool, error) {

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
	// TODO: check if using batch verification is faster (should be yes)
	stakingValid, err := signer.StakingPubKey.Verify(stakingSig, msg, v.staking)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	if !stakingValid {
		return false, fmt.Errorf("invalid staking sig")
	}

	return true, nil
}

// VerifyQC verifies the validity of a combined signature on a quorum certificate.
func (v *SingleVerifierV2) VerifyQC(signers flow.IdentityList, sigData []byte, block *model.Block) (bool, error) {
	// verify the aggregated staking signature
	// TODO: eventually VerifyMany will be a method of a stateful struct. The struct would
	// hold the message, all the participants keys, the latest verification aggregated public key,
	// as well as the latest list of signers (preferably a bit vector, using indices).
	// VerifyMany would only take the signature and the new list of signers (a bit vector preferably)
	// as inputs. A new struct needs to be used for each epoch since the list of participants is updated.

	msg := MakeVoteMessage(block.View, block.BlockID)

	aggregatedKey, err := v.keysAggregator.aggregatedStakingKey(signers)
	if err != nil {
		return false, fmt.Errorf("could not compute aggregated key: %w", err)
	}
	stakingValid, err := aggregatedKey.Verify(sigData, msg, v.staking)
	if err != nil {
		return false, fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	return stakingValid, nil
}
