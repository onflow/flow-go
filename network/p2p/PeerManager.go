package p2p

import (
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

// PeerManagerFactoryFunc is a factory function type for generating a PeerManager instance using the given host,
// peersProvider and logger
type PeerManagerFactoryFunc func(host host.Host, peersProvider PeersProvider, logger zerolog.Logger) (PeerManager, error)

type PeersProvider func() peer.IDSlice

type PeerManager interface {
	module.ReadyDoneAware

	// RequestPeerUpdate requests an update to the peer connections of this node.
	// If a peer update has already been requested (either as a periodic request or an on-demand request) and is outstanding,
	// then this call is a no-op.
	RequestPeerUpdate()

	ForceUpdatePeers()
}
