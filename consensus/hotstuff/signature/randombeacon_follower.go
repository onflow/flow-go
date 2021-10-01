package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding"
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
//  - nil if signature is valid,
//  - crypto.InvalidInputsError if the signature is invalid,
//  - other error if there is an exception.
func (r *randomBeaconFollower) Verify(signerIndex int, share crypto.Signature) error {

}

// TrustedAdd adds a share to the internal signature shares store.
// The operation is sequential.
// The function does not verify the signature is valid. It is the caller's responsibility
// to make sure the signature was previously verified.
// It returns:
//  - (true, nil) if the signature has been added, and enough shares have been collected.
//  - (false, nil) if the signature has been added, but not enough shares were collected.
//  - (false, error) if there is any exception adding the signature share.
func (r *randomBeaconFollower) TrustedAdd(signerIndex int, share crypto.Signature) (enoughshares bool, exception error) {

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
