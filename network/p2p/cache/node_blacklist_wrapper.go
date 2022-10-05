package cache

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// IdentifierSet represents a set of blacklisted node IDs.
type IdentifierSet map[flow.Identifier]struct{}

// Includes returns true iff id ∈ s
func (s IdentifierSet) Includes(id flow.Identifier) bool {
	_, found := s[id]
	return found
}

// NodeBlacklistWrapper is a wrapper for an `id.IdentityProvider` instance, where the
// wrapper overrides the `Ejected` flag to true for all NodeIDs in a `blacklist`.
// This wrapper implements the external-facing interfaces
// To avoid modifying the source of the identities, the wrapper created shallow copies
// of the identities (whenever necessary) and modifies the `Ejected` flag only in
// the copy.
// The `NodeBlacklistWrapper` internally represents the `blacklist` as a map, to enable
// performant lookup. However, the exported API works with `flow.IdentifierList` for
// blacklist, as this is a broadly supported data structure which lends itself better
// to config or command-line inputs.
type NodeBlacklistWrapper struct {
	m  sync.RWMutex
	db *badger.DB

	identityProvider id.IdentityProvider
	blacklist        IdentifierSet // `IdentifierSet` is a map, hence efficient O(1) lookup
}

var _ id.IdentityProvider = (*NodeBlacklistWrapper)(nil)

// NewNodeBlacklistWrapper wraps the given `IdentityProvider`. The blacklist is
// loaded from the data base (or assumed to be empty if no data base entry is present).
func NewNodeBlacklistWrapper(identityProvider id.IdentityProvider, db *badger.DB) (*NodeBlacklistWrapper, error) {
	blacklist, err := retrieveBlacklist(db)
	if err != nil {
		return nil, fmt.Errorf("failed to read set of blacklisted node IDs from data base: %w", err)
	}

	return &NodeBlacklistWrapper{
		db:               db,
		identityProvider: identityProvider,
		blacklist:        blacklist,
	}, nil
}

// Update sets the wrapper's internal set of blacklisted nodes to `blacklist`. Empty list and `nil`
// (equivalent to empty list) are accepted inputs. To avoid legacy entries in the data base, this
// function purges the entire data base entry if `blacklist` is empty.
// This implementation is _eventually consistent_, where changes are written to the data base first
// and then (non-atomically!) the in-memory set of blacklisted nodes is updated. This strongly
// benefits performance and modularity. No errors are expected during normal operations.
func (w *NodeBlacklistWrapper) Update(blacklist flow.IdentifierList) error {
	b := blacklist.Lookup() // convert slice to
	err := persistBlacklist(b, w.db)
	if err != nil {
		return fmt.Errorf("failed to persist set of blacklisted nodes to the data base: %w", err)
	}

	w.m.Lock()
	w.blacklist = b
	w.m.Unlock()
	return nil
}

// ClearBlacklist purges the set of blacklisted node IDs. Convenience function
// equivalent to w.Update(nil). No errors are expected during normal operations.
func (w *NodeBlacklistWrapper) ClearBlacklist() error {
	return w.Update(nil)
}

// GetBlacklist returns the set of blacklisted node IDs.
func (w *NodeBlacklistWrapper) GetBlacklist() flow.IdentifierList {
	w.m.RLock()
	defer w.m.RUnlock()

	identifiers := make(flow.IdentifierList, 0, len(w.blacklist))
	for i, _ := range w.blacklist {
		identifiers = append(identifiers, i)
	}
	return identifiers
}

// Identities returns the full identities of _all_ nodes currently known to the
// protocol that pass the provided filter. Caution, this includes ejected nodes.
// Please check the `Ejected` flag in the returned identities (or provide a
// filter for removing ejected nodes).
func (w *NodeBlacklistWrapper) Identities(filter flow.IdentityFilter) flow.IdentityList {
	identities := w.identityProvider.Identities(filter)
	if len(identities) == 0 {
		return identities
	}

	// Iterate over all returned identities and set ejected flag to true. We
	// copy both the return slice and identities of blacklisted nodes to avoid
	// any possibility of accidentally modifying the wrapped IdentityProvider
	idtx := make(flow.IdentityList, 0, len(identities))
	w.m.RLock()
	for _, identity := range identities {
		if w.blacklist.Includes(identity.NodeID) {
			var i flow.Identity = *identity // shallow copy is sufficient, because `Ejected` flag is in top-level struct
			i.Ejected = true
			idtx = append(idtx, &i)
		} else {
			idtx = append(idtx, identity)
		}
	}
	w.m.RUnlock()
	return idtx
}

// ByNodeID returns the full identity for the node with the given Identifier,
// where Identifier is the way the protocol refers to the node. The function
// has the same semantics as a map lookup, where the boolean return value is
// true if and only if Identity has been found, i.e. `Identity` is not nil.
// Caution: function returns include ejected nodes. Please check the `Ejected`
// flag in the identity.
func (w *NodeBlacklistWrapper) ByNodeID(identifier flow.Identifier) (*flow.Identity, bool) {
	identity, b := w.identityProvider.ByNodeID(identifier)
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

	w.m.RLock()
	isBlacklisted := w.blacklist.Includes(identity.NodeID)
	w.m.RUnlock()
	if !isBlacklisted {
		return identity
	}

	// For blacklisted nodes, we want to return their `Identity` with the `Ejected` flag
	// set to true. Caution: we need to copy the `Identity` before we override `Ejected`, as we
	// would otherwise potentially change the wrapped IdentityProvider.
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
	identity, b := w.identityProvider.ByPeerID(p)
	return w.applyBlacklist(identity), b
}

// persistBlacklist writes the given blacklist to the data base. To avoid legacy
// entries in the data base, we pure the entire data base entry if `blacklist` is
// empty. No errors are expected during normal operations.
func persistBlacklist(blacklist IdentifierSet, db *badger.DB) error {
	if len(blacklist) == 0 {
		return db.Update(operation.PurgeBlacklistedNodes())
	}
	return db.Update(operation.PersistBlacklistedNodes(blacklist))
}

// retrieveBlacklist reads the set of blacklisted nodes from the data base.
// In case no data base entry exists, an empty set (nil map) is returned.
// No errors are expected during normal operations.
func retrieveBlacklist(db *badger.DB) (IdentifierSet, error) {
	var blacklist map[flow.Identifier]struct{}
	err := db.View(operation.RetrieveBlacklistedNodes(&blacklist))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("unexpected error reading set of blacklisted nodes from data base: %w", err)
	}
	return blacklist, nil
}
