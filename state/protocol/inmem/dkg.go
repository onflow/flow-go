package inmem

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type DKG struct {
	encodable.DKG
}

func (d DKG) Size() uint                 { return d.DKG.Size }
func (d DKG) GroupKey() crypto.PublicKey { return d.DKG.GroupKey.PublicKey }

func (d DKG) Index(nodeID flow.Identifier) (uint, error) {
	part, exists := d.DKG.Participants[nodeID]
	if !exists {
		return 0, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return part.Index, nil
}

func (d DKG) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	part, exists := d.DKG.Participants[nodeID]
	if !exists {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return part.KeyShare, nil
}
