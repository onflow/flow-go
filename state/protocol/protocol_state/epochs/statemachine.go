package epochs

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// StateMachine implements a low-level interface for state-changing operations on the protocol state.
// It is used by higher level logic to evolve the protocol state when certain events that are stored in blocks are observed.
// The StateMachine is stateful and internally tracks the current protocol state. A separate instance is created for
// each block that is being processed.
type StateMachine interface {
	// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
	// CAUTION:
	// Do NOT call Build, if the StateMachine instance has returned a `protocol.InvalidServiceEventError`
	// at any time during its lifetime. After this error, the StateMachine is left with a potentially
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

	// View returns the view that is associated with this StateMachine.
	// The view of the StateMachine equals the view of the block carrying the respective updates.
	View() uint64

	// ParentState returns parent protocol state that is associated with this StateMachine.
	ParentState() *flow.RichProtocolStateEntry
}

type EpochStateMachine struct {
	candidate                        *flow.Header
	activeStateMachine               StateMachine
	parentState                      protocol_state.KVStoreReader
	mutator                          protocol_state.KVStoreMutator
	epochFallbackStateMachineFactory func() (StateMachine, error)

	setups           storage.EpochSetups
	commits          storage.EpochCommits
	protocolStateDB  storage.ProtocolState
	pendingDbUpdates []transaction.DeferredDBUpdate
}

var _ protocol_state.KeyValueStoreStateMachine = (*EpochStateMachine)(nil)

func NewEpochStateMachine(
	candidate *flow.Header,
	params protocol.GlobalParams,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	protocolStateDB storage.ProtocolState,
	parentState protocol_state.KVStoreReader,
	mutator protocol_state.KVStoreMutator,
) (*EpochStateMachine, error) {
	parentEpochState, err := protocolStateDB.ByBlockID(candidate.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not query parent protocol state at block (%x): %w", candidate.ParentID, err)
	}
	var (
		stateMachine StateMachine
	)
	candidateAttemptsInvalidEpochTransition := epochFallbackTriggeredByIncorporatingCandidate(candidate.View, params, parentEpochState)
	if parentEpochState.InvalidEpochTransitionAttempted || candidateAttemptsInvalidEpochTransition {
		// Case 1: InvalidEpochTransitionAttempted is true, indicating that we have encountered an invalid
		//         epoch service event or an invalid state transition previously in this fork.
		// Case 2: Incorporating the candidate block is itself an invalid epoch transition.
		//
		// In either case, Epoch Fallback Mode [EFM] has been tentatively triggered on this fork,
		// and we must use only the `epochFallbackStateMachine` along this fork.
		//
		// TODO for 'leaving Epoch Fallback via special service event': this might need to change.
		stateMachine = NewEpochFallbackStateMachine(candidate.View, parentEpochState)
	} else {
		stateMachine, err = NewStateMachine(candidate.View, parentEpochState)
	}
	if err != nil {
		return nil, fmt.Errorf("could not initialize protocol state machine: %w", err)
	}

	return &EpochStateMachine{
		candidate:          candidate,
		activeStateMachine: stateMachine,
		parentState:        parentState,
		mutator:            mutator,
		epochFallbackStateMachineFactory: func() (StateMachine, error) {
			return NewEpochFallbackStateMachine(candidate.View, parentEpochState), nil
		},
		setups:           setups,
		commits:          commits,
		protocolStateDB:  protocolStateDB,
		pendingDbUpdates: nil,
	}, nil
}

func (e *EpochStateMachine) Build() []transaction.DeferredDBUpdate {
	updatedEpochState, updatedStateID, hasChanges := e.activeStateMachine.Build()
	dbUpdates := e.pendingDbUpdates
	dbUpdates = append(dbUpdates, e.protocolStateDB.Index(e.candidate.ID(), updatedStateID))
	if hasChanges {
		dbUpdates = append(dbUpdates, operation.SkipDuplicatesTx(e.protocolStateDB.StoreTx(updatedStateID, updatedEpochState)))
	}
	e.mutator.SetEpochsStateID(updatedStateID)

	return dbUpdates
}

// ApplyServiceEventsFromValidatedSeals applies the state changes that are delivered via
// sealed service events:
//   - iterating over the sealed service events in order of increasing height
//   - identifying state-changing service event and calling into the embedded
//     StateMachine to apply the respective state update
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
func (e *EpochStateMachine) ProcessUpdate(update []*flow.ServiceEvent) error {
	parentProtocolState := e.activeStateMachine.ParentState()

	// perform protocol state transition to next epoch if next epoch is committed and we are at first block of epoch
	phase := parentProtocolState.EpochPhase()
	if phase == flow.EpochPhaseCommitted {
		activeSetup := parentProtocolState.CurrentEpochSetup
		if e.activeStateMachine.View() > activeSetup.FinalView {
			err := e.activeStateMachine.TransitionToNextEpoch()
			if err != nil {
				return fmt.Errorf("could not transition protocol state to next epoch: %w", err)
			}
		}
	}

	dbUpdates, err := e.applyServiceEventsFromOrderedResults(update)
	if err != nil {
		if protocol.IsInvalidServiceEventError(err) {
			dbUpdates, err = e.transitionToEpochFallbackMode(update)
			if err != nil {
				return irrecoverable.NewExceptionf("could not transition to epoch fallback mode: %w", err)
			}
		} else {
			return irrecoverable.NewExceptionf("could not apply service events from ordered results: %w", err)
		}
	}
	e.pendingDbUpdates = append(e.pendingDbUpdates, dbUpdates...)
	return nil
}

func (e *EpochStateMachine) View() uint64 {
	return e.activeStateMachine.View()
}

func (e *EpochStateMachine) ParentState() protocol_state.KVStoreReader {
	return e.parentState
}

// applyServiceEventsFromOrderedResults applies the service events contained within the list of results
// to the pending state tracked by `stateMutator`.
// Each result corresponds to one seal that was included in the payload of the block being processed by this `stateMutator`.
// Results must be ordered by block height.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if any service event is invalid or is not a valid state transition for the current protocol state
func (e *EpochStateMachine) applyServiceEventsFromOrderedResults(events []*flow.ServiceEvent) ([]func(tx *transaction.Tx) error, error) {
	var dbUpdates []transaction.DeferredDBUpdate
	for _, event := range events {
		switch ev := event.Event.(type) {
		case *flow.EpochSetup:
			processed, err := e.activeStateMachine.ProcessEpochSetup(ev)
			if err != nil {
				return nil, fmt.Errorf("could not process epoch setup event: %w", err)
			}

			if processed {
				// we'll insert the setup event when we insert the block
				dbUpdates = append(dbUpdates, e.setups.StoreTx(ev))
			}

		case *flow.EpochCommit:
			processed, err := e.activeStateMachine.ProcessEpochCommit(ev)
			if err != nil {
				return nil, fmt.Errorf("could not process epoch commit event: %w", err)
			}

			if processed {
				// we'll insert the commit event when we insert the block
				dbUpdates = append(dbUpdates, e.commits.StoreTx(ev))
			}
		default:
			continue
		}
	}
	return dbUpdates, nil
}

// transitionToEpochFallbackMode transitions the protocol state to Epoch Fallback Mode [EFM].
// This is implemented by switching to a different state machine implementation, which ignores all service events and epoch transitions.
// At the moment, this is a one-way transition: once we enter EFM, the only way to return to normal is with a spork.
func (e *EpochStateMachine) transitionToEpochFallbackMode(events []*flow.ServiceEvent) ([]func(tx *transaction.Tx) error, error) {
	var err error
	e.activeStateMachine, err = e.epochFallbackStateMachineFactory()
	if err != nil {
		return nil, fmt.Errorf("could not create epoch fallback state machine: %w", err)
	}
	dbUpdates, err := e.applyServiceEventsFromOrderedResults(events)
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
