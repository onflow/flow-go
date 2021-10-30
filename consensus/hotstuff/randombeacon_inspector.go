package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
)

// RandomBeaconInspector encapsulates all methods needed by a Hotstuff leader to validate the
// beacon votes and reconstruct a beacon signature.
// The random beacon methods are based on a threshold signature scheme.
type RandomBeaconInspector interface {
	// Verify verifies the signature share under the signer's public key and the message agreed upon.
	// The function is thread-safe and wait-free (i.e. allowing arbitrary many routines to
	// execute the business logic, without interfering with each other).
	// It allows concurrent verification of the given signature.
	// Returns :
	//  - engine.InvalidInputError if signerIndex is invalid
	//  - module/signature.ErrInvalidFormat if signerID is valid but signature is cryptographically invalid
	//  - other error if there is an unexpected exception.
	Verify(signerIndex int, share crypto.Signature) error

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
	//      - engine.InvalidInputError if signerIndex is invalid
	//  	- engine.DuplicatedEntryError if the signer has been already added
	//      - other error if there is an unexpected exception.
	TrustedAdd(signerIndex int, share crypto.Signature) (enoughshares bool, exception error)

	// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
	// a group signature. The function is thread-safe.
	EnoughShares() bool

	// Reconstruct reconstructs the group signature. The function is thread-safe but locks
	// its internal state, thereby permitting only one routine at a time.
	// The function errors (without sentinel) in any of the following cases:
	//  - Not enough shares were collected.
	//  - Any of the added signatures fails the deserialization.
	//  - The reconstructed group signature is invalid. This post-verification is required
	//	  for safety, as `TrustedAdd` allows adding invalid signatures.
	Reconstruct() (crypto.Signature, error)
}
