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

// NewStream establishes a new stream with the given peer using the provided protocol.ID on the libp2p host.
// This function is a critical part of the network communication, facilitating the creation of a dedicated
// bidirectional channel (stream) between two nodes in the network.
// If there exists no connection between the two nodes, the function attempts to establish one before creating the stream.
// If there are multiple connections between the two nodes, the function selects the best one (based on libp2p internal criteria) to create the stream.
//
// Usage:
// The function is intended to be used when there is a need to initiate a direct communication stream with a peer.
// It is typically invoked in scenarios where a node wants to send a message or start a series of messages to another
// node using a specific protocol. The protocol ID is used to ensure that both nodes communicate over the same
// protocol, which defines the structure and semantics of the communication.
//
// Expected errors:
// During normal operation, the function may encounter specific expected errors, which are handled as follows:
//
//   - ErrProtocolNotSupported: This error occurs when the remote node does not support the specified protocol ID,
//     which may indicate that the remote node is running a different version of the software or a different spork.
//     The error contains details about the peer ID and the unsupported protocol, and it is generated when the
//     underlying error message indicates a protocol mismatch. This is a critical error as it signifies that the
//     two nodes cannot communicate using the requested protocol, and it must be handled by either retrying with
//     a different protocol ID or by performing some form of negotiation or fallback.
//
//   - Any other error returned by the libp2p host: This error indicates that the stream creation failed due to
//     some unexpected error, which may be caused by a variety of reasons. This is NOT a critical error, and it
//     can be handled by retrying the stream creation or by performing some other action. Crashing node upon this
//     error is NOT recommended.
//
// Arguments:
//   - ctx: A context.Context that governs the lifetime of the stream creation. It can be used to cancel the
//     operation or to set deadlines.
//   - p: The peer.ID of the target node with which the stream is to be established.
//   - pid: The protocol.ID that specifies the communication protocol to be used for the stream.
//
// Returns:
//   - network.Stream: The successfully created stream, ready for reading and writing, or nil if an error occurs.
//   - error: An error encountered during stream creation, wrapped in a contextually appropriate error type when necessary,
//     or nil if the operation is successful.
func (l *LibP2PStreamFactory) NewStream(ctx context.Context, p peer.ID, pid protocol.ID) (network.Stream, error) {
	s, err := l.host.NewStream(ctx, p, pid)
	if err != nil {
		if strings.Contains(err.Error(), protocolNotSupportedStr) {
			return nil, NewProtocolNotSupportedErr(p, pid, err)
		}
		return nil, err
	}
	return s, err
}
