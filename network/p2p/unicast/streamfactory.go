package unicast

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/multiformats/go-multiaddr"
)

type StreamFactory interface {
	SetStreamHandler(pid protocol.ID, handler network.StreamHandler)
	DialAddress(pid protocol.ID) []multiaddr.Multiaddr
	ClearBackoff(pid peer.ID)
	Connect(ctx context.Context, pi peer.AddrInfo) error
	NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error)
}

type LibP2PStreamFactory struct {
	host host.Host
}

func (l *LibP2PStreamFactory) SetStreamHandler(pid protocol.ID, handler network.StreamHandler) {
	l.host.SetStreamHandler(pid, handler)
}

func (l *LibP2PStreamFactory) DialAddress(p peer.ID) []multiaddr.Multiaddr {
	return l.host.Peerstore().Addrs(p)
}

func (l *LibP2PStreamFactory) ClearBackoff(p peer.ID) {
	if swm, ok := l.host.Network().(*swarm.Swarm); ok {
		swm.Backoff().Clear(p)
	}
}

func (l *LibP2PStreamFactory) Connect(ctx context.Context, pid peer.AddrInfo) error {
	return l.host.Connect(ctx, pid)
}

func (l *LibP2PStreamFactory) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
	return l.host.NewStream(ctx, p, pids...)
}
