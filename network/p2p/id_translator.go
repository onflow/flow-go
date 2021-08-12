package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

type IDTranslator interface {
	GetPeerID(flow.Identifier) (peer.ID, error)
	GetFlowID(peer.ID) (flow.Identifier, error)
}
