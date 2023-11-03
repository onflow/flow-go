package protocol_state

import "github.com/onflow/flow-go/model/flow"

// ProtocolStateMachine implements a low-level interface for state-changing operations on the protocol state.
// It is used by higher level logic to evolve the protocol state when certain events that are stored in blocks are observed.
// The ProtocolStateMachine is stateful and internally tracks the current protocol state. A separate instance is created for
// each block that is being processed.
type ProtocolStateMachine interface {
	// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
	Build() (updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, hasChanges bool)
	// ProcessEpochSetup updates current protocol state with data from epoch setup event.
	// Processing epoch setup event also affects identity table for current epoch.
	// Observing an epoch setup event, transitions protocol state from staking to setup phase, we stop returning
	// identities from previous+current epochs and start returning identities from current+next epochs.
	// As a result of this operation protocol state for the next epoch will be created.
	// Expected errors during normal operations:
	// - `protocol.InvalidServiceEventError` if the service event is invalid or is not a valid state transition for the current protocol state
	ProcessEpochSetup(epochSetup *flow.EpochSetup) error
	// ProcessEpochCommit updates current protocol state with data from epoch commit event.
	// Observing an epoch setup commit, transitions protocol state from setup to commit phase, at this point we have
	// finished construction of the next epoch.
	// As a result of this operation protocol state for next epoch will be committed.
	// Expected errors during normal operations:
	// - `protocol.InvalidServiceEventError` if the service event is invalid or is not a valid state transition for the current protocol state
	ProcessEpochCommit(epochCommit *flow.EpochCommit) error
	// UpdateIdentity updates identity table with new identity entry.
	// Should pass identity which is already present in the table, otherwise an exception will be raised.
	// TODO: This function currently modifies both current+next identities based on input.
	//       This is incompatible with the design doc, and needs to be updated to modify current/next epoch separately
	// No errors are expected during normal operations.
	UpdateIdentity(updated *flow.DynamicIdentityEntry) error
	// SetInvalidStateTransitionAttempted sets a flag indicating that invalid state transition was attempted.
	// Such transition can be detected by compliance layer.
	SetInvalidStateTransitionAttempted()
	// TransitionToNextEpoch discards current protocol state and transitions to the next epoch.
	// Epoch transition is only allowed when:
	// - next epoch has been set up,
	// - next epoch has been committed,
	// - candidate block is in the next epoch.
	// No errors are expected during normal operations.
	TransitionToNextEpoch() error
	// View returns the view that is associated with this state updater.
	// The view of the ProtocolStateMachine equals the view of the block carrying the respective updates.
	View() uint64
	// ParentState returns parent protocol state that is associated with this state updater.
	ParentState() *flow.RichProtocolStateEntry
}
