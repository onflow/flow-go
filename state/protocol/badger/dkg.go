package badger

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

type DKG struct {
	commitEvent *flow.EpochCommit
}

func (d *DKG) Size() uint {
	participants := d.commitEvent.DKGParticipants
	return uint(len(participants))
}

func (d *DKG) GroupKey() crypto.PublicKey {
	return d.commitEvent.DKGGroupKey
}

func (d *DKG) Index(nodeID flow.Identifier) (uint, error) {
	participant, found := d.commitEvent.DKGParticipants[nodeID]
	if !found {
		return 0, fmt.Errorf("could not find *DKG participant data (%x)", nodeID)
	}
	return participant.Index, nil
}

func (d *DKG) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	participant, found := d.commitEvent.DKGParticipants[nodeID]
	if !found {
		return nil, fmt.Errorf("could not find *DKG participant data (%x)", nodeID)
	}
	return participant.KeyShare, nil
}
