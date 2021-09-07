package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// IDTranslator provides an interface for converting from Flow ID's to LibP2P peer ID's
// and vice versa.
type IDTranslator interface {
	// GetPeerID returns the peer ID for the given Flow ID
	GetPeerID(flow.Identifier) (peer.ID, error)

	// GetFlowID returns the Flow ID for the given peer ID
	GetFlowID(peer.ID) (flow.Identifier, error)
}
