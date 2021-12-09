package unicast

import (
	"fmt"
	"strings"

	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

// Flow Libp2p protocols
const (
	// FlowLibP2PProtocolCommonPrefix is the common prefix for all Libp2p protocol IDs used for Flow
	// ALL Flow libp2p protocols must start with this prefix.
	FlowLibP2PProtocolCommonPrefix = "/flow"

	FlowDHTProtocolIDPrefix = FlowLibP2PProtocolCommonPrefix + "/dht/"

	// FlowLibP2POneToOneProtocolIDPrefix is a unique Libp2p protocol ID prefix for Flow (https://docs.libp2p.io/concepts/protocols/)
	// All nodes communicate with each other using this protocol id suffixed with the id of the root block
	FlowLibP2POneToOneProtocolIDPrefix = FlowLibP2PProtocolCommonPrefix + "/push/"

	// FlowLibP2PPingProtocolPrefix is the Flow Ping protocol prefix
	FlowLibP2PPingProtocolPrefix = FlowLibP2PProtocolCommonPrefix + "/ping/"

	// FlowLibP2PProtocolGzipCompressedOneToOne represents the protocol id for compressed streams under gzip compressor.
	FlowLibP2PProtocolGzipCompressedOneToOne = FlowLibP2POneToOneProtocolIDPrefix + "/gzip/"
)

// IsFlowProtocolStream returns true if the libp2p stream is for a Flow protocol
func IsFlowProtocolStream(s libp2pnet.Stream) bool {
	p := string(s.Protocol())
	return strings.HasPrefix(p, FlowLibP2PProtocolCommonPrefix)
}

func FlowDHTProtocolID(sporkId flow.Identifier) protocol.ID {
	return protocol.ID(FlowDHTProtocolIDPrefix + sporkId.String())
}

func FlowPublicDHTProtocolID(sporkId flow.Identifier) protocol.ID {
	return protocol.ID(FlowDHTProtocolIDPrefix + "public/" + sporkId.String())
}

func FlowProtocolID(sporkId flow.Identifier) protocol.ID {
	return protocol.ID(FlowLibP2POneToOneProtocolIDPrefix + sporkId.String())
}

func PingProtocolId(sporkId flow.Identifier) protocol.ID {
	return protocol.ID(FlowLibP2PPingProtocolPrefix + sporkId.String())
}

type ProtocolName string
type ProtocolFactory func(zerolog.Logger, flow.Identifier, libp2pnet.StreamHandler) Protocol

func ToProtocolNames(names []string) []ProtocolName {
	p := make([]ProtocolName, 0)
	for _, name := range names {
		p = append(p, ProtocolName(name))
	}
	return p
}

func ToProtocolFactory(name ProtocolName) (ProtocolFactory, error) {
	switch name {
	case GzipCompressionUnicast:
		return func(logger zerolog.Logger, sporkId flow.Identifier, handler libp2pnet.StreamHandler) Protocol {
			return NewGzipCompressedUnicast(logger, sporkId, handler)
		}, nil
	default:
		return nil, fmt.Errorf("unknown unicast protocol name: %s", name)
	}
}

// Protocol represents a unicast protocol.
type Protocol interface {
	NewStream(s libp2pnet.Stream) (libp2pnet.Stream, error)
	Handler() libp2pnet.StreamHandler
	ProtocolId() protocol.ID
}
