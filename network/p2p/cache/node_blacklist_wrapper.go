package cache

import (
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network/p2p"
)

// NodeBlacklistWrapper is a wrapper for a `ProtocolStateIDCache` instance, where the
// wrapper overrides the `Ejected` flag to true for all NodeIDs in a `blacklist`.
// This wrapper implements the external-facing interfaces
// To avoid modifying the source of the identities, the wrapper created shallow copies
// of the identities (whenever necessary) and modifies the `Ejected` flag only in
// the copy.
type NodeBlacklistWrapper struct {
	c *ProtocolStateIDCache
	m sync.RWMutex

	blacklist map[flow.Identifier]struct{} // if nodeID appears in set, it is blacklisted
}

var _ id.IdentityProvider = (*NodeBlacklistWrapper)(nil)
var _ p2p.IDTranslator = (*ProtocolStateIDCache)(nil)

// NewNodeBlacklistWrapper wraps the given `ProtocolStateIDCache`. The blacklist is
// loaded from the data base (or assumed to be empty if no data base entry is present).
func NewNodeBlacklistWrapper(idCache *ProtocolStateIDCache, db *badger.DB) (*NodeBlacklistWrapper, error) {
	panic("implement me")
	return nil, nil
}

// Update sets the wrapper's internal set of blacklisted nodes to `blacklist`.
// Caution: the blacklist is _not_ copied and should not be mutated anymore after this call.
func (w *NodeBlacklistWrapper) Update(blacklist map[flow.Identifier]struct{}) {
	w.m.Lock()
	w.blacklist = blacklist
	w.m.Unlock()
}

// Identities returns the full identities of _all_ nodes currently known to the
// protocol that pass the provided filter. Caution, this includes ejected nodes.
// Please check the `Ejected` flag in the identities (or provide a filter for
// removing ejected nodes).
func (w *NodeBlacklistWrapper) Identities(filter flow.IdentityFilter) flow.IdentityList {
	identities := w.c.Identities(filter)
	if len(identities) == 0 {
		return identities
	}

	idtx := make(flow.IdentityList, 0, len(identities))

	w.m.RLock()
	defer w.m.RUnlock()
	for _, identity := range identities {
		if _, isBlacklisted := w.blacklist[identity.NodeID]; isBlacklisted {
			var i flow.Identity = *identity // shallow copy is sufficient, because `Ejected` flag is in top-level struct
			i.Ejected = true
			idtx = append(idtx, &i)
		} else {
			idtx = append(idtx, identity)
		}
	}
	return idtx
}

// ByNodeID returns the full identity for the node with the given Identifier,
// where Identifier is the way the protocol refers to the node. The function
// has the same semantics as a map lookup, where the boolean return value is
// true if and only if Identity has been found, i.e. `Identity` is not nil.
// Caution: function returns include ejected nodes. Please check the `Ejected`
// flag in the identity.
func (w *NodeBlacklistWrapper) ByNodeID(identifier flow.Identifier) (*flow.Identity, bool) {
	identity, b := w.c.ByNodeID(identifier)
	return w.applyBlacklist(identity), b
}

// applyBlacklist checks whether the node with the given identity is on the `blacklist`.
// Shortcuts:
//   - If the node's identity is nil, there is nothing to do because we don't generate identities here.
//   - If the node is already ejected, we don't have to check the black list.
func (w *NodeBlacklistWrapper) applyBlacklist(identity *flow.Identity) *flow.Identity {
	if identity == nil || identity.Ejected {
		return identity
	}

	// We only enter the following code when: identity ≠ nil  _and_  identity.Ejected = false.
	w.m.RLock()
	defer w.m.RUnlock()

	if _, isBlacklisted := w.blacklist[identity.NodeID]; !isBlacklisted {
		return identity // node not blacklisted
	}

	// For blacklisted nodes, we want to return their flow.Identity with the `Ejected` flag
	// set to true. Hence, we copy the identity, and override `Ejected`.
	var i flow.Identity = *identity // shallow copy is sufficient, because `Ejected` flag is in top-level struct
	i.Ejected = true
	return &i
}

// ByPeerID returns the full identity for the node with the given peer ID,
// where ID is the way the libP2P refers to the node. The function
// has the same semantics as a map lookup, where the boolean return value is
// true if and only if Identity has been found, i.e. `Identity` is not nil.
// Caution: function returns include ejected nodes. Please check the `Ejected`
// flag in the identity.
func (w *NodeBlacklistWrapper) ByPeerID(p peer.ID) (*flow.Identity, bool) {
	identity, b := w.c.ByPeerID(p)
	return w.applyBlacklist(identity), b
}

// GetPeerID returns the peer ID for the given Flow ID.
// During normal operations, the following error returns are expected
//   - ErrUnknownId if the given Identifier is unknown
func (w *NodeBlacklistWrapper) GetPeerID(flowID flow.Identifier) (pid peer.ID, err error) {
	return w.c.GetPeerID(flowID)
}

// GetFlowID returns the Flow ID for the given peer ID.
// During normal operations, the following error returns are expected
//   - ErrUnknownId if the given Identifier is unknown
func (w *NodeBlacklistWrapper) GetFlowID(peerID peer.ID) (fid flow.Identifier, err error) {
	return w.c.GetFlowID(peerID)
}
