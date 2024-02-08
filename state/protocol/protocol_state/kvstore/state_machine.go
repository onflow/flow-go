// TODO copied this file directly from design doc (https://www.notion.so/dapperlabs/Protocol-state-key-value-storage-497326ff9cf44ff4a70610a0dad329b3).

package kvstore

import (
	"github.com/onflow/flow-go/model/flow"
)

// StateMachine implements a low-level interface for state-changing operations on the key-value store.
// It is used by higher level logic to evolve the protocol state when certain events that are stored in blocks are observed.
// The StateMachine is stateful and internally tracks the current state of key-value store. A separate instance is created for
// each block that is being processed.
type StateMachine interface {
	// Build returns updated key-value store model, state ID and a flag indicating if there were any changes.
	// TODO: in design doc updatedState is concrete struct.
	//   I think it needs to be the interface/API type instead?
	Build() (updatedState LatestKVInterface, stateID flow.Identifier, hasChanges bool)

	// ProcessUpdate updates current state of key-value store.
	// Expected errors indicating that we have observed and invalid service event from protocol's point of view.
	//   - `protocol.InvalidServiceEventError` - if the service event is invalid for the current protocol state.
	//     CAUTION: the StateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
	//     after such error and discard the protocolStateMachine!
	ProcessUpdate(update *flow.ServiceEvent) error

	// View returns the view that is associated with this StateMachine.
	// The view of the StateMachine equals the view of the block carrying the respective updates.
	View() uint64

	// ParentState returns parent state that is associated with this state machine.
	ParentState() *LatestKVModel
}

type KeyValueStoreStateMachine struct {
	currentKeyValueStore *LatestKVModel
}

// ProcessUpdate updates current state of key-value store.
// Expected errors indicating that we have observed and invalid service event from protocol's point of view.
//   - `protocol.InvalidServiceEventError` - if the service event is invalid for the current protocol state.
//     CAUTION: the StateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
//     after such error and discard the protocolStateMachine!
func (sm *KeyValueStoreStateMachine) ProcessUpdate(ev *flow.ServiceEvent) error {
	//switch update := ev.(type) {
	//case *UpdateMinimumConsensusNodeVersion:
	//	// validate value
	//	if update.MinimumConsensusNodeVersion <= sm.currentKeyValueStore.GetMinimumConsensusNodeVersion() {
	//		// bail with error
	//	}
	//	sm.currentKeyValueStore.SetMinimumConsensusNodeVersion(update.MinimumConsensusNodeVersion)
	//	...
	//default:
	//	return fmt.Errorf("unsupported key %s with type %v", ev.Key(), ev.(type))
	//}
	//
	return nil
}
