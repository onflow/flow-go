package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
)

// PeerManagerFactoryFunc is a factory function type for generating a PeerManager instance using the given host,
// peersProvider and logger
type PeerManagerFactoryFunc func(host host.Host, peersProvider PeersProvider, logger zerolog.Logger) (PeerManager, error)

type PeersProvider func() peer.IDSlice

// PeerManager adds and removes connections to peers periodically and on request
type PeerManager interface {
	component.Component
	RateLimiterConsumer

	// RequestPeerUpdate requests an update to the peer connections of this node.
	// If a peer update has already been requested (either as a periodic request or an on-demand
	// request) and is outstanding, then this call is a no-op.
	RequestPeerUpdate()

	// ForceUpdatePeers initiates an update to the peer connections of this node immediately
	ForceUpdatePeers(context.Context)

	// SetPeersProvider sets the peer managers's peers provider and may be called at most once
	SetPeersProvider(PeersProvider)
}
