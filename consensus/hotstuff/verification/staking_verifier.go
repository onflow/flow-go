package verification

import (
	"fmt"

	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
)

// StakingVerifier is a verifier capable of verifying staking signature for each
// verifying operation. It's used primarily with collection cluster where hotstuff without beacon signers is used.
type StakingVerifier struct {
	stakingHasher       hash.Hasher
	timeoutObjectHasher hash.Hasher
}

var _ hotstuff.Verifier = (*StakingVerifier)(nil)

// NewStakingVerifier creates a new single verifier with the given dependencies.
func NewStakingVerifier() *StakingVerifier {
	return &StakingVerifier{
		stakingHasher:       msig.NewBLSHasher(msig.CollectorVoteTag),
		timeoutObjectHasher: msig.NewBLSHasher(msig.CollectorTimeoutTag),
	}
}

// VerifyVote verifies the validity of a single signature from a vote.
// Usually this method is only used to verify the proposer's vote, which is
// the vote included in a block proposal.
// The implementation returns the following sentinel errors:
//   - model.ErrInvalidSignature is the signature is invalid
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
func (v *StakingVerifier) VerifyVote(signer *flow.IdentitySkeleton, sigData []byte, view uint64, blockID flow.Identifier) error {

	// create the to-be-signed message
	msg := MakeVoteMessage(view, blockID)

	// verify each signature against the message
	stakingValid, err := signer.StakingPubKey.Verify(sigData, msg, v.stakingHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying staking signature: %w", err)
	}
	if !stakingValid {
		return fmt.Errorf("invalid sig for block %v: %w", blockID, model.ErrInvalidSignature)
	}

	return nil
}

// VerifyQC checks the cryptographic validity of the QC's `sigData` for the
// given block. It is the responsibility of the calling code to ensure
// that all `signers` are authorized, without duplicates. Return values:
//   - nil if `sigData` is cryptographically valid
//   - model.InvalidFormatError if `sigData` has an incompatible format
//   - model.InsufficientSignaturesError if `signers` is empty.
//   - model.ErrInvalidSignature if a signature is invalid
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
//
// In the single verification case, `sigData` represents a single signature (`crypto.Signature`).
func (v *StakingVerifier) VerifyQC(signers flow.IdentitySkeletonList, sigData []byte, view uint64, blockID flow.Identifier) error {
	msg := MakeVoteMessage(view, blockID)

	err := verifyAggregatedSignatureOneMessage(signers.PublicStakingKeys(), sigData, v.stakingHasher, msg)
	if err != nil {
		return fmt.Errorf("verifying aggregated staking signature failed for block %v: %w", blockID, err)
	}

	return nil
}

// VerifyTC checks cryptographic validity of the TC's `sigData` w.r.t. the
// given view. It is the responsibility of the calling code to ensure
// that all `signers` are authorized, without duplicates. Return values:
//   - nil if `sigData` is cryptographically valid
//   - model.InsufficientSignaturesError if `signers is empty.
//   - model.InvalidFormatError if `signers`/`highQCViews` have differing lengths
//   - model.ErrInvalidSignature if a signature is invalid
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
func (v *StakingVerifier) VerifyTC(signers flow.IdentitySkeletonList, sigData []byte, view uint64, highQCViews []uint64) error {
	return verifyTCSignatureManyMessages(signers.PublicStakingKeys(), sigData, view, highQCViews, v.timeoutObjectHasher)
}
