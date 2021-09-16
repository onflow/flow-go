package id

import (
	"github.com/onflow/flow-go/model/flow"
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
