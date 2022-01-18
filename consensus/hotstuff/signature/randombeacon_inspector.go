package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module/signature"
)

// randomBeaconInspector implements hotstuff.RandomBeaconInspector interface.
// All methods of this structure are concurrency-safe.
type randomBeaconInspector struct {
	inspector crypto.ThresholdSignatureInspector
}

// NewRandomBeaconInspector instantiates a new randomBeaconInspector.
//
// It errors with engine.InvalidInputError if any input is not valid.
func NewRandomBeaconInspector(
	groupPublicKey crypto.PublicKey,
	publicKeyShares []crypto.PublicKey,
	threshold int,
	message []byte,
) (*randomBeaconInspector, error) {

	inspector, err := crypto.NewBLSThresholdSignatureInspector(
		groupPublicKey,
		publicKeyShares,
		threshold,
		message,
		encoding.RandomBeaconTag)
	if err != nil {
		return nil, engine.NewInvalidInputErrorf("create a new Random Beacon inspector failed: %w", err)
	}

	return &randomBeaconInspector{
		inspector: inspector,
	}, nil
}

// Verify verifies the signature share under the signer's public key and the message agreed upon.
// The function is thread-safe and wait-free (i.e. allowing arbitrary many routines to
// execute the business logic, without interfering with each other).
// It allows concurrent verification of the given signature.
// Returns :
//  - engine.InvalidInputError if signerIndex is invalid
//  - module/signature.ErrInvalidFormat if signerID is valid but signature is cryptographically invalid
//  - other error if there is an unexpected exception.
func (r *randomBeaconInspector) Verify(signerIndex int, share crypto.Signature) error {
	verif, err := r.inspector.VerifyShare(signerIndex, share)
	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			return engine.NewInvalidInputErrorf("verify beacon share from %d failed: %w", signerIndex, err)
		}
		return fmt.Errorf("unexpected error verifying beacon signature from %d: %w", signerIndex, err)
	}

	if !verif { // invalid signature
		return fmt.Errorf("invalid beacon share from %d: %w", signerIndex, signature.ErrInvalidFormat)
	}
	return nil
}

// TrustedAdd adds a share to the internal signature shares store.
// There is no pre-check of the signature's validity _before_ adding it.
// It is the caller's responsibility to make sure the signature was previously verified.
// Nevertheless, the implementation guarantees safety (only correct threshold signatures
// are returned) through a post-check (verifying the threshold signature
// _after_ reconstruction before returning it).
// The function is thread-safe but locks its internal state, thereby permitting only
// one routine at a time to add a signature.
// Returns:
//  - (true, nil) if the signature has been added, and enough shares have been collected.
//  - (false, nil) if the signature has been added, but not enough shares were collected.
//  - (false, error) if there is any exception adding the signature share.
//      - engine.InvalidInputError if signerIndex is invalid (out of the valid range)
//  	- engine.DuplicatedEntryError if the signer has been already added
//      - other error if there is an unexpected exception.
func (r *randomBeaconInspector) TrustedAdd(signerIndex int, share crypto.Signature) (enoughshares bool, exception error) {
	// Trusted add to the crypto layer
	enough, err := r.inspector.TrustedAdd(signerIndex, share)
	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			return false, engine.NewInvalidInputErrorf("trusted add from %d failed: %w", signerIndex, err)
		}
		if crypto.IsDuplicatedSignerError(err) {
			return false, engine.NewDuplicatedEntryErrorf("trusted add from %d failed: %w", signerIndex, err)
		}
		return false, fmt.Errorf("unexpected error while adding share from %d: %w", signerIndex, err)
	}
	return enough, nil
}

// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
// a group signature.
//
// The function is write-blocking
func (r *randomBeaconInspector) EnoughShares() bool {
	return r.inspector.EnoughShares()
}

// Reconstruct reconstructs the group signature. The function is thread-safe but locks
// its internal state, thereby permitting only one routine at a time.
//
// Returns:
// - (signature, nil) if no error occurred
// - (nil, crypto.notEnoughSharesError) if not enough shares were collected
// - (nil, crypto.invalidInputsError) if at least one collected share does not serialize to a valid BLS signature,
//    or if the constructed signature failed to verify against the group public key and stored message. This post-verification
//    is required  for safety, as `TrustedAdd` allows adding invalid signatures.
// - (nil, error) for any other unexpected error.
func (r *randomBeaconInspector) Reconstruct() (crypto.Signature, error) {
	return r.inspector.ThresholdSignature()
}
