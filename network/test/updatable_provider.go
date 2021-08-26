package test

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
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
