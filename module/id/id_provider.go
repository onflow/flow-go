package id

import (
	"github.com/onflow/flow-go/model/flow"
)

type IdentifierProvider interface {
	Identifiers() flow.IdentifierList
}

type IdentityProvider interface {
	Identities(flow.IdentityFilter) flow.IdentityList
}
