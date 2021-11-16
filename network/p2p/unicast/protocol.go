package unicast

import (
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

type ProtocolName string

type Protocol interface {
	NewStream(s libp2pnet.Stream) (libp2pnet.Stream, error)
	Handler() libp2pnet.StreamHandler
	ProtocolId() protocol.ID
}
