package p2putils

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/node"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/p2p/unicast"
)

// FlowStream returns the Flow protocol Stream in the connection if one exist, else it returns nil
func FlowStream(conn network.Conn) network.Stream {
	for _, s := range conn.GetStreams() {
		if unicast.IsFlowProtocolStream(s) {
			return s
		}
	}
	return nil
}

// StreamLogger creates a logger for libp2p stream which logs the remote and local peer IDs and addresses
func StreamLogger(log zerolog.Logger, stream network.Stream) zerolog.Logger {
	logger := log.With().
		Str("protocol", string(stream.Protocol())).
		Str("remote_peer", stream.Conn().RemotePeer().String()).
		Str("remote_address", stream.Conn().RemoteMultiaddr().String()).
		Str("local_peer", stream.Conn().LocalPeer().String()).
		Str("local_address", stream.Conn().LocalMultiaddr().String()).Logger()
	return logger
}

// PeerAddressInfo generates the libp2p peer.AddrInfo for the given Flow.Identity.
// A node in flow is defined by a flow.Identity while it is defined by a peer.AddrInfo in libp2p.
//
//	flow.Identity        ---> peer.AddrInfo
//	|-- Address          --->   |-- []multiaddr.Multiaddr
//	|-- NetworkPublicKey --->   |-- ID
func PeerAddressInfo(identity flow.Identity) (peer.AddrInfo, error) {
	ip, port, key, err := node.NetworkingInfo(identity)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("could not translate identity to networking info %s: %w", identity.NodeID.String(), err)
	}

	addr := node.MultiAddressStr(ip, port)
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return peer.AddrInfo{}, err
	}

	id, err := peer.IDFromPublicKey(key)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("could not extract libp2p id from key:%w", err)
	}
	pInfo := peer.AddrInfo{ID: id, Addrs: []multiaddr.Multiaddr{maddr}}
	return pInfo, err
}
