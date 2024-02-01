package inmem

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type DKG struct {
	enc EncodableDKG
}

var _ protocol.DKG = (*DKG)(nil)

func NewDKG(enc EncodableDKG) *DKG {
	return &DKG{enc: enc}
}

func (d DKG) Size() uint                 { return uint(len(d.enc.Participants)) }
func (d DKG) GroupKey() crypto.PublicKey { return d.enc.GroupKey.PublicKey }

// Index returns the index for the given node. Error Returns:
// protocol.IdentityNotFoundError if nodeID is not a valid DKG participant.
func (d DKG) Index(nodeID flow.Identifier) (uint, error) {
	part, exists := d.enc.Participants[nodeID]
	if !exists {
		return 0, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return part.Index, nil
}

// KeyShare returns the public key share for the given node. Error Returns:
// protocol.IdentityNotFoundError if nodeID is not a valid DKG participant.
func (d DKG) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	part, exists := d.enc.Participants[nodeID]
	if !exists {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return part.KeyShare, nil
}
