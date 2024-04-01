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

// StateMachineFactoryMethod is a factory method to create state machines for evolving the protocol's epoch state.
// Currently, we have `HappyPathStateMachine` and `FallbackStateMachine` as StateMachine
// implementations, whose constructors both have the same signature as StateMachineFactoryMethod.
type StateMachineFactoryMethod = func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (StateMachine, error)

// EpochStateMachine is a hierarchical state machine that encapsulates the logic for protocol-compliant evolution of Epoch-related sub-state.
// EpochStateMachine processes a subset of service events that are relevant for the Epoch state, and ignores all other events.
// EpochStateMachine delegates the processing of service events to an embedded StateMachine,
// which is either a HappyPathStateMachine or a FallbackStateMachine depending on the operation mode of the protocol.
// It relies on Key-Value Store to commit read the parent state and to commit the updates to the Dynamic Protocol State.
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

// NewEpochStateMachine creates a new higher-level hierarchical state machine for protocol-compliant evolution of Epoch-related sub-state.
// NewEpochStateMachine performs initialization of state machine depending on the operation mode of the protocol.
// - for the happy path, it initializes a HappyPathStateMachine,
// - for the epoch fallback mode it initializes a FallbackStateMachine.
// No errors are expected during normal operations.
func NewEpochStateMachine(
	candidate *flow.Header,
	params protocol.GlobalParams,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	protocolStateDB storage.ProtocolState,
	parentState protocol_state.KVStoreReader,
	mutator protocol_state.KVStoreMutator,
	happyPathStateMachineFactory StateMachineFactoryMethod,
	epochFallbackStateMachineFactory StateMachineFactoryMethod,
) (*EpochStateMachine, error) {
	parentEpochState, err := protocolStateDB.ByBlockID(candidate.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not query parent protocol state at block (%x): %w", candidate.ParentID, err)
	}

	// sanity check: the parent epoch state ID must be set in KV store
	if parentEpochState.ID() != parentState.GetEpochStateID() {
		return nil, irrecoverable.NewExceptionf("broken invariant: parent epoch state ID mismatch, expected %x, got %x",
			parentState.GetEpochStateID(), parentEpochState.ID())
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
		stateMachine, err = epochFallbackStateMachineFactory(candidate.View, parentEpochState)
	} else {
		stateMachine, err = happyPathStateMachineFactory(candidate.View, parentEpochState)
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
			return epochFallbackStateMachineFactory(candidate.View, parentEpochState)
		},
		setups:           setups,
		commits:          commits,
		protocolStateDB:  protocolStateDB,
		pendingDbUpdates: nil,
	}, nil
}

// Build schedules updates to the protocol state by obtaining the updated state from the active state machine,
// preparing deferred DB updates and committing updated sub-state ID to the KV store.
// ATTENTION: In mature implementation all parts of the Dynamic Protocol State will rely on the Key-Value Store as storage
// but to avoid a large refactoring we are using a hybrid approach where only the epoch state ID is stored in the KV Store
// but the actual epoch state is stored separately, nevertheless, the epoch state ID is used to sanity check if the
// epoch state is consistent with the KV Store. Using this approach, we commit the epoch sub-state to the KV Store which in
// affects the Dynamic Protocol State ID which is essentially hash of the KV Store.
func (e *EpochStateMachine) Build() []transaction.DeferredDBUpdate {
	updatedEpochState, updatedStateID, hasChanges := e.activeStateMachine.Build()
	dbUpdates := e.pendingDbUpdates
	dbUpdates = append(dbUpdates, e.protocolStateDB.Index(e.candidate.ID(), updatedStateID))
	if hasChanges {
		dbUpdates = append(dbUpdates, operation.SkipDuplicatesTx(e.protocolStateDB.StoreTx(updatedStateID, updatedEpochState)))
	}
	e.mutator.SetEpochStateID(updatedStateID)

	return dbUpdates
}

// ProcessUpdate applies the state changes that are delivered via sealed service events.
// The block's payload might contain epoch preparation service events for the next
// epoch. In this case, we need to update the tentative protocol state.
// We need to validate whether all information is available in the protocol
// state to go to the next epoch when needed. In cases where there is a bug
// in the smart contract, it could be that this happens too late, and we should trigger epoch fallback mode.
// No errors are expected during normal operations.
func (e *EpochStateMachine) ProcessUpdate(update []*flow.ServiceEvent) error {
	parentProtocolState := e.activeStateMachine.ParentState()

	// perform protocol state transition to next epoch if next epoch is committed, and we are at first block of epoch
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

// View returns the view that is associated with this EpochStateMachine.
func (e *EpochStateMachine) View() uint64 {
	return e.activeStateMachine.View()
}

// ParentState returns parent state that is associated with this state machine.
func (e *EpochStateMachine) ParentState() protocol_state.KVStoreReader {
	return e.parentState
}

// applyServiceEventsFromOrderedResults applies the service events contained within the list of results
// to the pending state tracked by `stateMutator`.
// Each result corresponds to one seal that was included in the payload of the block being processed by this `stateMutator`.
// Results must be ordered by block height.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if any service event is invalid or is not a valid state transition for the current protocol state
func (e *EpochStateMachine) applyServiceEventsFromOrderedResults(orderedUpdates []*flow.ServiceEvent) ([]transaction.DeferredDBUpdate, error) {
	var dbUpdates []transaction.DeferredDBUpdate
	for _, event := range orderedUpdates {
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
func (e *EpochStateMachine) transitionToEpochFallbackMode(orderedUpdates []*flow.ServiceEvent) ([]transaction.DeferredDBUpdate, error) {
	var err error
	e.activeStateMachine, err = e.epochFallbackStateMachineFactory()
	if err != nil {
		return nil, fmt.Errorf("could not create epoch fallback state machine: %w", err)
	}
	dbUpdates, err := e.applyServiceEventsFromOrderedResults(orderedUpdates)
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
