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

// UpgradeRawStream implements protocol interface and returns the input stream without any modification.
func (p PlainStream) UpgradeRawStream(s libp2pnet.Stream) (libp2pnet.Stream, error) {
	return s, nil
}

func (p PlainStream) Handler(s libp2pnet.Stream) {
	p.handler(s)
}

func (p PlainStream) ProtocolId() protocol.ID {
	return p.protocolId
}
