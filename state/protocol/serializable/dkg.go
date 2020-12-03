package serializable

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type dkg struct {
	encodable.DKG
}

func (d dkg) Size() uint                 { return d.DKG.Size }
func (d dkg) GroupKey() crypto.PublicKey { return d.DKG.GroupKey.PublicKey }

func (d dkg) Index(nodeID flow.Identifier) (uint, error) {
	part, exists := d.DKG.Participants[nodeID]
	if !exists {
		return 0, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return part.Index, nil
}

func (d dkg) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	part, exists := d.DKG.Participants[nodeID]
	if !exists {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return part.KeyShare, nil
}
