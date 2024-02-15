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
	PeerId   peer.ID         // remote peer id; used as the "key" in the unicast config cache.
	EntityId flow.Identifier // cache the id for fast lookup (HeroCache).
}

var _ flow.Entity = (*UnicastConfigEntity)(nil)

// ID returns the ID of the unicast config entity; it is hash value of the peer id.
func (d UnicastConfigEntity) ID() flow.Identifier {
	return d.EntityId
}

// Checksum acts the same as ID.
func (d UnicastConfigEntity) Checksum() flow.Identifier {
	return d.ID()
}

// entityIdOf converts a peer ID to a flow ID by taking the hash of the peer ID.
// This is used to convert the peer ID in a notion that is compatible with HeroCache.
// This is not a protocol-level conversion, and is only used internally by the cache, MUST NOT be exposed outside the cache.
// Args:
// - peerId: the peer ID of the peer in the GossipSub protocol.
// Returns:
// - flow.Identifier: the flow ID of the peer.
func entityIdOf(pid peer.ID) flow.Identifier {
	return flow.MakeID(pid)
}
