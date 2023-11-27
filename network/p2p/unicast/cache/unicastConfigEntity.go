package unicastcache

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

// UnicastConfigEntity is a struct that represents a unicast config entry for storing in the unicast config cache.
// It implements the flow.Entity interface.
type UnicastConfigEntity struct {
	unicast.Config
	PeerId peer.ID         // remote peer id; used as the "key" in the unicast config cache.
	id     flow.Identifier // cache the id for fast lookup (HeroCache).
}

var _ flow.Entity = (*UnicastConfigEntity)(nil)

// ID returns the ID of the unicast config entity; it is hash value of the peer id.
func (d UnicastConfigEntity) ID() flow.Identifier {
	if d.id == flow.ZeroID {
		d.id = PeerIdToFlowId(d.PeerId)
	}
	return d.id
}

// Checksum acts the same as ID.
func (d UnicastConfigEntity) Checksum() flow.Identifier {
	return d.ID()
}

// PeerIdToFlowId converts a peer id to a flow id (hash value of the peer id).
func PeerIdToFlowId(pid peer.ID) flow.Identifier {
	return flow.MakeIDFromFingerPrint([]byte(pid))
}
