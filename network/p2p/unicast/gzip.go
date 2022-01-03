package unicast

import (
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/p2p/compressed"
)

const GzipCompressionUnicast = ProtocolName("gzip-compression")

func FlowGzipProtocolId(sporkId flow.Identifier) protocol.ID {
	return protocol.ID(FlowLibP2PProtocolGzipCompressedOneToOne + sporkId.String())
}

// GzipStream is a stream compression creates and returns a gzip-compressed stream out of input stream.
type GzipStream struct {
	protocolId     protocol.ID
	defaultHandler libp2pnet.StreamHandler
	logger         zerolog.Logger
}

func NewGzipCompressedUnicast(logger zerolog.Logger, sporkId flow.Identifier, defaultHandler libp2pnet.StreamHandler) *GzipStream {
	return &GzipStream{
		protocolId:     FlowGzipProtocolId(sporkId),
		defaultHandler: defaultHandler,
		logger:         logger.With().Str("subsystem", "gzip-unicast").Logger(),
	}
}

func (g GzipStream) NewStream(s libp2pnet.Stream) (libp2pnet.Stream, error) {
	return compressed.NewCompressedStream(s, compressor.GzipStreamCompressor{})
}

func (g GzipStream) Handler() libp2pnet.StreamHandler {
	return func(s libp2pnet.Stream) {
		// converts native libp2p stream to gzip-compressed stream
		s, err := g.NewStream(s)
		if err != nil {
			g.logger.Error().Err(err).Msg("could not create compressed stream")
			return
		}
		g.defaultHandler(s)
	}
}

func (g GzipStream) ProtocolId() protocol.ID {
	return g.protocolId
}
