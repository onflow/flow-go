package id

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/p2p"
)

// FixedIdentifierProvider implements an IdentifierProvider which provides a fixed list
// of identifiers.
type FixedIdentifierProvider struct {
	identifiers flow.IdentifierList
}

func NewFixedIdentifierProvider(identifiers flow.IdentifierList) *FixedIdentifierProvider {
	return &FixedIdentifierProvider{identifiers}
}

func (p *FixedIdentifierProvider) Identifiers() flow.IdentifierList {
	return p.identifiers
}

// FixedIdentityProvider implements an IdentityProvider which provides a fixed list
// of identities.
type FixedIdentityProvider struct {
	identities flow.IdentityList
}

func NewFixedIdentityProvider(identities flow.IdentityList) *FixedIdentityProvider {
	return &FixedIdentityProvider{identities}
}

func (p *FixedIdentityProvider) Identities(filter flow.IdentityFilter) flow.IdentityList {
	return p.identities.Filter(filter)
}

func (p *FixedIdentityProvider) ByNodeID(nodeID flow.Identifier) (*flow.Identity, bool) {
	results := p.identities.Filter(filter.HasNodeID(nodeID))
	if len(results) > 0 {
		return results[0], true
	}
	return nil, false
}

func (p *FixedIdentityProvider) ByPeerID(pid peer.ID) (*flow.Identity, bool) {
	for _, id := range p.identities {
		if peerID, err := p2p.PeerIDFromFlowPublicKey(id.NetworkPubKey); err == nil && peerID == pid {
			return id, true
		}
	}
	return nil, false
}
