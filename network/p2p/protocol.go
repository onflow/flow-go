package p2p

import (
	"strings"

	"github.com/libp2p/go-libp2p-core/network"
)

// Flow Libp2p protocols
const (

	// The common prefix for all Libp2p protocol IDs used for Flow
	// ALL Flow libp2p protocols must start with this prefix
	FlowLibP2PProtocolCommonPrefix = "/flow"

	// A unique Libp2p protocol ID prefix for Flow (https://docs.libp2p.io/concepts/protocols/)
	// All nodes communicate with each other using this protocol id suffixed with the id of the root block
	FlowLibP2POneToOneProtocolIDPrefix = FlowLibP2PProtocolCommonPrefix + "/push/"

	// the Flow Ping protocol prefix
	FlowLibP2PPingProtocolPrefix = FlowLibP2PProtocolCommonPrefix + "/ping/"

	// FlowLibP2PProtocolGzipCompressedOneToOne represents the protocol id for compressed streams under gzip compressor.
	FlowLibP2PProtocolGzipCompressedOneToOne = FlowLibP2POneToOneProtocolIDPrefix + "/gzip/"
)

// isFlowProtocolStream returns true if the libp2p stream is for a Flow protocol
func isFlowProtocolStream(stream network.Stream) bool {
	protocol := string(stream.Protocol())
	return strings.HasPrefix(protocol, FlowLibP2PProtocolCommonPrefix)
}
