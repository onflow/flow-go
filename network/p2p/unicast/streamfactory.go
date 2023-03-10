package unicast

import (
	"context"
	"errors"
	"strings"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/multiformats/go-multiaddr"
)

const (
	protocolNegotiationFailedStr = "failed to negotiate security protocol"
	protocolNotSupportedStr      = "protocol not supported"
)

// StreamFactory is a wrapper around libp2p host.Host to provide abstraction and encapsulation for unicast stream manager so that
// it can create libp2p streams with finer granularity.
type StreamFactory interface {
	SetStreamHandler(protocol.ID, network.StreamHandler)
	DialAddress(peer.ID) []multiaddr.Multiaddr
	ClearBackoff(peer.ID)
	// Connect connects host to peer with peerID.
	// Expected errors during normal operations:
	//   - NewSecurityProtocolNegotiationErr this indicates there was an issue upgrading the connection.
	Connect(context.Context, peer.AddrInfo) error
	// NewStream creates a new stream on the libp2p host.
	// Expected errors during normal operations:
	//   - ErrProtocolNotSupported this indicates remote node is running on a different spork.
	NewStream(context.Context, peer.ID, ...protocol.ID) (network.Stream, error)
}

type LibP2PStreamFactory struct {
	host host.Host
}

func NewLibP2PStreamFactory(h host.Host) StreamFactory {
	return &LibP2PStreamFactory{host: h}
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

// Connect connects host to peer with peerAddrInfo.
// Expected errors during normal operations:
//   - ErrSecurityProtocolNegotiationFailed this indicates there was an issue upgrading the connection.
func (l *LibP2PStreamFactory) Connect(ctx context.Context, peerAddrInfo peer.AddrInfo) error {
	err := l.host.Connect(ctx, peerAddrInfo)
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), protocolNegotiationFailedStr):
		return NewSecurityProtocolNegotiationErr(peerAddrInfo.ID, err)
	case errors.Is(err, swarm.ErrGaterDisallowedConnection):
		return NewGaterDisallowedConnectionErr(err)
	default:
		return err
	}
}

// NewStream creates a new stream on the libp2p host.
// Expected errors during normal operations:
//   - ErrProtocolNotSupported this indicates remote node is running on a different spork.
func (l *LibP2PStreamFactory) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
	s, err := l.host.NewStream(ctx, p, pids...)
	if err != nil {
		if strings.Contains(err.Error(), protocolNotSupportedStr) {
			return nil, NewProtocolNotSupportedErr(p, pids, err)
		}
		return nil, err
	}
	return s, err
}
