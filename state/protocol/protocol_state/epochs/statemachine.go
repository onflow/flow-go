package epochs

import "github.com/onflow/flow-go/model/flow"

// EpochStateMachine implements a low-level interface for state-changing operations on the protocol state.
// It is used by higher level logic to evolve the protocol state when certain events that are stored in blocks are observed.
// The EpochStateMachine is stateful and internally tracks the current protocol state. A separate instance is created for
// each block that is being processed.
type EpochStateMachine interface {
	// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
	// CAUTION:
	// Do NOT call Build, if the EpochStateMachine instance has returned a `protocol.InvalidServiceEventError`
	// at any time during its lifetime. After this error, the EpochStateMachine is left with a potentially
	// dysfunctional state and should be discarded.
	Build() (updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, hasChanges bool)

	// ProcessEpochSetup updates current protocol state with data from epoch setup event.
	// Processing epoch setup event also affects identity table for current epoch.
	// Observing an epoch setup event, transitions protocol state from staking to setup phase, we stop returning
	// identities from previous+current epochs and start returning identities from current+next epochs.
	// As a result of this operation protocol state for the next epoch will be created.
	// Returned boolean indicates if event triggered a transition in the state machine or not.
	// Implementors must never return (true, error).
	// Expected errors indicating that we are leaving the happy-path of the epoch transitions
	//   - `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
	//     CAUTION: the HappyPathStateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
	//     after such error and discard the HappyPathStateMachine!
	ProcessEpochSetup(epochSetup *flow.EpochSetup) (bool, error)

	// ProcessEpochCommit updates current protocol state with data from epoch commit event.
	// Observing an epoch setup commit, transitions protocol state from setup to commit phase.
	// At this point, we have finished construction of the next epoch.
	// As a result of this operation protocol state for next epoch will be committed.
	// Returned boolean indicates if event triggered a transition in the state machine or not.
	// Implementors must never return (true, error).
	// Expected errors indicating that we are leaving the happy-path of the epoch transitions
	//   - `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
	//     CAUTION: the HappyPathStateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
	//     after such error and discard the HappyPathStateMachine!
	ProcessEpochCommit(epochCommit *flow.EpochCommit) (bool, error)

	// EjectIdentity updates identity table by changing the node's participation status to 'ejected'.
	// Should pass identity which is already present in the table, otherwise an exception will be raised.
	// Expected errors during normal operations:
	// - `protocol.InvalidServiceEventError` if the updated identity is not found in current and adjacent epochs.
	EjectIdentity(nodeID flow.Identifier) error

	// TransitionToNextEpoch discards current protocol state and transitions to the next epoch.
	// Epoch transition is only allowed when:
	// - next epoch has been set up,
	// - next epoch has been committed,
	// - candidate block is in the next epoch.
	// No errors are expected during normal operations.
	TransitionToNextEpoch() error

	// View returns the view that is associated with this EpochStateMachine.
	// The view of the EpochStateMachine equals the view of the block carrying the respective updates.
	View() uint64

	// ParentState returns parent protocol state that is associated with this EpochStateMachine.
	ParentState() *flow.RichProtocolStateEntry
}
