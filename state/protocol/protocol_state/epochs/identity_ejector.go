package epochs

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// trackedDynamicIdentityList is a helper structure for tracking identity lists in the state machine.
// It is used to implement lazy initialization of the tracked identity list.
// The structure relies on holding a reference to the list that is being modified.
type trackedDynamicIdentityList struct {
	dynamicIdentities flow.DynamicIdentityEntryList
	identityLookup    map[flow.Identifier]*flow.DynamicIdentityEntry
}

// ejector is a dedicated structure for tracking ejected nodes in the state machine.
// It is capable of tracking multiple identity lists and ejecting nodes from them.
// The implementation is optimized for hot-path where ejections are rare, and by utilizing the lazy initialization,
// the data structures are not populated until the first ejection is requested.
// The ejector is used in the baseStateMachine to track ejected nodes and ensure that they are not readmitted during
// the lifetime of the state machine.
// It is not concurrency-safe.
type ejector struct {
	identityLists []trackedDynamicIdentityList
	ejected       []flow.Identifier
}

// newEjector implements a constructor for the ejector structure with a pre-allocated slice for identity lists.
// We are always going to add at least one element, and most often two (previous and current epoch), but never more than three.
func newEjector() ejector {
	return ejector{
		identityLists: make([]trackedDynamicIdentityList, 0, 3),
	}
}

// Eject marks the node as ejected in all tracked identity lists. If and only if the node is active in the previous
// or current or next epoch, the node's ejection status is set to true for all occurrences, and we return true. If
// `nodeID` is not found, we return false. This method is idempotent and behaves identically for repeated calls with
// the same `nodeID`. Repeated calls with the same input create minor performance overhead.
//
// If it's the first ejection during the `ejector`s lifetime (i.e. this `ejector` has no previous ejection events
// memorized), it populates an internal lookup for each `DynamicIdentityList` is tracks. This lazy initialization
// benefits the vastly common happy path (no ejection events during the ejector's lifetime).
func (e *ejector) Eject(nodeID flow.Identifier) bool {
	l := len(e.identityLists)
	if len(e.ejected) == 0 { // if this is the first ejection sealed in this block, we have to populate the lookup first
		for i := 0; i < l; i++ {
			e.identityLists[i].identityLookup = e.identityLists[i].dynamicIdentities.Lookup()
		}
	}
	e.ejected = append(e.ejected, nodeID)

	var nodeFound bool
	for i := 0; i < l; i++ {
		dynamicIdentity, found := e.identityLists[i].identityLookup[nodeID]
		if found {
			nodeFound = true
			dynamicIdentity.Ejected = true
		}
	}
	return nodeFound
}

// TrackDynamicIdentityList tracks a new DynamicIdentityList in the state machine.
// It is not allowed to readmit nodes that were ejected. Whenever a new DynamicIdentityList is tracked,
// we ensure that the ejection status of previously ejected nodes is not reverted.
// If a node was previously ejected and the new DynamicIdentityList contains the node with an `Ejected`
// status of `false`, a `protocol.InvalidServiceEventError` is returned and the ejector remains unchanged.
func (e *ejector) TrackDynamicIdentityList(list flow.DynamicIdentityEntryList) error {
	tracker := trackedDynamicIdentityList{dynamicIdentities: list}
	if len(e.ejected) > 0 {
		// nodes were already ejected in this block, so their ejection should not be reverted in the new `list`
		tracker.identityLookup = list.Lookup()
		for _, id := range e.ejected {
			dynamicIdentity, found := tracker.identityLookup[id]
			if found && !dynamicIdentity.Ejected {
				return protocol.NewInvalidServiceEventErrorf("node %v was previously ejected but next DynamicIdentityList reverts its ejection status", id)
			}
		}
	}
	e.identityLists = append(e.identityLists, tracker)
	return nil
}
