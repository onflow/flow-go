package p2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/id"
)

type IdentityProviderIdentityTranslator struct {
	idProvider id.IdentityProvider
}

func (t *IdentityProviderIdentityTranslator) GetFlowID(p peer.ID) (flow.Identifier, error) {
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

func (t *IdentityProviderIdentityTranslator) GetPeerID(n flow.Identifier) (peer.ID, error) {
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

func NewIdentityProviderIdentityTranslator(provider id.IdentityProvider) *IdentityProviderIdentityTranslator {
	return &IdentityProviderIdentityTranslator{provider}
}
