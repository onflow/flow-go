package id

import (
	"github.com/onflow/flow-go/model/flow"
)

type FixedIdentifierProvider struct {
	identifiers flow.IdentifierList
}

func NewFixedIdentifierProvider(identifiers flow.IdentifierList) *FixedIdentifierProvider {
	return &FixedIdentifierProvider{identifiers}
}

func (p *FixedIdentifierProvider) Identifiers() flow.IdentifierList {
	return p.identifiers
}

type FixedIdentityProvider struct {
	identities flow.IdentityList
}

func NewFixedIdentityProvider(identities flow.IdentityList) *FixedIdentityProvider {
	return &FixedIdentityProvider{identities}
}

func (p *FixedIdentityProvider) Identities(flow.IdentityFilter) flow.IdentityList {
	return p.identities
}
