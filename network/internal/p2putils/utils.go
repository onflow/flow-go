package p2putils

import (
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
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

// CountStream finds total number of outbound stream to the target id
func CountStream(host host.Host, targetID peer.ID, opts ...FilterOption) int {
	streams := FilterStream(host, targetID, append(opts, All())...)
	return len(streams)
}

// FilterOptions holds the filtering options used in FilterStream.
type FilterOptions struct {
	// dir specifies the direction of the streams to be filtered.
	// The default value is network.DirBoth, which considers both inbound and outbound streams.
	dir network.Direction

	// protocol specifies the protocol ID of the streams to be filtered.
	// The default value is an empty string, which considers streams of all protocol IDs.
	protocol protocol.ID

	// all specifies whether to return all matching streams or just the first matching stream.
	// The default value is false, which returns just the first matching stream.
	all bool
}

// FilterOption defines a function type that modifies FilterOptions.
type FilterOption func(*FilterOptions)

// Direction is a FilterOption for setting the direction of the streams to be filtered.
func Direction(dir network.Direction) FilterOption {
	return func(opts *FilterOptions) {
		opts.dir = dir
	}
}

// Protocol is a FilterOption for setting the protocol ID of the streams to be filtered.
func Protocol(protocol protocol.ID) FilterOption {
	return func(opts *FilterOptions) {
		opts.protocol = protocol
	}
}

// All is a FilterOption for setting whether to return all matching streams or just the first matching stream.
func All() FilterOption {
	return func(opts *FilterOptions) {
		opts.all = true
	}
}

// FilterStream filters the streams to a target peer based on the provided options.
// The default behavior is to consider all directions and protocols, and return just the first matching stream.
// This behavior can be customized by providing FilterOption values.
//
// Usage:
//
//   - To find all outbound streams to a target peer with a specific protocol ID:
//     streams := FilterStream(host, targetID, Direction(network.DirOutbound), Protocol(myProtocolID), All(true))
//
//   - To find the first inbound stream to a target peer, regardless of protocol ID:
//     stream := FilterStream(host, targetID, Direction(network.DirInbound))
//
// host is the host from which to filter streams.
// targetID is the ID of the target peer.
// options is a variadic parameter that allows zero or more FilterOption values to be provided.
//
// It returns a slice of network.Stream values that match the filtering criteria.
func FilterStream(host host.Host, targetID peer.ID, options ...FilterOption) []network.Stream {
	var filteredStreams []network.Stream
	const allProtocols = "*"
	// default values
	opts := FilterOptions{
		dir:      network.DirUnknown, // by default, consider both inbound and outbound streams
		protocol: allProtocols,       // by default, consider streams of all protocol IDs
		all:      false,              // by default, return just the first matching stream
	}

	// apply provided options
	for _, option := range options {
		option(&opts)
	}

	if host.Network().Connectedness(targetID) != network.Connected {
		return filteredStreams
	}

	conns := host.Network().ConnsToPeer(targetID)
	for _, conn := range conns {
		streams := conn.GetStreams()
		for _, stream := range streams {
			if (opts.dir == network.DirUnknown || stream.Stat().Direction == opts.dir) &&
				(opts.protocol == allProtocols || stream.Protocol() == opts.protocol) {
				filteredStreams = append(filteredStreams, stream)
				if !opts.all {
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

		// there should only be one valid IPv4 address
		return ipOrHostname, port, nil
	}
	return "", "", fmt.Errorf("ip address or hostname not found")
}
