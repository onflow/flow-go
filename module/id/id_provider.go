package id

import (
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// IdentifierProvider provides an interface to get a list of Identifiers representing
// a specific set of nodes in the network.
type IdentifierProvider interface {
	Identifiers() flow.IdentifierList
}

// IdentityProvider provides an interface to get a list of Identities representing
// the set of non-ejected participants in the Flow protocol.
type IdentityProvider interface {
	Identities(flow.IdentityFilter) flow.IdentityList
	ByNodeID(flow.Identifier) (*flow.Identity, bool)
	ByPeerID(peer.ID) (*flow.Identity, bool)
}
