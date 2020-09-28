package protocol

import (
	"github.com/onflow/flow-go/crypto"
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
	Index(nodeID flow.Identifier) (uint, error)

	// KeyShare returns the public key share for the given node.
	KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error)
}
