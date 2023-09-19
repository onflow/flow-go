package module

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// IdentifierProvider provides an interface to get a list of Identifiers representing
// a specific set of nodes in the network.
type IdentifierProvider interface {
	Identifiers() flow.IdentifierList
}

// IdentityProvider provides an interface to get a list of Identities representing
// the set of participants in the Flow protocol. CAUTION: return values will include
// ejected nodes, so callers must check the `Identity.Ejected` flag.
type IdentityProvider interface {
	// Identities returns the full identities of _all_ nodes currently known to the
	// protocol that pass the provided filter. Caution, this includes ejected nodes.
	// Please check the `Ejected` flag in the identities (or provide a filter for
	// removing ejected nodes).
	Identities(flow.IdentityFilter[flow.Identity]) flow.IdentityList

	// ByNodeID returns the full identity for the node with the given Identifier,
	// where Identifier is the way the protocol refers to the node. The function
	// has the same semantics as a map lookup, where the boolean return value is
	// true if and only if Identity has been found, i.e. `Identity` is not nil.
	// Caution: function returns include ejected nodes. Please check the `Ejected`
	// flag in the identity.
	ByNodeID(flow.Identifier) (*flow.Identity, bool)

	// ByPeerID returns the full identity for the node with the given peer ID,
	// where ID is the way the libP2P refers to the node. The function
	// has the same semantics as a map lookup, where the boolean return value is
	// true if and only if Identity has been found, i.e. `Identity` is not nil.
	// Caution: function returns include ejected nodes. Please check the `Ejected`
	// flag in the identity.
	ByPeerID(peer.ID) (*flow.Identity, bool)
}
