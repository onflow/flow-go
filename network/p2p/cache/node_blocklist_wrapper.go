package cache

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// IdentifierSet represents a set of node IDs (operator-defined) whose communication should be blocked.
type IdentifierSet map[flow.Identifier]struct{}

// Contains returns true iff id ∈ s
func (s IdentifierSet) Contains(id flow.Identifier) bool {
	_, found := s[id]
	return found
}

// NodeBlocklistWrapper is a wrapper for an `module.IdentityProvider` instance, where the
// wrapper overrides the `Ejected` flag to true for all NodeIDs in a `blocklist`.
// To avoid modifying the source of the identities, the wrapper creates shallow copies
// of the identities (whenever necessary) and modifies the `Ejected` flag only in
// the copy.
// The `NodeBlocklistWrapper` internally represents the `blocklist` as a map, to enable
// performant lookup. However, the exported API works with `flow.IdentifierList` for
// blocklist, as this is a broadly supported data structure which lends itself better
// to config or command-line inputs.
type NodeBlocklistWrapper struct {
	m  sync.RWMutex
	db *badger.DB

	identityProvider module.IdentityProvider
	blocklist        IdentifierSet                           // `IdentifierSet` is a map, hence efficient O(1) lookup
	distributor      p2p.DisallowListNotificationDistributor // distributor for the blocklist update notifications
}

var _ module.IdentityProvider = (*NodeBlocklistWrapper)(nil)

// NewNodeBlocklistWrapper wraps the given `IdentityProvider`. The blocklist is
// loaded from the database (or assumed to be empty if no database entry is present).
func NewNodeBlocklistWrapper(
	identityProvider module.IdentityProvider,
	db *badger.DB,
	distributor p2p.DisallowListNotificationDistributor) (*NodeBlocklistWrapper, error) {
	blocklist, err := retrieveBlocklist(db)
	if err != nil {
		return nil, fmt.Errorf("failed to read set of blocked node IDs from data base: %w", err)
	}

	if distributor == nil {
		panic("WARNING: NodeBlocklistWrapper created without a distributor. This is a bug.")
	}

	return &NodeBlocklistWrapper{
		db:               db,
		identityProvider: identityProvider,
		blocklist:        blocklist,
		distributor:      distributor,
	}, nil
}

// Update sets the wrapper's internal set of blocked nodes to `blocklist`. Empty list and `nil`
// (equivalent to empty list) are accepted inputs. To avoid legacy entries in the data base, this
// function purges the entire data base entry if `blocklist` is empty.
// This implementation is _eventually consistent_, where changes are written to the data base first
// and then (non-atomically!) the in-memory set of blocked nodes is updated. This strongly
// benefits performance and modularity. No errors are expected during normal operations.
func (w *NodeBlocklistWrapper) Update(blocklist flow.IdentifierList) error {
	b := blocklist.Lookup() // converts slice to map

	w.m.Lock()
	defer w.m.Unlock()
	err := persistBlocklist(b, w.db)
	if err != nil {
		return fmt.Errorf("failed to persist set of blocked nodes to the data base: %w", err)
	}
	w.blocklist = b
	err = w.distributor.DistributeBlockListNotification(blocklist)

	if err != nil {
		return fmt.Errorf("failed to distribute blocklist update notification: %w", err)
	}

	return nil
}

// ClearBlocklist purges the set of blocked node IDs. Convenience function
// equivalent to w.Update(nil). No errors are expected during normal operations.
func (w *NodeBlocklistWrapper) ClearBlocklist() error {
	return w.Update(nil)
}

// GetBlocklist returns the set of blocked node IDs.
func (w *NodeBlocklistWrapper) GetBlocklist() flow.IdentifierList {
	w.m.RLock()
	defer w.m.RUnlock()

	identifiers := make(flow.IdentifierList, 0, len(w.blocklist))
	for i := range w.blocklist {
		identifiers = append(identifiers, i)
	}
	return identifiers
}

// Identities returns the full identities of _all_ nodes currently known to the
// protocol that pass the provided filter. Caution, this includes ejected nodes.
// Please check the `Ejected` flag in the returned identities (or provide a
// filter for removing ejected nodes).
func (w *NodeBlocklistWrapper) Identities(filter flow.IdentityFilter) flow.IdentityList {
	identities := w.identityProvider.Identities(filter)
	if len(identities) == 0 {
		return identities
	}

	// Iterate over all returned identities and set ejected flag to true. We
	// copy both the return slice and identities of blocked nodes to avoid
	// any possibility of accidentally modifying the wrapped IdentityProvider
	idtx := make(flow.IdentityList, 0, len(identities))
	w.m.RLock()
	for _, identity := range identities {
		if w.blocklist.Contains(identity.NodeID) {
			var i = *identity // shallow copy is sufficient, because `Ejected` flag is in top-level struct
			i.Ejected = true
			if filter(&i) { // we need to check the filter here again, because the filter might drop ejected nodes and we are modifying the ejected status here
				idtx = append(idtx, &i)
			}
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
func (w *NodeBlocklistWrapper) ByNodeID(identifier flow.Identifier) (*flow.Identity, bool) {
	identity, b := w.identityProvider.ByNodeID(identifier)
	return w.setEjectedIfBlocked(identity), b
}

// setEjectedIfBlocked checks whether the node with the given identity is on the `blocklist`.
// Shortcuts:
//   - If the node's identity is nil, there is nothing to do because we don't generate identities here.
//   - If the node is already ejected, we don't have to check the blocklist.
func (w *NodeBlocklistWrapper) setEjectedIfBlocked(identity *flow.Identity) *flow.Identity {
	if identity == nil || identity.Ejected {
		return identity
	}

	w.m.RLock()
	isBlocked := w.blocklist.Contains(identity.NodeID)
	w.m.RUnlock()
	if !isBlocked {
		return identity
	}

	// For blocked nodes, we want to return their `Identity` with the `Ejected` flag
	// set to true. Caution: we need to copy the `Identity` before we override `Ejected`, as we
	// would otherwise potentially change the wrapped IdentityProvider.
	var i = *identity // shallow copy is sufficient, because `Ejected` flag is in top-level struct
	i.Ejected = true
	return &i
}

// ByPeerID returns the full identity for the node with the given peer ID,
// peer.ID is the libp2p-level identifier of a Flow node. The function
// has the same semantics as a map lookup, where the boolean return value is
// true if and only if Identity has been found, i.e. `Identity` is not nil.
// Caution: function returns include ejected nodes. Please check the `Ejected`
// flag in the identity.
func (w *NodeBlocklistWrapper) ByPeerID(p peer.ID) (*flow.Identity, bool) {
	identity, b := w.identityProvider.ByPeerID(p)
	return w.setEjectedIfBlocked(identity), b
}

// persistBlocklist writes the given blocklist to the database. To avoid legacy
// entries in the database, we prune the entire data base entry if `blocklist` is
// empty. No errors are expected during normal operations.
func persistBlocklist(blocklist IdentifierSet, db *badger.DB) error {
	if len(blocklist) == 0 {
		return db.Update(operation.PurgeBlocklist())
	}
	return db.Update(operation.PersistBlocklist(blocklist))
}

// retrieveBlocklist reads the set of blocked nodes from the data base.
// In case no database entry exists, an empty set (nil map) is returned.
// No errors are expected during normal operations.
func retrieveBlocklist(db *badger.DB) (IdentifierSet, error) {
	var blocklist map[flow.Identifier]struct{}
	err := db.View(operation.RetrieveBlocklist(&blocklist))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("unexpected error reading set of blocked nodes from data base: %w", err)
	}
	return blocklist, nil
}
