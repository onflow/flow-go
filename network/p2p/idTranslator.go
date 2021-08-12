package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

type IDTranslator interface {
	GetPeerID(flow.Identifier) (peer.ID, bool)
	GetFlowID(peer.ID) (flow.Identifier, bool)
}

type UnstakedNetworkPeerIDProvider struct {
	// basically, just do the conversion from flow ID to peer ID using whatever scheme we talked about.
	// whether this be, with caching or not
}

func NewUnstakedNetworkPeerIDProvider() *UnstakedNetworkPeerIDProvider {
	return &UnstakedNetworkPeerIDProvider{}
}

func GetPeerID(flowID flow.Identifier) (peer.ID, bool) {
	Flow
	return
}

func GetFlowID(peerID peer.ID) (flow.Identifier, bool) {
	Flow
	return
}

func FlowIDToPeerID(flowID flow.Identifier) peer.ID {

}

func IdentityToPeerID(id *flow.Identity) (pid peer.ID, err error) {
	pk, err := PublicKey(id.NetworkPubKey)
	if err != nil {
		// TODO: format the error
		return
	}

	pid, err = peer.IDFromPublicKey(pk)
	if err != nil {
		// TODO: format the error
		return
	}

	return
}
