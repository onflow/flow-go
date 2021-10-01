package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module/signature"
)

// randomBeaconFollower implements hotstuff.RandomBeaconFollower interface
type randomBeaconFollower struct {
	follower crypto.ThresholdSignatureFollower
}

// NewRandomBeaconFollower returns a new Random Beacon follower instance.
//
// It errors with crypto.InvalidInputsError if any input is not valid.
func NewRandomBeaconFollower(
	groupPublicKey crypto.PublicKey,
	publicKeyShares []crypto.PublicKey,
	threshold int,
	message []byte,
) (*randomBeaconFollower, error) {

	follower, err := crypto.NewBLSThresholdSignatureFollower(
		groupPublicKey,
		publicKeyShares,
		threshold,
		message,
		encoding.RandomBeaconTag)

	if err != nil {
		return nil, fmt.Errorf("create a new Random Beacon follower failed: %w", err)
	}

	return &randomBeaconFollower{
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
func (r *randomBeaconFollower) Verify(signerIndex int, share crypto.Signature) error {
	verif, err := r.follower.VerifyShare(signerIndex, share)

	// check for errors
	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			// this erorr happens because of an invalid index
			return engine.NewInvalidInputErrorf("Verify beacon signature from %d failed: %w", signerIndex, err)
		}
		else {
			// other exceptions
			return fmt.Errorf("Verify beacon signature from %d failed: %w", err)
		}
	}

	if !verif {
		// invalid signature
		return fmt.Errorf("invalid signature from %d: %w", signerIndex, signature.ErrInvalidFormat)
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
func (r *randomBeaconFollower) TrustedAdd(signerIndex int, share crypto.Signature) (enoughshares bool, exception error) {
	
	// check index and duplication
	ok, err := r.follower.HasShare(signerIndex, share)
	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			// means index is invalid
			return false, engine.NewInvalidInputErrorf("trusted add failed: %w", err)
		} else {
			// other exceptions
			return false, fmt.Errorf("trusted add failed: %w", err)
		}
	}
	if ok {
		// duplicate
		return false, engine.NewDuplicatedEntryErrorf("signer %d was already added", signerIndex)
	}
	
	// Trusted add to the crypto layer
	enough, err := r.follower.TrustedAdd(signerIndex, share)
	// sanity check for error, although error should be nil here
	if err != nil {
		return false, fmt.Errorf("trusted add failed: %w", err)
	}
	return enough, nil
}

// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
// a group signature.
func (r *randomBeaconFollower) EnoughShares() bool {

}

// Reconstruct reconstructs the group signature.
// The reconstructed signature is verified against the overall group public key and the message agreed upon.
// This is a sanity check that is necessary since "TrustedAdd" allows adding non-verified signatures.
// Reconstruct returns an error if the reconstructed signature fails the sanity verification, or if not enough shares have been collected.
func (r *randomBeaconFollower) Reconstruct() (crypto.Signature, error) {

}
