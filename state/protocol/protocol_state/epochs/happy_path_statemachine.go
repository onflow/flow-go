package epochs

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
)

// HappyPathStateMachine is a dedicated structure for evolving the Epoch-related portion of the overall Protocol State.
// Based on the content of a new block, it updates epoch data, including the identity table, on the happy path.
// The HappyPathStateMachine guarantees protocol-compliant evolution of Epoch-related sub-state via the
// following state transitions:
//   - epoch setup: transitions current epoch from staking to setup phase, creates next epoch protocol state when processed.
//   - epoch commit: transitions current epoch from setup to commit phase, commits next epoch protocol state when processed.
//   - epoch transition: on the first block of the new epoch (Formally, the block's parent is still in the last epoch,
//     while the new block has a view in the next epoch. Caution: the block's view is not necessarily the first view
//     in the epoch, as there might be leader failures)
//   - identity changes: updates identity table for previous (if available), current, and next epoch (if available).
//
// All updates are applied to a copy of parent protocol state, so parent protocol state is not modified. The stateMachine internally
// tracks the current protocol state. A separate instance should be created for each block to processing the updates therein.
type HappyPathStateMachine struct {
	baseStateMachine
}

var _ StateMachine = (*HappyPathStateMachine)(nil)

// NewStateMachine creates a new protocol state HappyPathStateMachine.
// An exception is returned in case the `InvalidEpochTransitionAttempted` flag is set in the `parentState`. This means that
// the protocol state evolution has reached an undefined state from the perspective of the happy path state machine.
func NewStateMachine(view uint64, parentState *flow.RichProtocolStateEntry) (*HappyPathStateMachine, error) {
	if parentState.InvalidEpochTransitionAttempted {
		return nil, irrecoverable.NewExceptionf("cannot create happy path protocol state machine at view (%d) for a parent state"+
			"which is in Epoch Fallback Mode", view)
	}
	return &HappyPathStateMachine{
		baseStateMachine: baseStateMachine{
			parentState: parentState,
			state:       parentState.ProtocolStateEntry.Copy(),
			view:        view,
		},
	}, nil
}

// ProcessEpochSetup updates the protocol state with data from the epoch setup event.
// Observing an epoch setup event also affects the identity table for current epoch:
//   - it transitions the protocol state from Staking to Epoch Setup phase
//   - we stop returning identities from previous+current epochs and instead returning identities from current+next epochs.
//
// As a result of this operation protocol state for the next epoch will be created.
// Returned boolean indicates if event triggered a transition in the state machine or not.
// Implementors must never return (true, error).
// Expected errors indicating that we are leaving the happy-path of the epoch transitions
//   - `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
//     CAUTION: the HappyPathStateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
//     after such error and discard the HappyPathStateMachine!
func (u *HappyPathStateMachine) ProcessEpochSetup(epochSetup *flow.EpochSetup) (bool, error) {
	err := protocol.IsValidExtendingEpochSetup(epochSetup, u.parentState.ProtocolStateEntry, u.parentState.CurrentEpochSetup)
	if err != nil {
		return false, fmt.Errorf("invalid epoch setup event: %w", err)
	}
	if u.state.NextEpoch != nil {
		return false, protocol.NewInvalidServiceEventErrorf("repeated setup for epoch %d", epochSetup.Counter)
	}

	// When observing setup event for subsequent epoch, construct the EpochStateContainer for `ProtocolStateEntry.NextEpoch`.
	// Context:
	// Note that the `EpochStateContainer.ActiveIdentities` only contains the nodes that are *active* in the next epoch. Active means
	// that these nodes are authorized to contribute to extending the chain. Nodes are listed in `ActiveIdentities` if and only if
	// they are part of the EpochSetup event for the respective epoch.
	//
	// sanity checking SAFETY-CRITICAL INVARIANT (I):
	//   - Per convention, the `flow.EpochSetup` event should list the IdentitySkeletons in canonical order. This is useful
	//     for most efficient construction of the full active Identities for an epoch. We enforce this here at the gateway
	//     to the protocol state, when we incorporate new information from the EpochSetup event.
	//   - Note that the system smart contracts manage the identity table as an unordered set! For the protocol state, we desire a fixed
	//     ordering to simplify various implementation details, like the DKG. Therefore, we order identities in `flow.EpochSetup` during
	//     conversion from cadence to Go in the function `convert.ServiceEvent(flow.ChainID, flow.Event)` in package `model/convert`
	// sanity checking SAFETY-CRITICAL INVARIANT (II):
	// While ejection status and dynamic weight are not part of the EpochSetup event, we can supplement this information as follows:
	//   - Per convention, service events are delivered (asynchronously) in an *order-preserving* manner. Furthermore, weight changes or
	//     node ejection is entirely mediated by system smart contracts and delivered via service events.
	//   - Therefore, the EpochSetup event contains the up-to-date snapshot of the epoch participants. Any weight changes or node ejection
	//     that happened before should be reflected in the EpochSetup event. Specifically, the initial weight should be reduced and ejected
	//     nodes should be no longer listed in the EpochSetup event.
	//   - Hence, the following invariant must be satisfied by the system smart contracts for all active nodes in the upcoming epoch:
	//      (i) The Ejected flag is false. Node X being ejected in epoch N (necessarily via a service event emitted by the system
	//          smart contracts earlier) but also being listed in the setup event for the subsequent epoch (service event emitted by
	//          the system smart contracts later) is illegal.
	//     (ii) When the EpochSetup event is emitted / processed, the weight of all active nodes equals their InitialWeight and

	// For collector clusters, we rely on invariants (I) and (II) holding. See `committees.Cluster` for details, specifically function
	// `constructInitialClusterIdentities(..)`. While the system smart contract must satisfy this invariant, we run a sanity check below.
	activeIdentitiesLookup := u.parentState.CurrentEpoch.ActiveIdentities.Lookup() // lookup NodeID → DynamicIdentityEntry for nodes _active_ in the current epoch
	nextEpochActiveIdentities := make(flow.DynamicIdentityEntryList, 0, len(epochSetup.Participants))
	prevNodeID := epochSetup.Participants[0].NodeID
	for idx, nextEpochIdentitySkeleton := range epochSetup.Participants {
		// sanity checking invariant (I):
		if idx > 0 && !flow.IsIdentifierCanonical(prevNodeID, nextEpochIdentitySkeleton.NodeID) {
			return false, protocol.NewInvalidServiceEventErrorf("epoch setup event lists active participants not in canonical ordering")
		}
		prevNodeID = nextEpochIdentitySkeleton.NodeID

		// sanity checking invariant (II.i):
		currentEpochDynamicProperties, found := activeIdentitiesLookup[nextEpochIdentitySkeleton.NodeID]
		if found && currentEpochDynamicProperties.Ejected { // invariance violated
			return false, protocol.NewInvalidServiceEventErrorf("node %v is ejected in current epoch %d but readmitted by EpochSetup event for epoch %d", nextEpochIdentitySkeleton.NodeID, u.parentState.CurrentEpochSetup.Counter, epochSetup.Counter)
		}

		nextEpochActiveIdentities = append(nextEpochActiveIdentities, &flow.DynamicIdentityEntry{
			NodeID:  nextEpochIdentitySkeleton.NodeID,
			Ejected: false, // according to invariant (II.i)
		})
	}

	// construct data container specifying next epoch
	u.state.NextEpoch = &flow.EpochStateContainer{
		SetupID:          epochSetup.ID(),
		CommitID:         flow.ZeroID,
		ActiveIdentities: nextEpochActiveIdentities,
	}

	// subsequent epoch commit event and update identities afterwards.
	u.nextEpochIdentitiesLookup = u.state.NextEpoch.ActiveIdentities.Lookup()
	return true, nil
}

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
func (u *HappyPathStateMachine) ProcessEpochCommit(epochCommit *flow.EpochCommit) (bool, error) {
	if u.state.NextEpoch == nil {
		return false, protocol.NewInvalidServiceEventErrorf("protocol state has been setup yet")
	}
	if u.state.NextEpoch.CommitID != flow.ZeroID {
		return false, protocol.NewInvalidServiceEventErrorf("protocol state has already a commit event")
	}
	err := protocol.IsValidExtendingEpochCommit(epochCommit, u.parentState.ProtocolStateEntry, u.parentState.NextEpochSetup)
	if err != nil {
		return false, fmt.Errorf("invalid epoch commit event: %w", err)
	}

	u.state.NextEpoch.CommitID = epochCommit.ID()
	return true, nil
}

// TransitionToNextEpoch updates the notion of 'current epoch', 'previous' and 'next epoch' in the protocol
// state. An epoch transition is only allowed when:
// - next epoch has been set up,
// - next epoch has been committed,
// - invalid state transition has not been attempted (this is ensured by constructor),
// - candidate block is in the next epoch.
// No errors are expected during normal operations.
func (u *HappyPathStateMachine) TransitionToNextEpoch() error {
	nextEpoch := u.state.NextEpoch
	// Check if there is next epoch protocol state
	if nextEpoch == nil {
		return fmt.Errorf("protocol state has not been setup yet")
	}
	// Check if there is a commit event for next epoch
	if nextEpoch.CommitID == flow.ZeroID {
		return fmt.Errorf("protocol state has not been committed yet")
	}
	// Check if we are at the next epoch, only then a transition is allowed
	if u.view < u.parentState.NextEpochSetup.FirstView {
		return fmt.Errorf("protocol state transition is only allowed when enterring next epoch")
	}
	u.state = &flow.ProtocolStateEntry{
		PreviousEpoch:                   &u.state.CurrentEpoch,
		CurrentEpoch:                    *u.state.NextEpoch,
		InvalidEpochTransitionAttempted: false,
	}
	u.rebuildIdentityLookup()
	return nil
}
