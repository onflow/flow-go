package optimistic_sync

import (
	"errors"
	"fmt"

	"go.uber.org/atomic"
)

// State represents the state of the processing pipeline
type State int32

const (
	// StatePending is the initial state after instantiation, before Run is called
	StatePending State = iota
	// StateProcessing represents the state where data processing (download and indexing) has been started
	StateProcessing
	// StateWaitingPersist represents the state where all data is indexed, but conditions to persist are not met
	StateWaitingPersist
	// StateComplete represents the state where all data is persisted to storage
	StateComplete
	// StateAbandoned represents the state where processing was aborted
	StateAbandoned
)

// String representation of states for logging
func (s State) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateWaitingPersist:
		return "waiting_persist"
	case StateProcessing:
		return "processing"
	case StateComplete:
		return "complete"
	case StateAbandoned:
		return "abandoned"
	default:
		return ""
	}
}

// IsTerminal returns true if the state is a terminal state (Complete or Abandoned).
func (s State) IsTerminal() bool {
	return s == StateComplete || s == StateAbandoned
}

// ErrInvalidPipelineState is returned when the initial value provided to NewState2Tracker is
// not a valid starting state for the State2 state machine.
var ErrInvalidPipelineState = errors.New("invalid pipeline state")

// State2 represents the state of the processing pipeline with granular download/indexing phases.
//
// State Transitions (see Pipeline2 spec):
//
//	┏━━━━━━━━━┓    ┏━━━━━━━━━━━━━┓    ┏━━━━━━━━━━┓    ┏━━━━━━━━━━━━━━━━┓    ┏━━━━━━━━━━━━┓    ┏━━━━━━━━━━┓
//	┃ Pending ┃───►┃ Downloading ┃───►┃ Indexing ┃───►┃ WaitingPersist ┃───►┃ Persisting ┃───►┃ Complete ┃
//	┗━━━━┯━━━━┛    ┗━━━━━━┯━━━━━━┛    ┗━━━━━┯━━━━┛    ┗━━━━━━━┯━━━━━━━━┛    ┗━━━━━━━━━━━━┛    ┗━━━━━━━━━━┛
//	     │                │                 │                 │
//	     └────────────────┴─────────────────┴─────────────────┴────►┏━━━━━━━━━━━┓
//	                                                                ┃ Abandoned ┃
//	                                                                ┗━━━━━━━━━━━┛
//
// Properties of our state transition function:
//   - reflexive: transitioning to the same state is allowed (no-op) for *valid* states
type State2 uint32

const (
	// State2Pending is the initial state after instantiation, before processing begins.
	State2Pending State2 = iota
	// State2Downloading represents the state where ExecutionData download is in progress.
	State2Downloading
	// State2Indexing represents the state where building the index from downloaded data is in progress.
	State2Indexing
	// State2WaitingPersist represents the state where all data is indexed, but conditions to persist are not met.
	State2WaitingPersist
	// State2Persisting represents the state where the conditions to persist are met, but the persisting process hasn't finished.
	State2Persisting
	// State2Complete represents the terminal state where all data is persisted to storage.
	State2Complete
	// State2Abandoned represents the terminal state where processing was aborted.
	State2Abandoned
)

// String returns the string representation of State2.
func (s State2) String() string {
	switch s {
	case State2Pending:
		return "pending"
	case State2Downloading:
		return "downloading"
	case State2Indexing:
		return "indexing"
	case State2WaitingPersist:
		return "waiting2persist"
	case State2Persisting:
		return "persisting"
	case State2Complete:
		return "complete"
	case State2Abandoned:
		return "abandoned"
	default:
		return "unknown"
	}
}

// IsTerminal returns true if the state is a terminal state (Complete or Abandoned).
func (s State2) IsTerminal() bool {
	return s == State2Complete || s == State2Abandoned
}

// IsValid returns true if the state is a valid State2 value.
func (s State2) IsValid() bool {
	switch s {
	case State2Pending, State2Downloading, State2Indexing, State2WaitingPersist, State2Persisting, State2Complete, State2Abandoned:
		return true
	default:
		return false
	}
}

// CanReach returns true if and only if there exists a _sequence_ of valid state transitions (zero or more)
// that reaches the `to` state. Formally, this is the reflexive transitive closure of the state transition
// function [State2.IsValidTransition].
// ATTENTION: this function considers MULTI-STEP state transitions (zero or more).
// To query the single-step state transition function, use function [State2.IsValidTransition].
func (s State2) CanReach(to State2) bool {
	if to == s && to.IsValid() { // our state transition is defined to be reflexive: a valid state can always transition to itself
		return true
	}
	// State2Complete and State2Abandoned are terminal states, only allowing self-transitions, which are covered
	// above. Hence, in the following, we only need to handle `State2Pending` up to `State2Persisting`.

	switch s {
	case State2Pending:
		return to == State2Downloading || to == State2Indexing || to == State2WaitingPersist || to == State2Persisting || to == State2Complete || to == State2Abandoned
	case State2Downloading:
		return to == State2Indexing || to == State2WaitingPersist || to == State2Persisting || to == State2Complete || to == State2Abandoned
	case State2Indexing:
		return to == State2WaitingPersist || to == State2Persisting || to == State2Complete || to == State2Abandoned
	case State2WaitingPersist:
		return to == State2Persisting || to == State2Complete || to == State2Abandoned
	case State2Persisting:
		return to == State2Complete
	default:
		return false
	}
}

// IsValidTransition returns true if transitioning from this state to the given state is valid.
// Transitioning to the same state is considered valid (no-op), if and only of the state is valid.
// ATTENTION: this function represents exclusively SINGLE-STEP state transitions.
// For multi-step reachability, use function [State2.CanReach].
func (s State2) IsValidTransition(to State2) bool {
	if to == s && to.IsValid() { // our state transition is defined to be reflexive: a valid state can always transition to itself
		return true
	}
	// State2Complete and State2Abandoned are terminal states, only allowing self-transitions, which are covered
	// above. Hence, in the following, we only need to handle `State2Pending` up to `State2WaitingPersist`.

	switch s {
	case State2Pending:
		return to == State2Downloading || to == State2Abandoned
	case State2Downloading:
		return to == State2Indexing || to == State2Abandoned
	case State2Indexing:
		return to == State2WaitingPersist || to == State2Abandoned
	case State2WaitingPersist:
		return to == State2Persisting || to == State2Abandoned
	case State2Persisting:
		return to == State2Complete
	default:
		return false
	}
}

// isValidInitialState returns true if the state is a valid initial state for State2Tracker.
// We only allow starting from State2Pending since the pipeline
// always starts from the beginning.
func (s State2) isValidInitialState() bool {
	return s == State2Pending
}

// State2Tracker is a concurrency-safe tracker for State2 using atomic operations.
// It enforces valid state transitions as defined by [State2.IsValidTransition].
// Concurrent write-read establishes a 'happens before' relation as detailed in https://go.dev/ref/mem
type State2Tracker struct {
	// The constructor follows the prevalent pattern of returning a reference type.
	// We embed the atomic Uint32 directly to avoid extra pointer dereferences for performance reasons.
	state2 atomic.Uint32
}

// NewState2Tracker instantiates a concurrency-safe state machine with valid state transitions
// as specified in State2.IsValidTransition. The intended use is to track the processing state
// of an ExecutionResult in the Pipeline2.
//
// TODO: at the moment, the input is unnecessary, because only the initial state `State2Pending` is accepted
//
// Expected error returns during normal operations:
//   - [ErrInvalidPipelineState]: if the initial value is not a valid starting state
func NewState2Tracker(initialState State2) (*State2Tracker, error) {
	if !initialState.isValidInitialState() {
		return nil, fmt.Errorf("state '%s' is not a valid starting state: %w", initialState.String(), ErrInvalidPipelineState)
	}
	return &State2Tracker{
		state2: *(atomic.NewUint32(uint32(initialState))),
	}, nil
}

// Set attempts to transition the state to the new state.
// The transition succeeds if and only if it is valid as defined by [State2.IsValidTransition].
// No matter whether the state transition succeeds (second return value), the `oldState` return
// value is always the state from which the transition was attempted.
// ATTENTION: this function performs a SINGLE-STEP state transition.
// For multi-step state evolution, use function [State2.Evolve].
func (t *State2Tracker) Set(newState State2) (oldState State2, success bool) {
	for {
		oldState = t.Value()
		if !oldState.IsValidTransition(newState) {
			return oldState, false
		}
		if t.state2.CompareAndSwap(uint32(oldState), uint32(newState)) {
			return oldState, true
		}
	}
}

// Evolve attempts a _sequence_ of valid state transitions (zero or more) to reach the specified `target`
// from the current state. The entire sequence is performed as a single ATOMIC operation. The state
// evolution succeeds if and only if it is valid as defined by [State2.CanReach]. No matter whether the
// state evolution succeeds (second return value), the `oldState` return value is always the state from
// which the evolution was attempted.
// ATTENTION: this function performs MULTI-STEP state transitions (zero or more).
// To perform a single-step state transition, use function [State2.Set].
func (t *State2Tracker) Evolve(target State2) (oldState State2, success bool) {
	for {
		oldState = t.Value()
		if !oldState.CanReach(target) {
			return oldState, false
		}
		if t.state2.CompareAndSwap(uint32(oldState), uint32(target)) {
			return oldState, true
		}
	}
}

// CompareAndSwap attempts an ATOMIC transition from the anticipated old state to `newState`. If the current
// state is different from `anticipatedOldState`, the transition fails. For matching current state, the
// transition succeeds if and only if it is valid as defined by [State2.IsValidTransition]. No matter
// whether the state transition succeeds (second return value), the `oldState` return value is always
// the state from which the transition was attempted (not necessarily equal to input `anticipatedOldState`).
// ATTENTION: this function performs a SINGLE-STEP state transition. (Multi-step state evolution is straight
// forward, but not yet needed for the use-case; hence not implemented).
func (t *State2Tracker) CompareAndSwap(anticipatedOldState, newState State2) (oldState State2, success bool) {
	for {
		oldState = t.Value()
		if oldState != anticipatedOldState {
			return oldState, false
		}
		if !oldState.IsValidTransition(newState) {
			return oldState, false
		}
		if t.state2.CompareAndSwap(uint32(oldState), uint32(newState)) {
			return oldState, true
		}
	}
}

// Value returns the current state.
func (t *State2Tracker) Value() State2 {
	return State2(t.state2.Load())
}
