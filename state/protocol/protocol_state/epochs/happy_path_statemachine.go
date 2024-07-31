package epochs

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
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
// tracks the current protocol state. A separate instance should be created for each block to process the updates therein.
// See flow.EpochPhase for detailed documentation about EFM and epoch phase transitions.
type HappyPathStateMachine struct {
	baseStateMachine
}

var _ StateMachine = (*HappyPathStateMachine)(nil)

// NewHappyPathStateMachine creates a new HappyPathStateMachine.
// An exception is returned in case the `EpochFallbackTriggered` flag is set in the `parentEpochState`. This means that
// the protocol state evolution has reached an undefined state from the perspective of the happy path state machine.
// No errors are expected during normal operations.
func NewHappyPathStateMachine(telemetry protocol_state.StateMachineTelemetryConsumer, view uint64, parentState *flow.RichEpochStateEntry) (*HappyPathStateMachine, error) {
	if parentState.EpochFallbackTriggered {
		return nil, irrecoverable.NewExceptionf("cannot create happy path protocol state machine at view (%d) for a parent state"+
			"which is in Epoch Fallback Mode", view)
	}
	base, err := newBaseStateMachine(telemetry, view, parentState, parentState.EpochStateEntry.Copy())
	if err != nil {
		return nil, fmt.Errorf("could not create base state machine: %w", err)
	}
	return &HappyPathStateMachine{
		baseStateMachine: *base,
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
	u.telemetry.OnServiceEventReceived(epochSetup.ServiceEvent())
	err := protocol.IsValidExtendingEpochSetup(epochSetup, u.state)
	if err != nil {
		u.telemetry.OnInvalidServiceEvent(epochSetup.ServiceEvent(), err)
		return false, fmt.Errorf("invalid epoch setup event for epoch %d: %w", epochSetup.Counter, err)
	}
	if u.state.NextEpoch != nil {
		err := protocol.NewInvalidServiceEventErrorf("repeated EpochSetup event for epoch %d", epochSetup.Counter)
		u.telemetry.OnInvalidServiceEvent(epochSetup.ServiceEvent(), err)
		return false, err
	}

	// When observing setup event for subsequent epoch, construct the EpochStateContainer for `MinEpochStateEntry.NextEpoch`.
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
	activeIdentitiesLookup := u.state.CurrentEpoch.ActiveIdentities.Lookup() // lookup NodeID → DynamicIdentityEntry for nodes _active_ in the current epoch
	nextEpochActiveIdentities, err := buildNextEpochActiveParticipants(activeIdentitiesLookup, u.state.CurrentEpochSetup, epochSetup)
	if err != nil {
		u.telemetry.OnInvalidServiceEvent(epochSetup.ServiceEvent(), err)
		return false, fmt.Errorf("failed to construct next epoch active participants: %w", err)
	}

	// construct data container specifying next epoch
	u.state.NextEpoch = &flow.EpochStateContainer{
		SetupID:          epochSetup.ID(),
		CommitID:         flow.ZeroID,
		ActiveIdentities: nextEpochActiveIdentities,
	}
	u.state.NextEpochSetup = epochSetup

	// subsequent epoch commit event and update identities afterwards.
	err = u.ejector.TrackDynamicIdentityList(u.state.NextEpoch.ActiveIdentities)
	if err != nil {
		if protocol.IsInvalidServiceEventError(err) {
			u.telemetry.OnInvalidServiceEvent(epochSetup.ServiceEvent(), err)
		}
		return false, fmt.Errorf("failed to track dynamic identity list for next epoch: %w", err)

	}
	u.telemetry.OnServiceEventProcessed(epochSetup.ServiceEvent())
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
	u.telemetry.OnServiceEventReceived(epochCommit.ServiceEvent())
	if u.state.NextEpoch == nil {
		err := protocol.NewInvalidServiceEventErrorf("received EpochCommit without prior EpochSetup")
		u.telemetry.OnInvalidServiceEvent(epochCommit.ServiceEvent(), err)
		return false, err
	}
	if u.state.NextEpoch.CommitID != flow.ZeroID {
		err := protocol.NewInvalidServiceEventErrorf("repeated EpochCommit event for epoch %d", epochCommit.Counter)
		u.telemetry.OnInvalidServiceEvent(epochCommit.ServiceEvent(), err)
		return false, err
	}
	err := protocol.IsValidExtendingEpochCommit(epochCommit, u.state.MinEpochStateEntry, u.state.NextEpochSetup)
	if err != nil {
		u.telemetry.OnInvalidServiceEvent(epochCommit.ServiceEvent(), err)
		return false, fmt.Errorf("invalid epoch commit event for epoch %d: %w", epochCommit.Counter, err)
	}

	u.state.NextEpoch.CommitID = epochCommit.ID()
	u.state.NextEpochCommit = epochCommit
	u.telemetry.OnServiceEventProcessed(epochCommit.ServiceEvent())
	return true, nil
}

// ProcessEpochRecover returns the sentinel error `protocol.InvalidServiceEventError`, which
// indicates that `EpochRecover` are not expected on the happy path of epoch lifecycle.
func (u *HappyPathStateMachine) ProcessEpochRecover(epochRecover *flow.EpochRecover) (bool, error) {
	u.telemetry.OnServiceEventReceived(epochRecover.ServiceEvent())
	err := protocol.NewInvalidServiceEventErrorf("epoch recover event for epoch %d received while on happy path", epochRecover.EpochSetup.Counter)
	u.telemetry.OnInvalidServiceEvent(epochRecover.ServiceEvent(), err)
	return false, err
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
// This is a side-effect-free function. This function only returns protocol.InvalidServiceEventError as errors.
func buildNextEpochActiveParticipants(activeIdentitiesLookup map[flow.Identifier]*flow.DynamicIdentityEntry, currentEpochSetup, nextEpochSetup *flow.EpochSetup) (flow.DynamicIdentityEntryList, error) {
	nextEpochActiveIdentities := make(flow.DynamicIdentityEntryList, 0, len(nextEpochSetup.Participants))
	prevNodeID := nextEpochSetup.Participants[0].NodeID
	for idx, nextEpochIdentitySkeleton := range nextEpochSetup.Participants {
		// sanity checking invariant (I):
		if idx > 0 && !flow.IsIdentifierCanonical(prevNodeID, nextEpochIdentitySkeleton.NodeID) {
			return nil, protocol.NewInvalidServiceEventErrorf("epoch setup event lists active participants not in canonical ordering")
		}
		prevNodeID = nextEpochIdentitySkeleton.NodeID

		// sanity checking invariant (II.i):
		currentEpochDynamicProperties, found := activeIdentitiesLookup[nextEpochIdentitySkeleton.NodeID]
		if found && currentEpochDynamicProperties.Ejected { // invariant violated
			return nil, protocol.NewInvalidServiceEventErrorf("node %v is ejected in current epoch %d but readmitted by EpochSetup event for epoch %d", nextEpochIdentitySkeleton.NodeID, currentEpochSetup.Counter, nextEpochSetup.Counter)
		}

		nextEpochActiveIdentities = append(nextEpochActiveIdentities, &flow.DynamicIdentityEntry{
			NodeID:  nextEpochIdentitySkeleton.NodeID,
			Ejected: false, // according to invariant (II.i)
		})
	}
	return nextEpochActiveIdentities, nil
}
