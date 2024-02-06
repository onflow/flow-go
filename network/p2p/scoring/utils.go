package scoring

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// HasValidFlowIdentity checks if the peer has a valid Flow identity.
func HasValidFlowIdentity(idProvider module.IdentityProvider, pid peer.ID) (*flow.Identity, error) {
	flowId, ok := idProvider.ByPeerID(pid)
	if !ok {
		return nil, NewInvalidPeerIDError(pid, PeerIdStatusUnknown)
	}

	if flowId.IsEjected() {
		return nil, NewInvalidPeerIDError(pid, PeerIdStatusEjected)
	}

	return flowId, nil
}
