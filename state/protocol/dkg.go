package protocol

import (
	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// DKG represents the result of running the distributed key generation
// procedure for the random beacon.
type DKG interface {

	// Size is the number of members in the DKG.
	Size() uint

	// GroupKey is the group public key.
	GroupKey() crypto.PublicKey

	// Index returns the index for the given node.
	// Error Returns:
	// * protocol.IdentityNotFoundError if nodeID is not a valid DKG participant.
	Index(nodeID flow.Identifier) (uint, error)

	// KeyShare returns the public key share for the given node.
	// Error Returns:
	// * protocol.IdentityNotFoundError if nodeID is not a valid DKG participant.
	KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error)
}
