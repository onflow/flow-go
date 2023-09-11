package p2putils

import (
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/utils/p2plogging"
)

// FlowStream returns the Flow protocol Stream in the connection if one exist, else it returns nil
func FlowStream(conn network.Conn) network.Stream {
	for _, s := range conn.GetStreams() {
		if protocols.IsFlowProtocolStream(s) {
			return s
		}
	}
	return nil
}

// StreamLogger creates a logger for libp2p stream which logs the remote and local peer IDs and addresses
func StreamLogger(log zerolog.Logger, stream network.Stream) zerolog.Logger {
	logger := log.With().
		Str("protocol", string(stream.Protocol())).
		Str("remote_peer", p2plogging.PeerId(stream.Conn().RemotePeer())).
		Str("remote_address", stream.Conn().RemoteMultiaddr().String()).
		Str("local_peer", p2plogging.PeerId(stream.Conn().LocalPeer())).
		Str("local_address", stream.Conn().LocalMultiaddr().String()).Logger()
	return logger
}

var directionLookUp = map[network.Direction]string{
	network.DirInbound:  "InBound",
	network.DirOutbound: "OutBound",
	network.DirUnknown:  "Unknown",
}

var connectednessLookup = map[network.Connectedness]string{
	network.CannotConnect: "CannotConnect",
	network.CanConnect:    "CanConnect",
	network.Connected:     "Connected",
	network.NotConnected:  "NotConnected",
}

// DirectionToString reverse translates libp2p network direction to string
func DirectionToString(direction network.Direction) (string, bool) {
	if dirStr, found := directionLookUp[direction]; found {
		return dirStr, true
	}
	return "", false
}

// ConnectednessToString reverse translates libp2p network connectedness to string
func ConnectednessToString(connectedness network.Connectedness) (string, bool) {
	if connStr, found := connectednessLookup[connectedness]; found {
		return connStr, true
	}
	return "", false

}

// FindOutboundStream finds an existing outbound stream to the target id if it exists by querying libp2p
func FindOutboundStream(host host.Host, targetID peer.ID, protocol core.ProtocolID) (network.Stream, bool) {
	streams := FilterStream(host, targetID, protocol, network.DirOutbound, false)
	if len(streams) > 0 {
		return streams[0], true
	}
	return nil, false
}

// CountStream finds total number of outbound stream to the target id
func CountStream(host host.Host, targetID peer.ID, protocol core.ProtocolID, dir network.Direction) int {
	streams := FilterStream(host, targetID, protocol, dir, true)
	return len(streams)
}

// FilterStream finds one or all existing outbound streams to the target id if it exists.
// if parameter all is true - all streams are found else the first stream found is returned
func FilterStream(host host.Host, targetID peer.ID, protocol core.ProtocolID, dir network.Direction, all bool) []network.Stream {

	var filteredStreams []network.Stream

	// choose the connection only if it is connected
	if host.Network().Connectedness(targetID) != network.Connected {
		return filteredStreams
	}

	// get all connections
	conns := host.Network().ConnsToPeer(targetID)

	// find a connection which is in the connected state
	for _, conn := range conns {

		// get all streams
		streams := conn.GetStreams()
		for _, stream := range streams {

			// choose a stream which is marked as outbound and is for the flow protocol
			if stream.Stat().Direction == dir && stream.Protocol() == protocol {
				filteredStreams = append(filteredStreams, stream)
				if !all {
					return filteredStreams
				}
			}
		}
	}
	return filteredStreams
}

// NetworkingInfo returns ip, port, libp2p public key of the identity.
func NetworkingInfo(identity flow.Identity) (string, string, crypto.PubKey, error) {
	// split the node address into ip and port
	ip, port, err := net.SplitHostPort(identity.Address)
	if err != nil {
		return "", "", nil, fmt.Errorf("could not parse address %s: %w", identity.Address, err)
	}

	// convert the Flow key to a LibP2P key
	lkey, err := keyutils.LibP2PPublicKeyFromFlow(identity.NetworkPubKey)
	if err != nil {
		return "", "", nil, fmt.Errorf("could not convert flow key to libp2p key: %w", err)
	}

	return ip, port, lkey, nil
}

// IPPortFromMultiAddress returns the IP/hostname and the port for the given multi-addresses
// associated with a libp2p host
func IPPortFromMultiAddress(addrs ...multiaddr.Multiaddr) (string, string, error) {

	var ipOrHostname, port string
	var err error

	for _, a := range addrs {
		// try and get the dns4 hostname
		ipOrHostname, err = a.ValueForProtocol(multiaddr.P_DNS4)
		if err != nil {
			// if dns4 hostname is not found, try and get the IP address
			ipOrHostname, err = a.ValueForProtocol(multiaddr.P_IP4)
			if err != nil {
				continue // this may not be a TCP IP multiaddress
			}
		}

		// if either IP address or hostname is found, look for the port number
		port, err = a.ValueForProtocol(multiaddr.P_TCP)
		if err != nil {
			// an IPv4 or DNS4 based multiaddress should have a port number
			return "", "", err
		}

		//there should only be one valid IPv4 address
		return ipOrHostname, port, nil
	}
	return "", "", fmt.Errorf("ip address or hostname not found")
}
