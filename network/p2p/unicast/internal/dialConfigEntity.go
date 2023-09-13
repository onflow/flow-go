package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// DialConfig is a struct that represents the dial config for a peer.
type DialConfig struct {
	DialBackoff        uint64 // number of times we have to try to dial the peer before we give up.
	StreamBackoff      uint64 // number of times we have to try to open a stream to the peer before we give up.
	LastSuccessfulDial uint64 // timestamp of the last successful dial to the peer.
}

// DialConfigEntity is a struct that represents a dial config entry for storing in the dial config cache.
// It implements the flow.Entity interface.
type DialConfigEntity struct {
	DialConfig
	PeerId peer.ID         // remote peer id; used as the "key" in the dial config cache.
	id     flow.Identifier // cache the id for fast lookup (HeroCache).
}

var _ flow.Entity = (*DialConfigEntity)(nil)

// ID returns the ID of the dial config entity; it is hash value of the peer id.
func (d DialConfigEntity) ID() flow.Identifier {
	if d.id == flow.ZeroID {
		d.id = PeerIdToFlowId(d.PeerId)
	}
	return d.id
}

// Checksum acts the same as ID.
func (d DialConfigEntity) Checksum() flow.Identifier {
	return d.ID()
}

// PeerIdToFlowId converts a peer id to a flow id (hash value of the peer id).
func PeerIdToFlowId(pid peer.ID) flow.Identifier {
	return flow.MakeIDFromFingerPrint([]byte(pid))
}

// DialConfigAdjustFunc is a function that is used to adjust the fields of a DialConfigEntity.
// The function is called with the current config and should return the adjusted record.
// Returned error indicates that the adjustment is not applied, and the config should not be updated.
// In BFT setup, the returned error should be treated as a fatal error.
type DialConfigAdjustFunc func(DialConfig) (DialConfig, error)
