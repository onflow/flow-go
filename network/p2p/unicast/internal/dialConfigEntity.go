package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

type DialConfigEntity struct {
	PeerId             peer.ID
	DialBackoff        uint64
	StreamBackoff      uint64
	LastSuccessfulDial uint64
	id                 flow.Identifier
}

var _ flow.Entity = (*DialConfigEntity)(nil)

func (d DialConfigEntity) ID() flow.Identifier {
	if d.id == flow.ZeroID {
		d.id = flow.MakeIDFromFingerPrint([]byte(d.PeerId))
	}
	return d.id
}

func (d DialConfigEntity) Checksum() flow.Identifier {
	return d.ID()
}
