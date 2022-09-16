package p2putils

import (
	"github.com/libp2p/go-libp2p/core/network"

	"github.com/onflow/flow-go/network/p2p/unicast"
)

// FlowStream returns the Flow protocol Stream in the connection if one exist, else it returns nil
func FlowStream(conn network.Conn) network.Stream {
	for _, s := range conn.GetStreams() {
		if unicast.IsFlowProtocolStream(s) {
			return s
		}
	}
	return nil
}
