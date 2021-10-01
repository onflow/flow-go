package hotstuff

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding"
)

// RandomBeaconFollower encapsulates all methods needed by a Hotstuff leader to validate the
// beacon votes and reconstruct a beacon signature.
// The random beacon methods are based on a threshold signature scheme.
type RandomBeaconFollower interface {
	// Verify verifies the signature share under the signer's public key and the message agreed upon.
	// It allows concurrent verification of the given signature.
	// It returns :
	//  - engine.InvalidInputError if signerIndex is invalid
	//  - module/signature.ErrInvalidFormat if signerID is valid but signature is cryptographically invalid
	//  - other error if there is an exception.
	// The function call is non-blocking
	Verify(signerIndex int, share crypto.Signature) error

	// TrustedAdd adds a share to the internal signature shares store.
	// The function does not verify the signature is valid. It is the caller's responsibility
	// to make sure the signature was previously verified.
	// It returns:
	//  - (true, nil) if the signature has been added, and enough shares have been collected.
	//  - (false, nil) if the signature has been added, but not enough shares were collected.
	//  - (false, error) if there is any exception adding the signature share.
	//      - engine.InvalidInputError if signerIndex is invalid (not a consensus participant)
	//  	- engine.DuplicatedEntryError if the signer has been already added
	// The function call is blocking
	TrustedAdd(signerIndex int, share crypto.Signature) (enoughshares bool, exception error)

	// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
	// a group signature.
	EnoughShares() bool

	// Reconstruct reconstructs the group signature.
	// The reconstructed signature is verified against the overall group public key and the message agreed upon.
	// This is a sanity check that is necessary since "TrustedAdd" allows adding non-verified signatures.
	// Reconstruct returns an error if the reconstructed signature fails the sanity verification, or if not enough shares have been collected.
	Reconstruct() (crypto.Signature, error)
}
