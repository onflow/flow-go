package id

import (
	"github.com/onflow/flow-go/model/flow"
)

// IdentityFilterIdentifierProvider implements an IdentifierProvider which provides the identifiers
// resulting from applying a filter to an IdentityProvider.
type IdentityFilterIdentifierProvider struct {
	filter           flow.IdentityFilter
	identityProvider IdentityProvider
}

func NewIdentityFilterIdentifierProvider(filter flow.IdentityFilter, identityProvider IdentityProvider) *IdentityFilterIdentifierProvider {
	return &IdentityFilterIdentifierProvider{filter, identityProvider}
}

func (p *IdentityFilterIdentifierProvider) Identifiers() flow.IdentifierList {
	return p.identityProvider.Identities(p.filter).NodeIDs()
}

type FilteredIdentifierProvider struct {
	filter   flow.IdentifierFilter
	provider IdentifierProvider
}

func NewFilteredIdentifierProvider(filter flow.IdentifierFilter, provider IdentifierProvider) *FilteredIdentifierProvider {
	return &FilteredIdentifierProvider{filter, provider}
}

func (p *FilteredIdentifierProvider) Identifiers() flow.IdentifierList {
	return p.provider.Identifiers().Filter(p.filter)
}
