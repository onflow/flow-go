package test

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/keyutils"
)

// UpdatableIDProvider implements an IdentityProvider which can be manually updated by setting
// the IdentityList to a new value.
// It also implements an IdentifierProvider which provides the identifiers of the IdentityList.
// This is mainly used to simulate epoch transitions in tests.
type UpdatableIDProvider struct {
	mu         sync.RWMutex
	identities flow.IdentityList
}

func NewUpdatableIDProvider(identities flow.IdentityList) *UpdatableIDProvider {
	return &UpdatableIDProvider{identities: identities}
}

// SetIdentities updates the IdentityList returned by this provider.
func (p *UpdatableIDProvider) SetIdentities(identities flow.IdentityList) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.identities = identities
}

func (p *UpdatableIDProvider) Identities(filter flow.IdentityFilter) flow.IdentityList {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.identities.Filter(filter)
}

func (p *UpdatableIDProvider) Identifiers() flow.IdentifierList {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.identities.NodeIDs()
}

func (p *UpdatableIDProvider) ByNodeID(flowID flow.Identifier) (*flow.Identity, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, v := range p.identities {
		if v.ID() == flowID {
			return v, true
		}
	}
	return nil, false
}

func (p *UpdatableIDProvider) ByPeerID(peerID peer.ID) (*flow.Identity, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, v := range p.identities {
		if id, err := keyutils.PeerIDFromFlowPublicKey(v.NetworkPubKey); err == nil {
			if id == peerID {
				return v, true
			}
		}

	}
	return nil, false

}
