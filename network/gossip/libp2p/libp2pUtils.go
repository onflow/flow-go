package libp2p

// All utilities for libp2p not natively provided by the library.

import (
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
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

// reverse translating libp2p network direction to string
func DirectionToString(direction network.Direction) string {
	if dirStr, found := directionLookUp[direction]; found {
		return dirStr
	} else {
		return "Not defined"
	}
}

// reverse translating libp2p network connectedness to string
func ConnectednessToString(connectedness network.Connectedness) string {
	if connStr, found := connectednessLookup[connectedness]; found {
		return connStr
	} else {
		return "Not defined"
	}
}

// FindOutboundStream finds an existing outbound stream to the target id if it exists, else it returns nil by querying the state of the libp2p host
func FindOutboundStream(host host.Host, targetID peer.ID, protocol core.ProtocolID, logger zerolog.Logger) network.Stream {

	// get all connections
	conns := host.Network().ConnsToPeer(targetID)

	// find a connection which is in the connected state
	for _, conn := range conns {

		// choose the connection only if it is connected
		if host.Network().Connectedness(targetID) == network.Connected {

			logger.Debug().
				Str("direction", DirectionToString(conn.Stat().Direction)).
				Msg("found existing connection")

			// get all streams
			streams := conn.GetStreams()
			for _, stream := range streams {

				logger.Debug().Str("protocol", string(stream.Protocol())).
					Str("direction", DirectionToString(stream.Stat().Direction)).
					Msg("found existing stream")

				// choose a stream which is marked as outbound and is for the flow protocol
				if stream.Stat().Direction == network.DirOutbound && stream.Protocol() == protocol {
					return stream
				}
			}
		}
	}
	return nil
}

func CountStream(host host.Host, targetID peer.ID, protocol core.ProtocolID, dir network.Direction) int {

	count := 0

	// get all connections
	conns := host.Network().ConnsToPeer(targetID)

	// find a connection which is in the connected state
	for _, conn := range conns {

		// choose the connection only if it is connected
		if host.Network().Connectedness(targetID) == network.Connected {

			// get all streams
			streams := conn.GetStreams()
			for _, stream := range streams {

				// choose a stream which is marked as outbound and is for the flow protocol
				if stream.Stat().Direction == dir && stream.Protocol() == protocol {
					count++
				}
			}
		}
	}
	return count
}
