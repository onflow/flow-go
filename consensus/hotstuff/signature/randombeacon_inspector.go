package signature

import (
	"fmt"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/signature"
)

// randomBeaconInspector implements hotstuff.RandomBeaconInspector interface.
// All methods of this structure are concurrency-safe.
type randomBeaconInspector struct {
	inspector crypto.ThresholdSignatureInspector
}

// NewRandomBeaconInspector instantiates a new randomBeaconInspector.
// The constructor errors with a `model.ConfigurationError` in any of the following cases
//   - n is not between `ThresholdSignMinSize` and `ThresholdSignMaxSize`,
//     for n the number of participants `n := len(publicKeyShares)`
//   - threshold value is not in interval [1, n-1]
//   - any input public key is not a BLS key
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
		signature.RandomBeaconTag)
	if err != nil {
		if crypto.IsInvalidInputsError(err) || crypto.IsNotBLSKeyError(err) {
			return nil, model.NewConfigurationErrorf("invalid parametrization for BLS Threshold Signature Inspector: %w", err)
		}
		return nil, fmt.Errorf("unexpected exception while instantiating BLS Threshold Signature Inspector: %w", err)
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
//   - model.InvalidSignerError if signerIndex is invalid
//   - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
//   - other error if there is an unexpected exception.
func (r *randomBeaconInspector) Verify(signerIndex int, share crypto.Signature) error {
	valid, err := r.inspector.VerifyShare(signerIndex, share)
	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			return model.NewInvalidSignerError(err)
		}
		return fmt.Errorf("unexpected error verifying beacon signature from %d: %w", signerIndex, err)
	}

	if !valid { // invalid signature
		return fmt.Errorf("invalid beacon share from signer Index %d: %w", signerIndex, model.ErrInvalidSignature)
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
//   - (true, nil) if the signature has been added, and enough shares have been collected.
//   - (false, nil) if the signature has been added, but not enough shares were collected.
//
// The following errors are expected during normal operations:
//   - model.InvalidSignerError if signerIndex is invalid (out of the valid range)
//   - model.DuplicatedSignerError if the signer has been already added
//   - other error if there is an unexpected exception.
func (r *randomBeaconInspector) TrustedAdd(signerIndex int, share crypto.Signature) (bool, error) {
	// Trusted add to the crypto layer
	enough, err := r.inspector.TrustedAdd(signerIndex, share)
	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			return false, model.NewInvalidSignerError(err)
		}
		if crypto.IsDuplicatedSignerError(err) {
			return false, model.NewDuplicatedSignerError(err)
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
//   - (signature, nil) if no error occurred
//   - (nil, model.InsufficientSignaturesError) if not enough shares were collected
//   - (nil, model.InvalidSignatureIncluded) if at least one collected share does not serialize to a valid BLS signature,
//     or if the constructed signature failed to verify against the group public key and stored message. This post-verification
//     is required  for safety, as `TrustedAdd` allows adding invalid signatures.
//   - (nil, error) for any other unexpected error.
func (r *randomBeaconInspector) Reconstruct() (crypto.Signature, error) {
	sig, err := r.inspector.ThresholdSignature()
	if err != nil {
		if crypto.IsInvalidInputsError(err) || crypto.IsInvalidSignatureError(err) {
			return nil, model.NewInvalidSignatureIncludedError(err)
		}
		if crypto.IsNotEnoughSharesError(err) {
			return nil, model.NewInsufficientSignaturesError(err)
		}
		return nil, fmt.Errorf("unexpected error during random beacon sig reconstruction: %w", err)
	}
	return sig, nil
}
