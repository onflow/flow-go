package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// DialConfigEntity is a struct that represents a dial config entry for storing in the dial config cache.
// It implements the flow.Entity interface.
type DialConfigEntity struct {
	PeerId             peer.ID         // remote peer id; used as the "key" in the dial config cache.
	DialBackoff        uint64          // number of times we have to try to dial the peer before we give up.
	StreamBackoff      uint64          // number of times we have to try to open a stream to the peer before we give up.
	LastSuccessfulDial uint64          // timestamp of the last successful dial to the peer.
	id                 flow.Identifier // cache the id for fast lookup (HeroCache).
}

var _ flow.Entity = (*DialConfigEntity)(nil)

// ID returns the ID of the dial config entity; it is hash value of the peer id.
func (d DialConfigEntity) ID() flow.Identifier {
	if d.id == flow.ZeroID {
		d.id = flow.MakeIDFromFingerPrint([]byte(d.PeerId))
	}
	return d.id
}

// Checksum acts the same as ID.
func (d DialConfigEntity) Checksum() flow.Identifier {
	return d.ID()
}
