package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructor implements hotstuff.RandomBeaconReconstructor.
// The implementation wraps the hotstuff.RandomBeaconInspector and translates the signer identity into signer index.
// It has knowledge about DKG to be able to map signerID to signerIndex
type RandomBeaconReconstructor struct {
	dkg                   hotstuff.DKG                   // to lookup signer index by signer ID
	randomBeaconInspector hotstuff.RandomBeaconInspector // a stateful object for this block. It's used for both storing all sig shares and producing the node's own share by signing the block
}

var _ hotstuff.RandomBeaconReconstructor = &RandomBeaconReconstructor{}

func NewRandomBeaconReconstructor(dkg hotstuff.DKG, randomBeaconInspector hotstuff.RandomBeaconInspector) *RandomBeaconReconstructor {
	return &RandomBeaconReconstructor{
		dkg:                   dkg,
		randomBeaconInspector: randomBeaconInspector,
	}
}

// Verify verifies the signature share under the signer's public key and the message agreed upon.
// The function is thread-safe and wait-free (i.e. allowing arbitrary many routines to
// execute the business logic, without interfering with each other).
// It allows concurrent verification of the given signature.
// Returns :
//  - engine.InvalidInputError if signerIndex is invalid
//  - module/signature.ErrInvalidFormat if signerID is valid but signature is cryptographically invalid
//  - other error if there is an unexpected exception.
func (r *RandomBeaconReconstructor) Verify(signerID flow.Identifier, sig crypto.Signature) error {
	signerIndex, err := r.dkg.Index(signerID)
	if err != nil {
		return fmt.Errorf("could not map signerID %v to signerIndex: %w", signerID, err)
	}
	return r.randomBeaconInspector.Verify(int(signerIndex), sig)
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
//      - engine.InvalidInputError if signerIndex is invalid
//  	- engine.DuplicatedEntryError if the signer has been already added
//      - other error if there is an unexpected exception.
func (r *RandomBeaconReconstructor) TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error) {
	signerIndex, err := r.dkg.Index(signerID)
	if err != nil {
		return false, fmt.Errorf("could not map signerID %v to signerIndex: %w", signerID, err)
	}
	return r.randomBeaconInspector.TrustedAdd(int(signerIndex), sig)
}

// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
// a group signature. The function is thread-safe.
func (r *RandomBeaconReconstructor) EnoughShares() bool {
	return r.randomBeaconInspector.EnoughShares()
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
func (r *RandomBeaconReconstructor) Reconstruct() (crypto.Signature, error) {
	return r.randomBeaconInspector.Reconstruct()
}
