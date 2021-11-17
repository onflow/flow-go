package unicast

import (
	"fmt"

	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

type ProtocolName string
type ProtocolFactory func(zerolog.Logger, flow.Identifier, libp2pnet.StreamHandler) Protocol

func ToProtocolFactory(name ProtocolName) (ProtocolFactory, error) {
	switch name {
	case GzipCompressionUnicast:
		return func(logger zerolog.Logger, rootBlockID flow.Identifier, handler libp2pnet.StreamHandler) Protocol {
			return NewGzipCompressedUnicast(logger, rootBlockID, handler)
		}, nil
	default:
		return nil, fmt.Errorf("unknown unicast protocol name: %s", name)
	}
}

type Protocol interface {
	NewStream(s libp2pnet.Stream) (libp2pnet.Stream, error)
	Handler() libp2pnet.StreamHandler
	ProtocolId() protocol.ID
}
