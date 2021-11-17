package unicast

import (
	"context"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"
)

type StreamFactory interface {
	SetStreamHandler(pid protocol.ID, handler network.StreamHandler)
	DialAddress(pid protocol.ID) []multiaddr.Multiaddr
	ClearBackoff(pid peer.ID)
	Connect(ctx context.Context, pi peer.AddrInfo) error
	NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error)
}
