package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module/signature"
)

// randomBeaconInspector implements hotstuff.RandomBeaconInspector interface.
// All methods of this structure are thread-safe.
type randomBeaconInspector struct {
	follower crypto.ThresholdSignatureInspector
}

// NewRandomBeaconInspector returns a new Random Beacon follower instance.
//
// It errors with engine.InvalidInputError if any input is not valid.
func NewRandomBeaconInspector(
	groupPublicKey crypto.PublicKey,
	publicKeyShares []crypto.PublicKey,
	threshold int,
	message []byte,
) (*randomBeaconInspector, error) {

	follower, err := crypto.NewBLSThresholdSignatureInspector(
		groupPublicKey,
		publicKeyShares,
		threshold,
		message,
		encoding.RandomBeaconTag)

	if err != nil {
		return nil, engine.NewInvalidInputErrorf("create a new Random Beacon follower failed: %w", err)
	}

	return &randomBeaconInspector{
		follower: follower,
	}, nil
}

// Verify verifies the signature share under the signer's public key and the message agreed upon.
// It allows concurrent verification of the given signature.
// It returns :
//  - engine.InvalidInputError if signerIndex is invalid
//  - module/signature.ErrInvalidFormat if signerID is valid but signature is cryptographically invalid
//  - other error if there is an exception.
// The function call is non-blocking
func (r *randomBeaconInspector) Verify(signerIndex int, share crypto.Signature) error {
	verif, err := r.follower.VerifyShare(signerIndex, share)

	// check for errors
	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			// this erorr happens because of an invalid index
			return engine.NewInvalidInputErrorf("Verify beacon signature from %d failed: %w", signerIndex, err)
		} else {
			// other exceptions
			return fmt.Errorf("Verify beacon signature from %d failed: %w", signerIndex, err)
		}
	}

	if !verif {
		// invalid signature
		return fmt.Errorf("invalid beacon signature from %d: %w", signerIndex, signature.ErrInvalidFormat)
	}
	return nil
}

// TrustedAdd adds a share to the internal signature shares store.
// The function does not verify the signature is valid. It is the caller's responsibility
// to make sure the signature was previously verified.
// It returns:
//  - (true, nil) if the signature has been added, and enough shares have been collected.
//  - (false, nil) if the signature has been added, but not enough shares were collected.
//  - (false, error) if there is any exception adding the signature share.
//      - engine.InvalidInputError if signerIndex is invalid
//  	- engine.DuplicatedEntryError if the signer has been already added
//      - other error if other exceptions
// The function call is blocking.
func (r *randomBeaconInspector) TrustedAdd(signerIndex int, share crypto.Signature) (enoughshares bool, exception error) {

	// Trusted add to the crypto layer
	enough, err := r.follower.TrustedAdd(signerIndex, share)

	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			// means index is invalid
			return false, engine.NewInvalidInputErrorf("trusted add failed: %w", err)
		} else if crypto.IsDuplicatedSignerError(err) {
			// signer was added
			return false, engine.NewDuplicatedEntryErrorf("trusted add failed: %w", err)
		} else {
			// other exceptions
			return false, fmt.Errorf("trusted add failed because of an exception: %w", err)
		}
	}
	return enough, nil
}

// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
// a group signature.
//
// The function is write-blocking
func (r *randomBeaconInspector) EnoughShares() bool {
	return r.follower.EnoughShares()
}

// Reconstruct reconstructs the group signature.
//
// Returns:
// - (signature, nil) if no error occured
// - (nil, crypto.notEnoughSharesError) if not enough shares were collected
// - (nil, crypto.invalidInputsError) if at least one collected share does not serialize to a valid BLS signature,
//    or if the constructed signature failed to verify against the group public key and stored message. This post-verification
//    is required  for safety, as `TrustedAdd` allows adding invalid signatures.
// - (nil, error) for any other unexpected error.
// The function is blocking.
func (r *randomBeaconInspector) Reconstruct() (crypto.Signature, error) {
	return r.follower.ThresholdSignature()
}
