//go:build relic
// +build relic

package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
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
func (v *StakingVerifier) VerifyVote(signer *flow.Identity, sigData []byte, view uint64, blockID flow.Identifier) error {

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
//   - model.ErrInvalidSignature if a signature is invalid
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
//
// In the single verification case, `sigData` represents a single signature (`crypto.Signature`).
func (v *StakingVerifier) VerifyQC(signers flow.IdentityList, sigData []byte, view uint64, blockID flow.Identifier) error {
	if len(signers) == 0 {
		return model.NewInsufficientSignaturesErrorf("empty list of signers")
	}
	msg := MakeVoteMessage(view, blockID)

	// verify the aggregated staking signature
	// TODO: to be replaced by module/signature.PublicKeyAggregator in V2
	aggregatedKey, err := crypto.AggregateBLSPublicKeys(signers.PublicStakingKeys()) // caution: requires non-empty slice of keys!
	if err != nil {
		// `AggregateBLSPublicKeys` returns a `crypto.invalidInputsError` in two distinct cases:
		//  (i) In case no keys are provided, i.e.  `len(signers) == 0`.
		//      This scenario _is expected_ during normal operations, because a byzantine
		//      proposer might construct an (invalid) QC with an empty list of signers.
		// (ii) In case some provided public keys type is not BLS.
		//      This scenario is _not expected_ during normal operations, because all keys are
		//      guaranteed by the protocol to be BLS keys.
		//
		// By checking `len(signers) == 0` upfront , we can rule out case (i) as a source of error.
		// Hence, if we encounter an error here, we know it is case (ii). Thereby, we can clearly
		// distinguish a faulty _external_ input from an _internal_ uncovered edge-case.
		return fmt.Errorf("could not compute aggregated key: %w", err)
	}
	stakingValid, err := aggregatedKey.Verify(sigData, msg, v.stakingHasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying staking signature: %w", err)
	}

	if !stakingValid {
		return fmt.Errorf("invalid aggregated staking sig for block %v: %w", blockID, model.ErrInvalidSignature)
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
func (v *StakingVerifier) VerifyTC(signers flow.IdentityList, sigData []byte, view uint64, highQCViews []uint64) error {
	return verifyTC(signers, sigData, view, highQCViews, v.timeoutObjectHasher)
}

// verifyTC checks cryptographic validity of the TC's `sigData` w.r.t. the
// given view. It is the responsibility of the calling code to ensure
// that all `signers` are authorized, without duplicates. Return values:
//   - nil if `sigData` is cryptographically valid
//   - model.InsufficientSignaturesError if `signers is empty.
//   - model.InvalidFormatError if `signers`/`highQCViews` have differing lengths
//   - model.ErrInvalidSignature if a signature is invalid
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
func verifyTC(signers flow.IdentityList, sigData []byte, view uint64, highQCViews []uint64, hasher hash.Hasher) error {
	if len(signers) == 0 {
		return model.NewInsufficientSignaturesErrorf("empty list of signers")
	}
	if len(signers) != len(highQCViews) {
		return model.NewInvalidFormatErrorf("signers and highQCViews mismatch")
	}

	pks := make([]crypto.PublicKey, 0, len(signers))
	messages := make([][]byte, 0, len(signers))
	hashers := make([]hash.Hasher, 0, len(signers))
	for i, identity := range signers {
		pks = append(pks, identity.StakingPubKey)
		messages = append(messages, MakeTimeoutMessage(view, highQCViews[i]))
		hashers = append(hashers, hasher)
	}

	valid, err := crypto.VerifyBLSSignatureManyMessages(pks, sigData, messages, hashers)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid aggregated TC signature for view %d: %w", view, model.ErrInvalidSignature)
	}
	return nil
}
