package p2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/id"
)

// IdentityProviderIDTranslator implements an IDTranslator which provides ID
// translation capabilities for an IdentityProvider.
type IdentityProviderIDTranslator struct {
	idProvider id.IdentityProvider
}

func (t *IdentityProviderIDTranslator) GetFlowID(p peer.ID) (flow.Identifier, error) {
	key, err := p.ExtractPublicKey()
	if err != nil {
		return flow.ZeroID, err
	}
	flowKey, err := FlowPublicKeyFromLibP2P(key)
	if err != nil {
		return flow.ZeroID, err
	}
	ids := t.idProvider.Identities(filter.HasNetworkingKey(flowKey))
	if len(ids) == 0 {
		return flow.ZeroID, fmt.Errorf("could not find identity corresponding to peer id %v", p.Pretty())
	}
	return ids[0].NodeID, nil
}

func (t *IdentityProviderIDTranslator) GetPeerID(n flow.Identifier) (peer.ID, error) {
	ids := t.idProvider.Identities(filter.HasNodeID(n))
	if len(ids) == 0 {
		return "", fmt.Errorf("could not find identity with id %v", n.String())
	}
	key, err := LibP2PPublicKeyFromFlow(ids[0].NetworkPubKey)
	if err != nil {
		return "", err
	}
	pid, err := peer.IDFromPublicKey(key)
	if err != nil {
		return "", err
	}
	return pid, nil
}

func NewIdentityProviderIDTranslator(provider id.IdentityProvider) *IdentityProviderIDTranslator {
	return &IdentityProviderIDTranslator{provider}
}
