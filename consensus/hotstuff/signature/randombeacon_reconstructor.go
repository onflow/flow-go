package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
)

// RandomBeaconReconstructor implements hotstuff.RandomBeaconReconstructor.
// The implementation wraps the hotstuff.RandomBeaconInspector and translates the signer identity into signer index.
// It has knowledge about DKG to be able to map signerID to signerIndex
type RandomBeaconReconstructor struct {
	hotstuff.RandomBeaconInspector              // a stateful object for this block. It's used for both storing all sig shares and producing the node's own share by signing the block
	dkg                            hotstuff.DKG // to lookup signer index by signer ID
}

var _ hotstuff.RandomBeaconReconstructor = &RandomBeaconReconstructor{}

// NewRandomBeaconReconstructor performs initialization of RandomBeaconInspector and creates RandomBeaconReconstructor as result.
// This constructor accepts next arguments:
// - committee of all hotstuff participants
// - msg that will be used for threshold signature
// - information about DKG at proposed block
// Under normal operations next return values are expected:
// - (nil, engine.InvalidInputError) - in case of invalid arguments that result in invalid configuration of RandomBeaconInspector
// - (nil, protocol.IdentityNotFoundError) - in case DKG key share is missing
// - (nil, exception) if any unexpected error happened
// - (*RandomBeaconReconstructor, nil) - in case of success.
func NewRandomBeaconReconstructor(allParticipants flow.IdentityList, msg []byte, dkg hotstuff.DKG) (*RandomBeaconReconstructor, error) {
	publicKeyShares := make([]crypto.PublicKey, 0, len(allParticipants))

	for _, participant := range allParticipants {
		pk, err := dkg.KeyShare(participant.NodeID)
		if err != nil {
			return nil, fmt.Errorf("could not get random beacon key share for %x: %w", participant.NodeID, err)
		}
		publicKeyShares = append(publicKeyShares, pk)
	}

	threshold := msig.RandomBeaconThreshold(int(dkg.Size()))
	inspector, err := NewRandomBeaconInspector(dkg.GroupKey(), publicKeyShares, threshold, msg)
	if err != nil {
		return nil, fmt.Errorf("could not create random beacon inspector: %w", err)
	}

	return &RandomBeaconReconstructor{
		RandomBeaconInspector: inspector,
		dkg:                   dkg,
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
func (r *RandomBeaconReconstructor) Verify(signerID flow.Identifier, sig crypto.Signature) error {
	signerIndex, err := r.dkg.Index(signerID)
	if err != nil {
		return fmt.Errorf("could not map signerID %v to signerIndex: %w", signerID, err)
	}
	return r.RandomBeaconInspector.Verify(int(signerIndex), sig)
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
	return r.RandomBeaconInspector.TrustedAdd(int(signerIndex), sig)
}
