package unicast

import (
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/compressed"
)

const GzipCompressionUnicast = ProtocolName("gzip-compression")

// GzipStream is a stream compression creates and returns a gzip-compressed stream out of input stream.
type GzipStream struct {
	protocolId     protocol.ID
	defaultHandler libp2pnet.StreamHandler
}

func NewGzipCompressedUnicast(rootBlockID flow.Identifier, defaultHandler libp2pnet.StreamHandler) *GzipStream {
	return &GzipStream{
		protocolId:     p2p.GzipCompressedProtocolId(rootBlockID),
		defaultHandler: defaultHandler,
	}
}

func (g GzipStream) NewStream(s libp2pnet.Stream) (libp2pnet.Stream, error) {
	return compressed.NewCompressedStream(s, compressor.GzipStreamCompressor{})
}

func (g GzipStream) Handler() libp2pnet.StreamHandler {
	return g.handler
}

func (g GzipStream) ProtocolId() protocol.ID {
	return g.protocolId
}
