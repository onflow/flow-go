package id

import (
	"github.com/onflow/flow-go/model/flow"
)

// CustomIdentifierProvider implements an IdentifierProvider which provides results from the given function.
type CustomIdentifierProvider struct {
	identifiers func() flow.IdentifierList
}

func NewCustomIdentifierProvider(identifiers func() flow.IdentifierList) *CustomIdentifierProvider {
	return &CustomIdentifierProvider{identifiers}
}

func (p *CustomIdentifierProvider) Identifiers() flow.IdentifierList {
	return p.identifiers()
}
