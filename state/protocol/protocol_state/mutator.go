package protocol_state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// StateMachineFactoryMethod is a factory to create state machines for evolving the protocol state.
// Currently, we have `protocolStateMachine` and `epochFallbackStateMachine` as ProtocolStateMachine
// implementations, whose constructors both have the same signature as StateMachineFactoryMethod.
type StateMachineFactoryMethod = func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (ProtocolStateMachine, error)

// stateMutator is a stateful object to evolve the protocol state. It is instantiated from the parent block's protocol state.
// State-changing operations can be iteratively applied and the stateMutator will internally evolve its in-memory state.
// While the StateMutator does not modify the database, it internally tracks the necessary database updates to persist its
// dependencies (specifically EpochSetup and EpochCommit events). Upon calling `Build` the stateMutator returns the updated
// protocol state, its ID and all database updates necessary for persisting the updated protocol state.
//
// The StateMutator is used by a replica's compliance layer to update protocol state when observing state-changing service in
// blocks. It is used by the primary in the block building process to obtain the correct protocol state for a proposal.
// Specifically, the leader may include state-changing service events in the block payload. The flow protocol prescribes that
// the proposal needs to include the ID of the protocol state, _after_ processing the payload incl. all state-changing events.
// Therefore, the leader instantiates a StateMutator, applies the service events to it and builds the updated protocol state ID.
//
// Not safe for concurrent use.
type stateMutator struct {
	headers                          storage.Headers
	results                          storage.ExecutionResults
	setups                           storage.EpochSetups
	commits                          storage.EpochCommits
	stateMachine                     ProtocolStateMachine
	epochFallbackStateMachineFactory func() (ProtocolStateMachine, error)
	pendingDbUpdates                 transaction.DeferredDBUpdates
}

var _ protocol.StateMutator = (*stateMutator)(nil)

// newStateMutator creates a new instance of stateMutator.
// stateMutator performs initialization of state machine depending on the operation mode of the protocol.
// No errors are expected during normal operations.
func newStateMutator(
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	params protocol.GlobalParams,
	candidateView uint64,
	parentState *flow.RichProtocolStateEntry,
	happyPathStateMachineFactory StateMachineFactoryMethod,
	epochFallbackStateMachineFactory StateMachineFactoryMethod,
) (*stateMutator, error) {
	var (
		stateMachine ProtocolStateMachine
		err          error
	)
	candidateAttemptsInvalidEpochTransition := epochFallbackTriggeredByIncorporatingCandidate(candidateView, params, parentState)
	if parentState.InvalidEpochTransitionAttempted || candidateAttemptsInvalidEpochTransition {
		// Case 1: InvalidEpochTransitionAttempted is true, indicating that we have encountered an invalid
		//         epoch service event or an invalid state transition previously in this fork.
		// Case 2: Incorporating the candidate block is itself an invalid epoch transition.
		//
		// In either case, Epoch Fallback Mode [EFM] has been tentatively triggered on this fork,
		// and we must use only the `epochFallbackStateMachine` along this fork.
		//
		// TODO for 'leaving Epoch Fallback via special service event': this might need to change.
		stateMachine, err = epochFallbackStateMachineFactory(candidateView, parentState)
	} else {
		stateMachine, err = happyPathStateMachineFactory(candidateView, parentState)
	}
	if err != nil {
		return nil, fmt.Errorf("could not initialize protocol state machine: %w", err)
	}

	return &stateMutator{
		headers:      headers,
		results:      results,
		setups:       setups,
		commits:      commits,
		stateMachine: stateMachine,
		// instead of storing arguments that later might be used when entering EFM, capture them in factory method.
		epochFallbackStateMachineFactory: func() (ProtocolStateMachine, error) {
			return epochFallbackStateMachineFactory(candidateView, parentState)
		},
	}, nil
}

// Build returns:
//   - hasChanges: flag whether there were any changes; otherwise, `updatedState` and `stateID` equal the parent state
//   - updatedState: the ProtocolState after applying all updates.
//   - stateID: the hash commitment to the `updatedState`
//   - dbUpdates: database updates necessary for persisting the updated protocol state's *dependencies*.
//     If hasChanges is false, updatedState is empty. Caution: persisting the `updatedState` itself and adding
//     it to the relevant indices is _not_ in `dbUpdates`. Persisting and indexing `updatedState` is the responsibility
//     of the calling code (specifically `FollowerState`).
//
// updated protocol state entry, state ID and a flag indicating if there were any changes.
func (m *stateMutator) Build() (hasChanges bool, updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, dbUpdates transaction.DeferredDBUpdates) {
	updatedState, stateID, hasChanges = m.stateMachine.Build()
	dbUpdates = m.pendingDbUpdates
	return
}

// ApplyServiceEventsFromValidatedSeals applies the state changes that are delivered via
// sealed service events:
//   - iterating over the sealed service events in order of increasing height
//   - identifying state-changing service event and calling into the embedded
//     ProtocolStateMachine to apply the respective state update
//   - tracking deferred database updates necessary to persist the updated
//     protocol state's *dependencies*. Persisting and indexing `updatedState`
//     is the responsibility of the calling code (specifically `FollowerState`)
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
//     approvals in the form of a seal attest to the correctness of the sealed execution result,
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
//     `InvalidEpochTransitionAttempted` flag. All subsequent Epoch-lifecycle events are ignored.
//   - A consistency or sanity check failing within the StateMutator is likely the symptom of an internal bug
//     in the node software or state corruption, i.e. case (b). This is the only scenario where the error return
//     of this function is not nil. If such an exception is returned, continuing is not an option.
func (m *stateMutator) ApplyServiceEventsFromValidatedSeals(seals []*flow.Seal) error {
	parentProtocolState := m.stateMachine.ParentState()

	// perform protocol state transition to next epoch if next epoch is committed and we are at first block of epoch
	phase := parentProtocolState.EpochPhase()
	if phase == flow.EpochPhaseCommitted {
		activeSetup := parentProtocolState.CurrentEpochSetup
		if m.stateMachine.View() > activeSetup.FinalView {
			// TODO: this is a temporary workaround to allow for the epoch transition to be triggered
			// most likely it will be not needed when we refactor protocol state entries and define strict safety rules.
			err := m.stateMachine.TransitionToNextEpoch()
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
		// Per API contract, the input seals must have already passed verification, which necessitates
		// successful ordering. Hence, calling protocol.OrderedSeals with the same inputs that succeeded
		// earlier now failed. In all cases, this is an exception.
		return irrecoverable.NewExceptionf("ordering already validated seals unexpectedly failed: %w", err)
	}
	results := make([]*flow.ExecutionResult, 0, len(orderedSeals))
	for _, seal := range orderedSeals {
		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return fmt.Errorf("could not get result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}
		results = append(results, result)
	}
	dbUpdates, err := m.applyServiceEventsFromOrderedResults(results)
	if err != nil {
		if protocol.IsInvalidServiceEventError(err) {
			dbUpdates, err = m.transitionToEpochFallbackMode(results)
			if err != nil {
				return irrecoverable.NewExceptionf("could not transition to epoch fallback mode: %w", err)
			}
		} else {
			return irrecoverable.NewExceptionf("could not apply service events from ordered results: %w", err)
		}
	}
	m.pendingDbUpdates = append(m.pendingDbUpdates, dbUpdates...)
	return nil
}

// applyServiceEventsFromOrderedResults applies the service events contained within the list of results
// to the pending state tracked by `stateMutator`.
// Each result corresponds to one seal that was included in the payload of the block being processed by this `stateMutator`.
// Results must be ordered by block height.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if any service event is invalid or is not a valid state transition for the current protocol state
func (m *stateMutator) applyServiceEventsFromOrderedResults(results []*flow.ExecutionResult) ([]func(tx *transaction.Tx) error, error) {
	var dbUpdates transaction.DeferredDBUpdates
	for _, result := range results {
		for _, event := range result.ServiceEvents {
			switch ev := event.Event.(type) {
			case *flow.EpochSetup:
				processed, err := m.stateMachine.ProcessEpochSetup(ev)
				if err != nil {
					return nil, fmt.Errorf("could not process epoch setup event: %w", err)
				}

				if processed {
					// we'll insert the setup event when we insert the block
					dbUpdates = append(dbUpdates, m.setups.StoreTx(ev))
				}

			case *flow.EpochCommit:
				processed, err := m.stateMachine.ProcessEpochCommit(ev)
				if err != nil {
					return nil, fmt.Errorf("could not process epoch commit event: %w", err)
				}

				if processed {
					// we'll insert the commit event when we insert the block
					dbUpdates = append(dbUpdates, m.commits.StoreTx(ev))
				}
			case *flow.VersionBeacon:
				// do nothing for now
			default:
				return nil, fmt.Errorf("invalid service event type (type_name=%s, go_type=%T)", event.Type, ev)
			}
		}
	}
	return dbUpdates, nil
}

// transitionToEpochFallbackMode transitions the protocol state to Epoch Fallback Mode [EFM].
// This is implemented by switching to a different state machine implementation, which ignores all service events and epoch transitions.
// At the moment, this is a one-way transition: once we enter EFM, the only way to return to normal is with a spork.
func (m *stateMutator) transitionToEpochFallbackMode(results []*flow.ExecutionResult) ([]func(tx *transaction.Tx) error, error) {
	var err error
	m.stateMachine, err = m.epochFallbackStateMachineFactory()
	if err != nil {
		return nil, fmt.Errorf("could not create epoch fallback state machine: %w", err)
	}
	dbUpdates, err := m.applyServiceEventsFromOrderedResults(results)
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not apply service events after transition to epoch fallback mode: %w", err)
	}
	return dbUpdates, nil
}

// epochFallbackTriggeredByIncorporatingCandidate checks whether incorporating the input block B
// would trigger epoch fallback mode [EFM] along the current fork. We trigger epoch fallback mode
// when:
//  1. The next epoch has not been committed as of B (EpochPhase ≠ flow.EpochPhaseCommitted) AND
//  2. B is the first incorporated block with view greater than or equal to the epoch commitment
//     deadline for the current epoch
//
// In protocol terms, condition 1 means that an EpochCommit service event for the upcoming epoch has
// not yet been sealed as of block B. Formally, a service event S is considered sealed as of block B if:
//   - S was emitted during execution of some block A, s.t. A is an ancestor of B.
//   - The seal for block A was included in some block C, s.t C is an ancestor of B.
//
// For further details see `params.EpochCommitSafetyThreshold()`.
func epochFallbackTriggeredByIncorporatingCandidate(candidateView uint64, params protocol.GlobalParams, parentState *flow.RichProtocolStateEntry) bool {
	if parentState.EpochPhase() == flow.EpochPhaseCommitted { // Requirement 1
		return false
	}
	return candidateView+params.EpochCommitSafetyThreshold() >= parentState.CurrentEpochSetup.FinalView // Requirement 2
}
