package verification

import (
	"fmt"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"

	"encoding/binary"
)

// MakeVoteMessage generates the message we have to sign in order to be able
// to verify signatures without having the full block. To that effect, each data
// structure that is signed contains the sometimes redundant view number and
// block ID; this allows us to create the signed message and verify the signed
// message without having the full block contents.
func MakeVoteMessage(view uint64, blockID flow.Identifier) []byte {
	msg := make([]byte, 8, 8+flow.IdentifierLen)
	binary.BigEndian.PutUint64(msg, view)
	msg = append(msg, blockID[:]...)
	return msg
}

// MakeTimeoutMessage generates the message we have to sign in order to be able
// to contribute to Active Pacemaker protocol. Each replica signs with the highest QC view
// known to that replica.
func MakeTimeoutMessage(view uint64, newestQCView uint64) []byte {
	msg := make([]byte, 16)
	binary.BigEndian.PutUint64(msg[:8], view)
	binary.BigEndian.PutUint64(msg[8:], newestQCView)
	return msg
}

// verifyAggregatedSignatureOneMessage encapsulates the logic of verifying an aggregated signature
// under the same message.
// Proofs of possession of all input keys are assumed to be valid (checked by the protocol).
// This logic is commonly used across the different implementations of `hotstuff.Verifier`.
// In this context, all signatures apply to blocks.
// Return values:
//   - nil if `aggregatedSig` is valid against the public keys and message.
//   - model.InsufficientSignaturesError if `pubKeys` is empty or nil.
//   - model.ErrInvalidSignature if the signature is invalid against the public keys and message.
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
func verifyAggregatedSignatureOneMessage(
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
		//  (i) In case no keys are provided, i.e. `len(pubKeys) == 0`.
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

// verifyTCSignatureManyMessages checks cryptographic validity of the TC's signature w.r.t.
// multiple messages and public keys.
// Proofs of possession of all input keys are assumed to be valid (checked by the protocol).
// This logic is commonly used across the different implementations of `hotstuff.Verifier`.
// It is the responsibility of the calling code to ensure that all `pks` are authorized,
// without duplicates. The caller must also make sure the `hasher` passed is non nil and has
// 128-bytes outputs.
// Return values:
//   - nil if `sigData` is cryptographically valid
//   - model.InsufficientSignaturesError if `pks` is empty.
//   - model.InvalidFormatError if `pks`/`highQCViews` have differing lengths
//   - model.ErrInvalidSignature if a signature is invalid
//   - unexpected errors should be treated as symptoms of bugs or uncovered
//     edge cases in the logic (i.e. as fatal)
func verifyTCSignatureManyMessages(
	pks []crypto.PublicKey,
	sigData crypto.Signature,
	view uint64,
	highQCViews []uint64,
	hasher hash.Hasher,
) error {
	if len(pks) != len(highQCViews) {
		return model.NewInvalidFormatErrorf("public keys and highQCViews mismatch")
	}

	messages := make([][]byte, 0, len(pks))
	hashers := make([]hash.Hasher, 0, len(pks))
	for i := 0; i < len(pks); i++ {
		messages = append(messages, MakeTimeoutMessage(view, highQCViews[i]))
		hashers = append(hashers, hasher)
	}

	valid, err := crypto.VerifyBLSSignatureManyMessages(pks, sigData, messages, hashers)
	if err != nil {
		// `VerifyBLSSignatureManyMessages` returns an error in a few cases:
		//  (i)  In case no keys are provided, i.e. `len(pks) == 0`.
		//       This scenario _is expected_ during normal operations, because a byzantine
		//       proposer might construct an (invalid) TC with an empty list of signers.
		// (ii)  In case some provided public keys type is not BLS.
		//       This scenario is _not expected_ during normal operations, because all keys are
		//       guaranteed by the protocol to be BLS keys.
		// (iii) In case at least one hasher is nil. This is not expected because
		//       as per the function assumption.
		// (iv)  In case at least one hasher has an incorrect size. This is not expected
		//       because as per the function assumption.
		if crypto.IsBLSAggregateEmptyListError(err) { // check case (i)
			return model.NewInsufficientSignaturesErrorf("aggregating public keys failed: %w", err)
		}
		// any other error is not expected during normal operations
		return fmt.Errorf("signature verification failed: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid aggregated TC signature for view %d: %w", view, model.ErrInvalidSignature)
	}
	return nil
}
