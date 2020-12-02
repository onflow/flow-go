package serializable

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

type dkg struct {
	size         uint
	groupKey     crypto.PublicKey
	participants map[flow.Identifier]flow.DKGParticipant
}

func (d *dkg) Size() uint {
	return d.size
}

func (d *dkg) GroupKey() crypto.PublicKey {
	return d.groupKey
}

func (d *dkg) Index(nodeID flow.Identifier) (uint, error) {
	return d.participants[nodeID].Index, nil
}

func (d *dkg) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	return d.participants[nodeID].KeyShare, nil
}
