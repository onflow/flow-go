package protocol_state

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// stateMutator is a stateful object to evolve the protocol state. It is instantiated from the parent block's protocol state.
// State-changing operations can be iteratively applied and the stateMutator will internally evolve its in-memory state.
// While the StateMutator does not modify the database, it internally tracks all necessary updates. Upon calling `Build`
// the stateMutator returns the updated protocol state, its ID and all database updates necessary for persisting the updated
// protocol state.
// stateMutator locally tracks pending DB updates, which are returned as a slice of deferred DB updates when calling `Build`.
type stateMutator struct {
	headers          storage.Headers
	results          storage.ExecutionResults
	setups           storage.EpochSetups
	commits          storage.EpochCommits
	stateMachine     ProtocolStateMachine
	params           protocol.InstanceParams
	pendingDbUpdates []func(tx *transaction.Tx) error
}

var _ protocol.StateMutator = (*stateMutator)(nil)

// newStateMutator creates a new instance of stateMutator.
func newStateMutator(
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	stateMachine ProtocolStateMachine,
	params protocol.InstanceParams,
) *stateMutator {
	return &stateMutator{
		setups:       setups,
		params:       params,
		headers:      headers,
		results:      results,
		commits:      commits,
		stateMachine: stateMachine,
	}
}

// Build returns:
//   - hasChanges: flag whether there were any changes; otherwise, `updatedState` and `stateID` equal the parent state
//   - updatedState: the ProtocolState after applying all updates
//   - stateID: the hash commitment to the `updatedState`
//   - dbUpdates: database updates necessary for persisting the updated protocol state
//
// updated protocol state entry, state ID and a flag indicating if there were any changes.
func (m *stateMutator) Build() (hasChanges bool, updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, dbUpdates []func(tx *transaction.Tx) error) {
	updatedState, stateID, hasChanges = m.stateMachine.Build()
	dbUpdates = m.pendingDbUpdates
	return
}

// ApplyServiceEvents applies the state changes that are delivered via
// sealed service events:
//   - iterating over the sealed service events in order of increasing height
//   - identifying state-changing service event and calling into the embedded
//     ProtocolStateMachine to apply the respective state update
//   - tracking deferred database updates necessary to persist the updated state
//     and its data dependencies
//
// All updates only mutate the `StateMutator`'s internal in-memory copy of the
// protocol state, without changing the parent state (i.e. the state we started from).
//
// SAFETY REQUIREMENT:
// The StateMutator assumes that the proposal has passed the following correctness checks!
//   - The seals in the payload continuously follow the ancestry of this fork. Specifically,
//     there are no gaps in the seals.
//   - The seals guarantee correctness of the sealed execution result, including the contained
//     service events. This is actively checked by the verification node, whose aggregated
//     approvals in the form of a seal attestation to the correctness of the sealed execution result,
//     including the contained.
//
// Consensus nodes actively verify protocol compliance for any block proposal they receive,
// including integrity of each seal individually as well as the seals continuously following the
// fork. Light clients only process certified blocks, which guarantees that consensus nodes already
// ran those checks and found the proposal to be valid.
//
// Details on SERVICE EVENTS:
// Consider a chain where a service event is emitted during execution of block A.
// Block B contains an execution receipt for A. Block C contains a seal for block
// A's execution result.
//
//	A <- .. <- B(RA) <- .. <- C(SA)
//
// Service Events are included within execution results, which are stored
// opaquely as part of the block payload in block B. We only validate, process and persist
// the typed service event to storage once we process C, the block containing the
// seal for block A. This is because we rely on the sealing subsystem to validate
// correctness of the service event before processing it.
// Consequently, any change to the protocol state introduced by a service event
// emitted during execution of block A would only become visible when querying
// C or its descendants.
//
// Error returns:
//   - Per convention, the input seals from the block payload have already been confirmed to be protocol compliant.
//     Hence, the service events in the sealed execution results represent the honest execution path.
//     Therefore, the sealed service events should encode a valid evolution of the protocol state -- provided
//     the system smart contracts are correct.
//   - As we can rule out byzantine attacks as the source of failures, the only remaining sources of problems
//     can be (a) bugs in the system smart contracts or (b) bugs in the node implementation.
//     A service event not representing a valid state transition despite all consistency checks passing
//     is interpreted as case (a) and handled internally within the StateMutator. In short, we go into Epoch
//     Fallback Mode by copying the parent state (a valid state snapshot) and setting the
//     `InvalidStateTransitionAttempted` flag. All subsequent Epoch-lifecycle events are ignored.
//   - A consistency or sanity check failing within the StateMutator is likely the symptom of an internal bug
//     in the node software or state corruption, i.e. case (b). This is the only scenario where the error return
//     of this function is not nil. If such an exception is returned, continuing is not an option.
func (m *stateMutator) ApplyServiceEvents(seals []*flow.Seal) error {
	var dbUpdates []func(tx *transaction.Tx) error
	epochFallbackTriggered, err := m.params.EpochFallbackTriggered()
	if err != nil {
		return fmt.Errorf("could not retrieve epoch fallback status: %w", err)
	}

	parentProtocolState := m.stateMachine.ParentState()
	epochStatus := parentProtocolState.EpochStatus()
	activeSetup := parentProtocolState.CurrentEpochSetup

	// never process service events after epoch fallback is triggered
	if epochStatus.InvalidServiceEventIncorporated || epochFallbackTriggered {
		return nil
	}

	// perform protocol state transition to next epoch if next epoch is committed and we are at first block of epoch
	phase, err := epochStatus.Phase()
	if err != nil {
		return fmt.Errorf("could not determine epoch phase: %w", err)
	}
	if phase == flow.EpochPhaseCommitted {
		if m.stateMachine.View() > activeSetup.FinalView {
			// TODO: this is a temporary workaround to allow for the epoch transition to be triggered
			// most likely it will be not needed when we refactor protocol state entries and define strict safety rules.
			err = m.stateMachine.TransitionToNextEpoch()
			if err != nil {
				return fmt.Errorf("could not transition protocol state to next epoch: %w", err)
			}
		}
	}

	// We apply service events from blocks which are sealed by this candidate block.
	// The block's payload might contain epoch preparation service events for the next
	// epoch. In this case, we need to update the tentative protocol state.
	// We need to validate whether all information is available in the protocol
	// state to go to the next epoch when needed. In cases where there is a bug
	// in the smart contract, it could be that this happens too late and we should trigger epoch fallback mode.

	// block payload may not specify seals in order, so order them by block height before processing
	orderedSeals, err := protocol.OrderedSeals(seals, m.headers)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("ordering seals: parent payload contains seals for unknown block: %s", err.Error())
		}
		return fmt.Errorf("unexpected error ordering seals: %w", err)
	}
	for _, seal := range orderedSeals {
		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return fmt.Errorf("could not get result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}

		for _, event := range result.ServiceEvents {
			switch ev := event.Event.(type) {
			case *flow.EpochSetup:
				err = m.stateMachine.ProcessEpochSetup(ev)
				if err != nil {
					if protocol.IsInvalidServiceEventError(err) {
						// we have observed an invalid service event, which triggers epoch fallback mode
						m.stateMachine.SetInvalidStateTransitionAttempted()
						return nil
					}
					return irrecoverable.NewExceptionf("could not process epoch setup event: %w", err)
				}

				// we'll insert the setup event when we insert the block
				dbUpdates = append(dbUpdates, m.setups.StoreTx(ev))

			case *flow.EpochCommit:
				err = m.stateMachine.ProcessEpochCommit(ev)
				if err != nil {
					if protocol.IsInvalidServiceEventError(err) {
						// we have observed an invalid service event, which triggers epoch fallback mode
						m.stateMachine.SetInvalidStateTransitionAttempted()
						return nil
					}
					return irrecoverable.NewExceptionf("could not process epoch commit event: %w", err)
				}

				// we'll insert the commit event when we insert the block
				dbUpdates = append(dbUpdates, m.commits.StoreTx(ev))
			case *flow.VersionBeacon:
				// do nothing for now
			default:
				return fmt.Errorf("invalid service event type (type_name=%s, go_type=%T)", event.Type, ev)
			}
		}
	}

	m.pendingDbUpdates = append(m.pendingDbUpdates, dbUpdates...)
	return nil
}
