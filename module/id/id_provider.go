package id

import (
	"github.com/onflow/flow-go/model/flow"
)

// IdentifierProvider provides an interface to get a list of Identifiers representing
// a specific set of nodes in the network.
type IdentifierProvider interface {
	Identifiers() flow.IdentifierList
}

// IdentifierProvider provides an interface to get a list of Identities representing
// the set of staked participants in the Flow protocol.
type IdentityProvider interface {
	Identities(flow.IdentityFilter) flow.IdentityList
}
