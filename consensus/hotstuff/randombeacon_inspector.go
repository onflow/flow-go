package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
)

// RandomBeaconInspector encapsulates all methods needed by a Hotstuff leader to validate the
// beacon votes and reconstruct a beacon signature.
// The random beacon methods are based on a threshold signature scheme.
type RandomBeaconInspector interface {
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
	// The function errors if not enough shares were collected and if any signature
	// fails the deserialization.
	// It also performs a final verification against the stored message and group public key
	// and errors (without sentinel) if the result is not valid. This is required for the function safety since
	// `TrustedAdd` allows adding invalid signatures.
	// The function is blocking.
	Reconstruct() (crypto.Signature, error)
}
