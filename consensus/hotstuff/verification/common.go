package verification

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// MakeVoteMessage generates the message we have to sign in order to be able
// to verify signatures without having the full block. To that effect, each data
// structure that is signed contains the sometimes redundant view number and
// block ID; this allows us to create the signed message and verify the signed
// message without having the full block contents.
func MakeVoteMessage(view uint64, blockID flow.Identifier) []byte {
	msg := flow.MakeID(struct {
		BlockID flow.Identifier
		View    uint64
	}{
		BlockID: blockID,
		View:    view,
	})
	return msg[:]
}

// verifyAggregatedSignature encapsulates the logic of verifying an aggregated signature
// under the same message.
// Proofs of possession of all input keys are assumed to be valid (checked by the protocol).
// This logic is commonly used across the different implementations of `hotstuff.Verifier`.
// In this context, all signatures apply to blocks.
// Return values:
//   - nil if `aggregatedSig` is valid against the public keys and message.
//   - model.InsufficientSignaturesError if `signers` is empty or nil.
//   - model.ErrInvalidSignature if the signature is invalid against the public keys and message.
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
func verifyAggregatedSignature(
	pubKeys []crypto.PublicKey, // public keys of actors to verify against
	aggregatedSig crypto.Signature, // aggregated signature to be checked
	hasher hash.Hasher, // hasher (contains usage-specific domain-separation tag)
	msg []byte, // message to verify against
) error {
	// TODO: as further optimization, replace the following call with model/signature.PublicKeyAggregator
	// the function could accept the public key aggrgator as an input
	aggregatedKey, err := crypto.AggregateBLSPublicKeys(pubKeys)
	if err != nil {
		// `AggregateBLSPublicKeys` returns an error in two distinct cases:
		//  (i) In case no keys are provided, i.e. `len(signers) == 0`.
		//      This scenario _is expected_ during normal operations, because a byzantine
		//      proposer might construct an (invalid) QC with an empty list of signers.
		// (ii) In case some provided public keys type is not BLS.
		//      This scenario is _not expected_ during normal operations, because all keys are
		//      guaranteed by the protocol to be BLS keys.
		if crypto.IsBLSAggregateEmptyListError(err) { // check case (i)
			return model.NewInsufficientSignaturesErrorf("aggregating public keys failed: %w", err)
		}
		// case (ii) or any other error are not expected during normal operations
		return fmt.Errorf("internal error computing aggregated key: %w", err)
	}

	valid, err := aggregatedKey.Verify(aggregatedSig, msg, hasher)
	if err != nil {
		return fmt.Errorf("internal error while verifying aggregated signature: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid aggregated signature: %w", model.ErrInvalidSignature)
	}
	return nil
}
