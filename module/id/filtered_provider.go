package id

import (
	"github.com/onflow/flow-go/model/flow"
)

// FilteredIdentifierProvider implements an IdentifierProvider which provides the identifiers
// resulting from applying a filter to an IdentityProvider.
type FilteredIdentifierProvider struct {
	filter           flow.IdentityFilter
	identityProvider IdentityProvider
}

func NewFilteredIdentifierProvider(filter flow.IdentityFilter, identityProvider IdentityProvider) *FilteredIdentifierProvider {
	return &FilteredIdentifierProvider{filter, identityProvider}
}

func (p *FilteredIdentifierProvider) Identifiers() flow.IdentifierList {
	return p.identityProvider.Identities(p.filter).NodeIDs()
}
