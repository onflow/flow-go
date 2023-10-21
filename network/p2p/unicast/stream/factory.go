package stream

import (
	"context"
	"errors"
	"strings"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"

	"github.com/onflow/flow-go/network/p2p"
)

const (
	protocolNegotiationFailedStr = "failed to negotiate security protocol"
	protocolNotSupportedStr      = "protocol not supported"
)

type LibP2PStreamFactory struct {
	host host.Host
}

var _ p2p.StreamFactory = (*LibP2PStreamFactory)(nil)

func NewLibP2PStreamFactory(h host.Host) p2p.StreamFactory {
	return &LibP2PStreamFactory{host: h}
}

func (l *LibP2PStreamFactory) SetStreamHandler(pid protocol.ID, handler network.StreamHandler) {
	l.host.SetStreamHandler(pid, handler)
}

// Connect connects host to peer with peerAddrInfo.
// All errors returned from this function can be considered benign. We expect the following errors during normal operations:
//   - ErrSecurityProtocolNegotiationFailed this indicates there was an issue upgrading the connection.
//   - ErrGaterDisallowedConnection this indicates the connection was disallowed by the gater.
//   - There may be other unexpected errors from libp2p but they should be considered benign.
func (l *LibP2PStreamFactory) Connect(ctx context.Context, peerAddrInfo peer.AddrInfo) error {
	// libp2p internally uses swarm dial - https://github.com/libp2p/go-libp2p-swarm/blob/master/swarm_dial.go
	// to connect to a peer. Swarm dial adds a back off each time it fails connecting to a peer. While this is
	// the desired behaviour for pub-sub (1-k style of communication) for 1-1 style we want to retry the connection
	// immediately without backing off and fail-fast.
	// Hence, explicitly cancel the dial back off (if any) and try connecting again
	if swm, ok := l.host.Network().(*swarm.Swarm); ok {
		swm.Backoff().Clear(peerAddrInfo.ID)
	}

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
