//go:build relic
// +build relic

package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
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

var _ hotstuff.Verifier = (*StakingVerifier)(nil)

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
// * model.ErrInvalidFormat if the signature has an incompatible format.
// * model.ErrInvalidSignature is the signature is invalid
// * unexpected errors should be treated as symptoms of bugs or uncovered
//   edge cases in the logic (i.e. as fatal)
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
// that all `voters` are authorized, without duplicates. Return values:
//  - nil if `sigData` is cryptographically valid
//  - model.ErrInvalidFormat if `sigData` has an incompatible format
//  - model.ErrInvalidSignature if a signature is invalid
//  - unexpected errors should be treated as symptoms of bugs or uncovered
//	  edge cases in the logic (i.e. as fatal)
// In the single verification case, `sigData` represents a single signature (`crypto.Signature`).
func (v *StakingVerifier) VerifyQC(signers flow.IdentityList, sigData []byte, view uint64, blockID flow.Identifier) error {
	if len(signers) == 0 {
		return fmt.Errorf("empty list of signers: %w", model.ErrInvalidFormat)
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

func (v *StakingVerifier) VerifyTC(voters flow.IdentityList, sigData []byte, view uint64, highQCViews []uint64) error {
	pks := make([]crypto.PublicKey, 0, len(voters))
	messages := make([][]byte, 0, len(voters))
	hashers := make([]hash.Hasher, 0, len(voters))
	for _, identity := range voters {
		pks = append(pks, identity.StakingPubKey)
		// TODO(active-pacemaker): construct valid message
		var msg []byte
		messages = append(messages, msg)
		hashers = append(hashers, v.stakingHasher)
	}

	valid, err := crypto.VerifyBLSSignatureManyMessages(pks, sigData, messages, hashers)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid aggregated TC signature for view %d: %w", view, err)
	}
	return nil
}
