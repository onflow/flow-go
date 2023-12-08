package signature

import (
	"fmt"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// RandomBeaconReconstructor implements hotstuff.RandomBeaconReconstructor.
// The implementation wraps the hotstuff.RandomBeaconInspector and translates the signer identity into signer index.
// It has knowledge about DKG to be able to map signerID to signerIndex
type RandomBeaconReconstructor struct {
	hotstuff.RandomBeaconInspector              // a stateful object for this epoch. It's used for both verifying all sig shares and reconstructing the threshold signature.
	dkg                            hotstuff.DKG // to lookup signer index by signer ID
}

var _ hotstuff.RandomBeaconReconstructor = (*RandomBeaconReconstructor)(nil)

func NewRandomBeaconReconstructor(dkg hotstuff.DKG, randomBeaconInspector hotstuff.RandomBeaconInspector) *RandomBeaconReconstructor {
	return &RandomBeaconReconstructor{
		RandomBeaconInspector: randomBeaconInspector,
		dkg:                   dkg,
	}
}

// Verify verifies the signature share under the signer's public key and the message agreed upon.
// The function is thread-safe and wait-free (i.e. allowing arbitrary many routines to
// execute the business logic, without interfering with each other).
// It allows concurrent verification of the given signature.
// Returns :
//   - model.InvalidSignerError if signerID is invalid
//   - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
//   - other error if there is an unexpected exception.
func (r *RandomBeaconReconstructor) Verify(signerID flow.Identifier, sig crypto.Signature) error {
	signerIndex, err := r.dkg.Index(signerID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return model.NewInvalidSignerErrorf("signer %v is not a valid random beacon participant: %w", signerID, err)
		}
		return fmt.Errorf("internal error retrieving DKG index for %v: %w", signerID, err)
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
//   - (true, nil) if the signature has been added, and enough shares have been collected.
//   - (false, nil) if the signature has been added, but not enough shares were collected.
//
// The following errors are expected during normal operations:
//   - model.InvalidSignerError if signerIndex is invalid (out of the valid range)
//   - model.DuplicatedSignerError if the signer has been already added
//   - other error if there is an unexpected exception.
func (r *RandomBeaconReconstructor) TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error) {
	signerIndex, err := r.dkg.Index(signerID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return false, model.NewInvalidSignerErrorf("signer %v is not a valid random beacon participant: %w", signerID, err)
		}
		return false, fmt.Errorf("internal error retrieving DKG index for %v: %w", signerID, err)
	}
	return r.RandomBeaconInspector.TrustedAdd(int(signerIndex), sig)
}
