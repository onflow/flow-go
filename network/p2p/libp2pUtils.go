package p2p

// All utilities for libp2p not natively provided by the library.

import (
	"fmt"
	"net"

	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

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
	streams := filterStream(host, targetID, protocol, network.DirOutbound, false)
	if len(streams) > 0 {
		return streams[0], true
	}
	return nil, false
}

// CountStream finds total number of outbound stream to the target id
func CountStream(host host.Host, targetID peer.ID, protocol core.ProtocolID, dir network.Direction) int {
	streams := filterStream(host, targetID, protocol, dir, true)
	return len(streams)
}

// filterStream finds one or all existing outbound streams to the target id if it exists.
// if parameter all is true - all streams are found else the first stream found is returned
func filterStream(host host.Host, targetID peer.ID, protocol core.ProtocolID, dir network.Direction, all bool) []network.Stream {

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

// PeerInfoFromID converts the flow.Identity to peer.AddrInfo.
// A node in flow is defined by a flow.Identity while it is defined by a peer.AddrInfo in libp2p.
// flow.Identity           ---> peer.AddrInfo
//    |-- Address          --->   |-- []multiaddr.Multiaddr
//    |-- NetworkPublicKey --->   |-- ID
func PeerInfoFromID(id flow.Identity) (peer.AddrInfo, error) {
	addr, err := PeerAddressInfo(id)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("failed to convert flow Identity %s to peer.AddrInfo: %w", id.String(), err)
	}

	return addr, nil
}

// NodeAddressFromIdentity returns the libp2p.NodeAddress for the given flow.identity
func NodeAddressFromIdentity(flowIdentity flow.Identity) (NodeAddress, error) {
	// split the node address into ip and port
	ip, port, err := net.SplitHostPort(flowIdentity.Address)
	if err != nil {
		return NodeAddress{}, fmt.Errorf("could not parse address %s: %w", flowIdentity.Address, err)
	}

	// convert the Flow key to a LibP2P key
	lkey, err := publicKey(flowIdentity.NetworkPubKey)
	if err != nil {
		return NodeAddress{}, fmt.Errorf("could not convert flow key to libp2p key: %w", err)
	}

	// create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowIdentity.NodeID.String(), IP: ip, Port: port, PubKey: lkey}

	return nodeAddress, nil
}

// networkingInfo returns ip, port, libp2p public key of the identity.
func networkingInfo(identity flow.Identity) (string, string, crypto.PubKey, error) {
	// split the node address into ip and port
	ip, port, err := net.SplitHostPort(identity.Address)
	if err != nil {
		return "", "", nil, fmt.Errorf("could not parse address %s: %w", identity.Address, err)
	}

	// convert the Flow key to a LibP2P key
	lkey, err := publicKey(identity.NetworkPubKey)
	if err != nil {
		return "", "", nil, fmt.Errorf("could not convert flow key to libp2p key: %w", err)
	}

	return ip, port, lkey, nil
}
