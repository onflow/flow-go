package p2p

import (
	"context"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
)

type UnicastManager interface {
	WithDefaultHandler(defaultHandler libp2pnet.StreamHandler)
	Register(unicast protocols.ProtocolName) error
	CreateStream(ctx context.Context, peerID peer.ID, maxAttempts int) (libp2pnet.Stream, []multiaddr.Multiaddr, error)
}
