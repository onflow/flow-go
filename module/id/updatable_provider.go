package id

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type UpdatableIDProvider struct {
	mu         sync.RWMutex
	identities flow.IdentityList
}

func NewUpdatableIDProvider(identities flow.IdentityList) *UpdatableIDProvider {
	return &UpdatableIDProvider{identities: identities}
}

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
