package cache

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
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

// NodeDisallowListingWrapper is a wrapper for an `module.IdentityProvider` instance, where the
// wrapper overrides the `Ejected` flag to true for all NodeIDs in a `disallowList`.
// To avoid modifying the source of the identities, the wrapper creates shallow copies
// of the identities (whenever necessary) and modifies the `Ejected` flag only in
// the copy.
// The `NodeDisallowListingWrapper` internally represents the `disallowList` as a map, to enable
// performant lookup. However, the exported API works with `flow.IdentifierList` for
// disallowList, as this is a broadly supported data structure which lends itself better
// to config or command-line inputs.
// When a node is disallow-listed, the networking layer connection to that node is closed and no
// incoming or outgoing connections are established with that node.
// TODO: terminology change - rename `blocklist` to `disallowList` everywhere to be consistent with the code.
type NodeDisallowListingWrapper struct {
	m  sync.RWMutex
	db *badger.DB

	identityProvider module.IdentityProvider
	disallowList     IdentifierSet // `IdentifierSet` is a map, hence efficient O(1) lookup

	// updateConsumerOracle is called whenever the disallow-list is updated.
	// Note that we do not use the `updateConsumer` directly due to the circular dependency between the
	// networking layer Underlay interface (i.e., updateConsumer), and the wrapper (i.e., NodeDisallowListingWrapper).
	// Underlay needs identity provider to be initialized, and identity provider needs this wrapper to be initialized.
	// Hence, if we pass the updateConsumer by the interface value, it will be nil at the time of initialization.
	// Instead, we use the oracle function to get the updateConsumer whenever we need it.
	updateConsumerOracle func() network.DisallowListNotificationConsumer
}

var _ module.IdentityProvider = (*NodeDisallowListingWrapper)(nil)

// NewNodeDisallowListWrapper wraps the given `IdentityProvider`. The disallow-list is
// loaded from the database (or assumed to be empty if no database entry is present).
func NewNodeDisallowListWrapper(
	identityProvider module.IdentityProvider,
	db *badger.DB,
	updateConsumerOracle func() network.DisallowListNotificationConsumer) (*NodeDisallowListingWrapper, error) {

	disallowList, err := retrieveDisallowList(db)
	if err != nil {
		return nil, fmt.Errorf("failed to read set of disallowed node IDs from data base: %w", err)
	}

	return &NodeDisallowListingWrapper{
		db:                   db,
		identityProvider:     identityProvider,
		disallowList:         disallowList,
		updateConsumerOracle: updateConsumerOracle,
	}, nil
}

// Update sets the wrapper's internal set of blocked nodes to `disallowList`. Empty list and `nil`
// (equivalent to empty list) are accepted inputs. To avoid legacy entries in the database, this
// function purges the entire data base entry if `disallowList` is empty.
// This implementation is _eventually consistent_, where changes are written to the database first
// and then (non-atomically!) the in-memory set of blocked nodes is updated. This strongly
// benefits performance and modularity. No errors are expected during normal operations.
//
// Args:
// - disallowList: list of node IDs to be disallow-listed from the networking layer, i.e., the existing connections
// to these nodes will be closed and no new connections will be established (neither incoming nor outgoing).
//
// Returns:
// - error: if the update fails, e.g., due to a database error. Any returned error is irrecoverable and the caller
// should abort the process.
func (w *NodeDisallowListingWrapper) Update(disallowList flow.IdentifierList) error {
	b := disallowList.Lookup() // converts slice to map

	w.m.Lock()
	defer w.m.Unlock()
	err := persistDisallowList(b, w.db)
	if err != nil {
		return fmt.Errorf("failed to persist set of blocked nodes to the data base: %w", err)
	}
	w.disallowList = b
	w.updateConsumerOracle().OnDisallowListNotification(&network.DisallowListingUpdate{
		FlowIds: disallowList,
		Cause:   network.DisallowListedCauseAdmin,
	})

	return nil
}

// ClearDisallowList purges the set of blocked node IDs. Convenience function
// equivalent to w.Update(nil). No errors are expected during normal operations.
func (w *NodeDisallowListingWrapper) ClearDisallowList() error {
	return w.Update(nil)
}

// GetDisallowList returns the set of blocked node IDs.
func (w *NodeDisallowListingWrapper) GetDisallowList() flow.IdentifierList {
	w.m.RLock()
	defer w.m.RUnlock()

	identifiers := make(flow.IdentifierList, 0, len(w.disallowList))
	for i := range w.disallowList {
		identifiers = append(identifiers, i)
	}
	return identifiers
}

// Identities returns the full identities of _all_ nodes currently known to the
// protocol that pass the provided filter. Caution, this includes ejected nodes.
// Please check the `Ejected` flag in the returned identities (or provide a
// filter for removing ejected nodes).
func (w *NodeDisallowListingWrapper) Identities(filter flow.IdentityFilter[flow.Identity]) flow.IdentityList {
	identities := w.identityProvider.Identities(filter)
	if len(identities) == 0 {
		return identities
	}

	// Iterate over all returned identities and set the `EpochParticipationStatus` to `flow.EpochParticipationStatusEjected`.
	// We copy both the return slice and identities of blocked nodes to avoid
	// any possibility of accidentally modifying the wrapped IdentityProvider
	idtx := make(flow.IdentityList, 0, len(identities))
	w.m.RLock()
	for _, identity := range identities {
		if w.disallowList.Contains(identity.NodeID) {
			var i = *identity // shallow copy is sufficient, because `EpochParticipationStatus` is a value type in DynamicIdentity which is also a value type.
			i.EpochParticipationStatus = flow.EpochParticipationStatusEjected
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
func (w *NodeDisallowListingWrapper) ByNodeID(identifier flow.Identifier) (*flow.Identity, bool) {
	identity, b := w.identityProvider.ByNodeID(identifier)
	return w.setEjectedIfBlocked(identity), b
}

// setEjectedIfBlocked checks whether the node with the given identity is on the `disallowList`.
// Shortcuts:
//   - If the node's identity is nil, there is nothing to do because we don't generate identities here.
//   - If the node is already ejected, we don't have to check the disallowList.
func (w *NodeDisallowListingWrapper) setEjectedIfBlocked(identity *flow.Identity) *flow.Identity {
	if identity == nil || identity.EpochParticipationStatus == flow.EpochParticipationStatusEjected {
		return identity
	}

	w.m.RLock()
	isBlocked := w.disallowList.Contains(identity.NodeID)
	w.m.RUnlock()
	if !isBlocked {
		return identity
	}

	// For blocked nodes, we want to return their `Identity` with the `EpochParticipationStatus`
	// set to `flow.EpochParticipationStatusEjected`.
	// Caution: we need to copy the `Identity` before we override `EpochParticipationStatus`, as we
	// would otherwise potentially change the wrapped IdentityProvider.
	var i = *identity // shallow copy is sufficient, because `EpochParticipationStatus` is a value type in DynamicIdentity which is also a value type.
	i.EpochParticipationStatus = flow.EpochParticipationStatusEjected
	return &i
}

// ByPeerID returns the full identity for the node with the given peer ID,
// peer.ID is the libp2p-level identifier of a Flow node. The function
// has the same semantics as a map lookup, where the boolean return value is
// true if and only if Identity has been found, i.e. `Identity` is not nil.
// Caution: function returns include ejected nodes. Please check the `Ejected`
// flag in the identity.
func (w *NodeDisallowListingWrapper) ByPeerID(p peer.ID) (*flow.Identity, bool) {
	identity, b := w.identityProvider.ByPeerID(p)
	return w.setEjectedIfBlocked(identity), b
}

// persistDisallowList writes the given disallowList to the database. To avoid legacy
// entries in the database, we prune the entire data base entry if `disallowList` is
// empty. No errors are expected during normal operations.
func persistDisallowList(disallowList IdentifierSet, db *badger.DB) error {
	if len(disallowList) == 0 {
		return db.Update(operation.PurgeBlocklist())
	}
	return db.Update(operation.PersistBlocklist(disallowList))
}

// retrieveDisallowList reads the set of blocked nodes from the data base.
// In case no database entry exists, an empty set (nil map) is returned.
// No errors are expected during normal operations.
func retrieveDisallowList(db *badger.DB) (IdentifierSet, error) {
	var blocklist map[flow.Identifier]struct{}
	err := db.View(operation.RetrieveBlocklist(&blocklist))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("unexpected error reading set of blocked nodes from data base: %w", err)
	}
	return blocklist, nil
}
