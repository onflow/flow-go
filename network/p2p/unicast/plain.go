package unicast

import (
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

// PlainStream is a stream factory that reflects the same input stream without any modification.
type PlainStream struct {
	handler    libp2pnet.StreamHandler
	protocolId protocol.ID
}

func (p PlainStream) NewStream(s libp2pnet.Stream) (libp2pnet.Stream, error) {
	return s, nil
}

func (p PlainStream) Handler() libp2pnet.StreamHandler {
	return p.handler
}

func (p PlainStream) ProtocolId() protocol.ID {
	return p.protocolId
}
