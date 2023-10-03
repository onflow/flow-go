package id

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// IdentityFilterIdentifierProvider implements an IdentifierProvider which provides the identifiers
// resulting from applying a filter to an IdentityProvider.
type IdentityFilterIdentifierProvider struct {
	filter           flow.IdentityFilter[flow.Identity]
	identityProvider module.IdentityProvider
}

func NewIdentityFilterIdentifierProvider(filter flow.IdentityFilter[flow.Identity], identityProvider module.IdentityProvider) *IdentityFilterIdentifierProvider {
	return &IdentityFilterIdentifierProvider{filter, identityProvider}
}

func (p *IdentityFilterIdentifierProvider) Identifiers() flow.IdentifierList {
	return p.identityProvider.Identities(p.filter).NodeIDs()
}
