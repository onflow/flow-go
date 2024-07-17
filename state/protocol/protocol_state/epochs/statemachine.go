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

// StateMachine implements a low-level interface for state-changing operations on the Epoch state.
// It is used by higher level logic to coordinate the Epoch handover, evolving its internal state
// when Epoch-related Service Events are sealed or specific view-thresholds are reached.
//
// The StateMachine is fork-aware, in that it starts with the Epoch state of the parent block and
// evolves the state based on the relevant information in the child block (specifically Service Events
// sealed in the child block and the child block's view). A separate instance must be created for each
// block that is being processed. Calling `Build()` constructs a snapshot of the resulting Epoch state.
type StateMachine interface {
	// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
	// CAUTION:
	// Do NOT call Build, if the StateMachine instance has returned a `protocol.InvalidServiceEventError`
	// at any time during its lifetime. After this error, the StateMachine is left with a potentially
	// dysfunctional state and should be discarded.
	Build() (updatedState *flow.EpochStateEntry, stateID flow.Identifier, hasChanges bool)

	// ProcessEpochSetup updates the internally-maintained interim Epoch state with data from epoch setup event.
	// Processing an epoch setup event also affects the identity table for the current epoch.
	// Specifically, we transition the Epoch state from staking to setup phase, we stop returning
	// identities from previous+current epochs and start returning identities from current+next epochs.
	// As a result of this operation protocol state for the next epoch will be created.
	// Returned boolean indicates if event triggered a transition in the state machine or not.
	// Implementors must never return (true, error).
	// Expected errors indicating that we are leaving the happy-path of the epoch transitions
	//   - `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
	//     CAUTION: the StateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
	//     after such error and discard the StateMachine!
	ProcessEpochSetup(epochSetup *flow.EpochSetup) (bool, error)

	// ProcessEpochCommit updates the internally-maintained interim Epoch state with data from epoch commit event.
	// Observing an epoch setup commit, transitions protocol state from setup to commit phase.
	// At this point, we have finished construction of the next epoch.
	// As a result of this operation protocol state for next epoch will be committed.
	// Returned boolean indicates if event triggered a transition in the state machine or not.
	// Implementors must never return (true, error).
	// Expected errors indicating that we are leaving the happy-path of the epoch transitions
	//   - `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
	//     CAUTION: the StateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
	//     after such error and discard the StateMachine!
	ProcessEpochCommit(epochCommit *flow.EpochCommit) (bool, error)

	// ProcessEpochRecover updates the internally-maintained interim Epoch state with data from epoch recover
	// event in an attempt to recover from Epoch Fallback Mode [EFM] and get back on happy path.
	// Specifically, after successfully processing this event, we will have a next epoch (as specified by the
	// EpochRecover event) in the protocol state, which is in the committed phase. Subsequently, the epoch
	// protocol can proceed following the happy path. Therefore, we set `EpochFallbackTriggered` back to false.
	//
	// The boolean return indicates if the input event triggered a transition in the state machine or not.
	// For the EpochRecover event, we return false if and only if there is an error. The reason is that
	// either the `EpochRecover` event is rejected (leading to `InvalidServiceEventError`) or there is an
	// exception processing the event. Otherwise, an `EpochRecover` event must always lead to a state change.
	// Expected errors during normal operations:
	//   - `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
	ProcessEpochRecover(epochRecover *flow.EpochRecover) (bool, error)

	// EjectIdentity updates the identity table by changing the node's participation status to 'ejected'
	// If and only if the node is active in the previous or current or next epoch, the node's ejection status
	// is set to true for all occurrences, and we return true.  If `nodeID` is not found, we return false. This
	// method is idempotent and behaves identically for repeated calls with the same `nodeID` (repeated calls
	// with the same input create minor performance overhead though).
	EjectIdentity(nodeID flow.Identifier) bool

	// TransitionToNextEpoch transitions our reference frame of 'current epoch' to the pending but committed epoch.
	// Epoch transition is only allowed when:
	// - next epoch has been committed,
	// - candidate block is in the next epoch.
	// No errors are expected during normal operations.
	TransitionToNextEpoch() error

	// View returns the view associated with this state machine.
	// The view of the state machine equals the view of the block carrying the respective updates.
	View() uint64

	// ParentState returns parent protocol state associated with this state machine.
	ParentState() *flow.RichEpochStateEntry
}

// StateMachineFactoryMethod is a factory method to create state machines for evolving the protocol's epoch state.
// Currently, we have `HappyPathStateMachine` and `FallbackStateMachine` as StateMachine
// implementations, whose constructors both have the same signature as StateMachineFactoryMethod.
type StateMachineFactoryMethod func(candidateView uint64, parentState *flow.RichEpochStateEntry) (StateMachine, error)

// EpochStateMachine is a hierarchical state machine that encapsulates the logic for protocol-compliant evolution of Epoch-related sub-state.
// EpochStateMachine processes a subset of service events that are relevant for the Epoch state, and ignores all other events.
// EpochStateMachine delegates the processing of service events to an embedded StateMachine,
// which is either a HappyPathStateMachine or a FallbackStateMachine depending on the operation mode of the protocol.
// It relies on Key-Value Store to read the parent state and to persist the snapshot of the updated Epoch state.
type EpochStateMachine struct {
	activeStateMachine               StateMachine
	parentState                      protocol.KVStoreReader
	mutator                          protocol_state.KVStoreMutator
	epochFallbackStateMachineFactory func() (StateMachine, error)

	setups               storage.EpochSetups
	commits              storage.EpochCommits
	epochProtocolStateDB storage.EpochProtocolStateEntries
	pendingDbUpdates     *transaction.DeferredBlockPersist
}

var _ protocol_state.KeyValueStoreStateMachine = (*EpochStateMachine)(nil)

// NewEpochStateMachine creates a new higher-level hierarchical state machine for protocol-compliant evolution of Epoch-related sub-state.
// NewEpochStateMachine performs initialization of state machine depending on the operation mode of the protocol.
// - for the happy path, it initializes a HappyPathStateMachine,
// - for the epoch fallback mode it initializes a FallbackStateMachine.
// No errors are expected during normal operations.
func NewEpochStateMachine(
	candidateView uint64,
	parentBlockID flow.Identifier,
	params protocol.GlobalParams,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	epochProtocolStateDB storage.EpochProtocolStateEntries,
	parentState protocol.KVStoreReader,
	mutator protocol_state.KVStoreMutator,
	happyPathStateMachineFactory StateMachineFactoryMethod,
	epochFallbackStateMachineFactory StateMachineFactoryMethod,
) (*EpochStateMachine, error) {
	parentEpochState, err := epochProtocolStateDB.ByBlockID(parentBlockID)
	if err != nil {
		return nil, fmt.Errorf("could not query parent protocol state at block (%x): %w", parentBlockID, err)
	}

	// sanity check: the parent epoch state ID must be set in KV store
	if parentEpochState.ID() != parentState.GetEpochStateID() {
		return nil, irrecoverable.NewExceptionf("broken invariant: parent epoch state ID mismatch, expected %x, got %x",
			parentState.GetEpochStateID(), parentEpochState.ID())
	}

	var stateMachine StateMachine
	candidateTriggersEpochFallback := epochFallbackTriggeredByIncorporatingCandidate(candidateView, params, parentEpochState)
	if parentEpochState.EpochFallbackTriggered || candidateTriggersEpochFallback {
		// Case 1: EpochFallbackTriggered is true, indicating that we have encountered an invalid
		//         epoch service event or an invalid state transition previously in this fork.
		// Case 2: Incorporating the candidate block is itself an invalid epoch transition.
		//
		// In either case, Epoch Fallback Mode [EFM] has been tentatively triggered on this fork,
		// and we must use only the `epochFallbackStateMachine` along this fork.
		//
		// TODO for 'leaving Epoch Fallback via special service event': this might need to change.
		stateMachine, err = epochFallbackStateMachineFactory(candidateView, parentEpochState)
	} else {
		stateMachine, err = happyPathStateMachineFactory(candidateView, parentEpochState)
	}
	if err != nil {
		return nil, fmt.Errorf("could not initialize protocol state machine: %w", err)
	}

	return &EpochStateMachine{
		activeStateMachine: stateMachine,
		parentState:        parentState,
		mutator:            mutator,
		epochFallbackStateMachineFactory: func() (StateMachine, error) {
			return epochFallbackStateMachineFactory(candidateView, parentEpochState)
		},
		setups:               setups,
		commits:              commits,
		epochProtocolStateDB: epochProtocolStateDB,
		pendingDbUpdates:     transaction.NewDeferredBlockPersist(),
	}, nil
}

// Build schedules updates to the protocol state by obtaining the updated state from the active state machine,
// preparing deferred DB updates and committing updated sub-state ID to the KV store.
// ATTENTION: In mature implementation all parts of the Dynamic Protocol State will rely on the Key-Value Store as storage
// but to avoid a large refactoring we are using a hybrid approach where only the epoch state ID is stored in the KV Store
// but the actual epoch state is stored separately, nevertheless, the epoch state ID is used to sanity check if the
// epoch state is consistent with the KV Store. Using this approach, we commit the epoch sub-state to the KV Store which in
// affects the Dynamic Protocol State ID which is essentially hash of the KV Store.
func (e *EpochStateMachine) Build() (*transaction.DeferredBlockPersist, error) {
	updatedEpochState, updatedStateID, hasChanges := e.activeStateMachine.Build()
	e.pendingDbUpdates.AddIndexingOp(func(blockID flow.Identifier, tx *transaction.Tx) error {
		return e.epochProtocolStateDB.Index(blockID, updatedStateID)(tx)
	})
	if hasChanges {
		e.pendingDbUpdates.AddDbOp(operation.SkipDuplicatesTx(
			e.epochProtocolStateDB.StoreTx(updatedStateID, updatedEpochState.MinEpochStateEntry)))
	}
	e.mutator.SetEpochStateID(updatedStateID)

	return e.pendingDbUpdates, nil
}

// EvolveState applies the state change(s) on the Epoch sub-state based on information from the candidate block
// (under construction). Information that potentially changes the state (compared to the parent block's state):
//   - Service Events sealed in the candidate block
//   - the candidate block's view (already provided at construction time)
//
// CAUTION: EvolveState MUST be called for all candidate blocks, even if `sealedServiceEvents` is empty!
// This is because also the absence of expected service events by a certain view can also result in the
// Epoch state changing. (For example, not having received the EpochCommit event for the next epoch, but
// approaching the end of the current epoch.)
//
// The block's payload might contain epoch preparation service events for the next epoch. In this case,
// we need to update the tentative protocol state. We need to validate whether all information is available
// in the protocol state to go to the next epoch when needed. In cases where there is a bug in the smart
// contract, it could be that this happens too late, and we should trigger epoch fallback mode.
// No errors are expected during normal operations.
func (e *EpochStateMachine) EvolveState(sealedServiceEvents []flow.ServiceEvent) error {
	parentProtocolState := e.activeStateMachine.ParentState()

	// perform protocol state transition to next epoch if next epoch is committed, and we are at first block of epoch
	// TODO: The current implementation has edge cases for future light clients and can potentially drive consensus
	//       into an irreconcilable state (not sure). See for details https://github.com/onflow/flow-go/issues/5631
	//       These edge cases are very unlikely, so this is an acceptable implementation in the short - mid term.
	//       However, this code will likely need to be changed when working on EFM recovery.
	phase := parentProtocolState.EpochPhase()
	if phase == flow.EpochPhaseCommitted {
		if e.activeStateMachine.View() > parentProtocolState.CurrentEpochFinalView() {
			err := e.activeStateMachine.TransitionToNextEpoch()
			if err != nil {
				return fmt.Errorf("could not transition protocol state to next epoch: %w", err)
			}
		}
	}

	dbUpdates, err := e.applyServiceEventsFromOrderedResults(sealedServiceEvents)
	if err != nil {
		if protocol.IsInvalidServiceEventError(err) {
			dbUpdates, err = e.transitionToEpochFallbackMode(sealedServiceEvents)
			if err != nil {
				return irrecoverable.NewExceptionf("could not transition to epoch fallback mode: %w", err)
			}
		} else {
			return irrecoverable.NewExceptionf("could not apply service events from ordered results: %w", err)
		}
	}
	e.pendingDbUpdates.AddIndexingOps(dbUpdates.Pending())
	return nil
}

// View returns the view associated with this state machine.
// The view of the state machine equals the view of the block carrying the respective updates.
func (e *EpochStateMachine) View() uint64 {
	return e.activeStateMachine.View()
}

// ParentState returns parent state associated with this state machine.
func (e *EpochStateMachine) ParentState() protocol.KVStoreReader {
	return e.parentState
}

// applyServiceEventsFromOrderedResults applies the service events contained within the list of results
// to the pending state tracked by `stateMutator`.
// Each result corresponds to one seal that was included in the payload of the block being processed by this `stateMutator`.
// Results must be ordered by block height.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if any service event is invalid or is not a valid state transition for the current protocol state
func (e *EpochStateMachine) applyServiceEventsFromOrderedResults(orderedUpdates []flow.ServiceEvent) (*transaction.DeferredBlockPersist, error) {
	dbUpdates := transaction.NewDeferredBlockPersist()
	for _, event := range orderedUpdates {
		switch ev := event.Event.(type) {
		case *flow.EpochSetup:
			processed, err := e.activeStateMachine.ProcessEpochSetup(ev)
			if err != nil {
				return nil, fmt.Errorf("could not process epoch setup event: %w", err)
			}
			if processed {
				dbUpdates.AddDbOp(e.setups.StoreTx(ev)) // we'll insert the setup event when we insert the block
			}

		case *flow.EpochCommit:
			processed, err := e.activeStateMachine.ProcessEpochCommit(ev)
			if err != nil {
				return nil, fmt.Errorf("could not process epoch commit event: %w", err)
			}
			if processed {
				dbUpdates.AddDbOp(e.commits.StoreTx(ev)) // we'll insert the commit event when we insert the block
			}
		case *flow.EpochRecover:
			processed, err := e.activeStateMachine.ProcessEpochRecover(ev)
			if err != nil {
				return nil, fmt.Errorf("could not process epoch recover event: %w", err)
			}
			if processed {
				dbUpdates.AddDbOps(e.setups.StoreTx(&ev.EpochSetup), e.commits.StoreTx(&ev.EpochCommit)) // we'll insert the setup & commit events when we insert the block
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
func (e *EpochStateMachine) transitionToEpochFallbackMode(orderedUpdates []flow.ServiceEvent) (*transaction.DeferredBlockPersist, error) {
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
func epochFallbackTriggeredByIncorporatingCandidate(candidateView uint64, params protocol.GlobalParams, parentState *flow.RichEpochStateEntry) bool {
	if parentState.EpochPhase() == flow.EpochPhaseCommitted { // Requirement 1
		return false
	}
	return candidateView+params.EpochCommitSafetyThreshold() >= parentState.CurrentEpochSetup.FinalView // Requirement 2
}
