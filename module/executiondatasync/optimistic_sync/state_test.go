package optimistic_sync

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allState2Values includes all valid State2 values plus some invalid ones
// permitted by the underlying uint32 type.
var allState2Values = []State2{
	// valid states
	State2Pending,        // 0
	State2Downloading,    // 1
	State2Indexing,       // 2
	State2WaitingPersist, // 3
	State2Complete,       // 4
	State2Abandoned,      // 5

	// invalid (but permitted by the underlying uint32 type)
	State2(State2Abandoned + 1),
	State2(999),
}

// TestState2_IsValid tests the IsValid method for State2
func TestState2_IsValid(t *testing.T) {
	// Valid states
	assert.True(t, State2Pending.IsValid())
	assert.True(t, State2Downloading.IsValid())
	assert.True(t, State2Indexing.IsValid())
	assert.True(t, State2WaitingPersist.IsValid())
	assert.True(t, State2Complete.IsValid())
	assert.True(t, State2Abandoned.IsValid())

	// invalid states
	assert.False(t, State2(State2Abandoned+1).IsValid())
	assert.False(t, State2(999).IsValid())
}

// TestState2_String tests the String method for State2
func TestState2_String(t *testing.T) {
	assert.Equal(t, "pending", State2Pending.String())
	assert.Equal(t, "downloading", State2Downloading.String())
	assert.Equal(t, "indexing", State2Indexing.String())
	assert.Equal(t, "waiting2persist", State2WaitingPersist.String())
	assert.Equal(t, "complete", State2Complete.String())
	assert.Equal(t, "abandoned", State2Abandoned.String())

	assert.Equal(t, "unknown", State2(State2Abandoned+1).String())
	assert.Equal(t, "unknown", State2(999).String())
}

// TestState2_IsTerminal tests the IsTerminal method for State2
func TestState2_IsTerminal(t *testing.T) {
	// Valid states that are not terminal states
	assert.False(t, State2Pending.IsTerminal())
	assert.False(t, State2Downloading.IsTerminal())
	assert.False(t, State2Indexing.IsTerminal())
	assert.False(t, State2WaitingPersist.IsTerminal())

	// Valid terminal states
	assert.True(t, State2Complete.IsTerminal())
	assert.True(t, State2Abandoned.IsTerminal())

	// Also invalid states should not be considered terminal
	assert.False(t, State2(State2Abandoned+1).IsTerminal())
	assert.False(t, State2(999).IsTerminal())
}

// TestState2_IsValidTransition tests the IsValidTransition method.
// Valid transitions are defined as follows:
//
//	┏━━━━━━━━━━━┓    ┏━━━━━━━━━━━━━━┓    ┏━━━━━━━━━━━┓    ┏━━━━━━━━━━━━━━━━┓    ┏━━━━━━━━━━━┓
//	┃  Pending  ┃───►┃ Downloading  ┃───►┃ Indexing  ┃───►┃ WaitingPersist ┃───►┃ Complete  ┃
//	┗━━━━━┯━━━━━┛    ┗━━━━━━┯━━━━━━━┛    ┗━━━━━┯━━━━━┛    ┗━━━━━━━┯━━━━━━━━┛    ┗━━━━━━━━━━━┛
//	      │                 │                  │                  │       ┏━━━━━━━━━━━┓
//	      └─────────────────┴──────────────────┴──────────────────┴──────►┃ Abandoned ┃
//	                                                                      ┗━━━━━━━━━━━┛
//
// Within this test, we also verify that:
// * out state transition function is reflexive, i.e. self-transitions from the state to itself are allowed (no-op)
// * backward transitions are not allowed (e.g., Indexing -> Downloading).
// * skipping states is not allowed (e.g., Pending -> Indexing).
// * terminal states (Complete and Abandoned) cannot transition to any other state.
func TestState2_IsValidTransition(t *testing.T) {

	t.Run("state transitions from `State2Pending`", func(t *testing.T) {
		from := State2Pending
		for _, toValid := range []State2{State2Pending, State2Downloading, State2Abandoned} {
			assert.True(t, from.IsValidTransition(toValid))
		}
		for _, toInvalid := range []State2{State2Indexing, State2WaitingPersist, State2Complete, State2Abandoned + 1, 999} {
			assert.False(t, from.IsValidTransition(toInvalid))
		}
	})

	t.Run("state transitions from `State2Downloading`", func(t *testing.T) {
		from := State2Downloading
		for _, toValid := range []State2{State2Downloading, State2Indexing, State2Abandoned} {
			assert.True(t, from.IsValidTransition(toValid))
		}
		for _, toInvalid := range []State2{State2Pending, State2WaitingPersist, State2Complete, State2Abandoned + 1, 999} {
			assert.False(t, from.IsValidTransition(toInvalid))
		}
	})

	t.Run("state transitions from `State2Indexing`", func(t *testing.T) {
		from := State2Indexing
		for _, toValid := range []State2{State2Indexing, State2WaitingPersist, State2Abandoned} {
			assert.True(t, from.IsValidTransition(toValid))
		}
		for _, toInvalid := range []State2{State2Pending, State2Downloading, State2Complete, State2Abandoned + 1, 999} {
			assert.False(t, from.IsValidTransition(toInvalid))
		}
	})

	t.Run("state transitions from `State2WaitingPersist`", func(t *testing.T) {
		from := State2WaitingPersist
		for _, toValid := range []State2{State2Complete, State2Abandoned} {
			assert.True(t, from.IsValidTransition(toValid))
		}
		for _, toInvalid := range []State2{State2Pending, State2Downloading, State2Indexing, State2Abandoned + 1, 999} {
			assert.False(t, from.IsValidTransition(toInvalid))
		}
	})

	t.Run("states `Complete` and `Abandoned` are terminal", func(t *testing.T) {
		// terminal states only allow self-transitions
		for _, toInvalid := range allState2Values {
			assert.Equal(t, State2Complete == toInvalid, State2Complete.IsValidTransition(toInvalid), "%s -> %s is not a valid transition", State2Complete, toInvalid)
			assert.Equal(t, State2Abandoned == toInvalid, State2Abandoned.IsValidTransition(toInvalid), "%s -> %s is not a valid transition", State2Abandoned, toInvalid)
		}
	})
}

// TestState2_IsValid_isValidInitialState verifies that the internal method
// `isValidInitialState` only considers State2Pending as valid initial state.
func TestState2_IsValid_isValidInitialState(t *testing.T) {
	for _, initState := range allState2Values {
		assert.Equal(t, initState == State2Pending, initState.isValidInitialState(), "%s should not be considered a valid initial state", initState)
	}
}

// TestNewState2Tracker_RejectsInvalidStartingStates verifies that the State Tracker
// can only be initialized with State2Pending.
func TestState2Tracker_RejectsInvalidStartingStates(t *testing.T) {
	t.Run("Happy path: State Tracker initialized in state pending", func(t *testing.T) {
		tracker, err := NewState2Tracker(State2Pending)
		require.NoError(t, err)
		require.NotNil(t, tracker)
		assert.Equal(t, State2Pending, tracker.Value())
	})

	t.Run("Invalid states for initializing State Tracker", func(t *testing.T) {
		for _, initState := range allState2Values {
			if initState == State2Pending { // single state that should be permitted as starting state
				continue
			}
			tracker, err := NewState2Tracker(initState)
			assert.ErrorIs(t, err, ErrInvalidPipelineState)
			assert.Nil(t, tracker)
		}
	})
}

// InitializeAndTransitionTo_HappyPath is a utility method. It initializes a State2Tracker
// and transitions it to the specified target state along the HAPPY PATH
func initializeAndTransitionTo_HappyPath(t *testing.T, target State2) *State2Tracker {
	// recursively initialize and forward
	var prior State2
	switch target {
	case State2Pending: // base case
		tracker, err := NewState2Tracker(State2Pending)
		require.NoError(t, err)
		assert.Equal(t, State2Pending, tracker.Value())
		return tracker
	case State2Downloading: // Pending -> Downloading
		prior = State2Pending
	case State2Indexing: // Downloading -> Indexing
		prior = State2Downloading
	case State2WaitingPersist: // Indexing -> WaitingPersist
		prior = State2Indexing
	case State2Complete: // WaitingPersist -> Complete
		prior = State2WaitingPersist
	case State2Abandoned: // can be reached from any state except `Complete`; we choose the shortest route
		prior = State2Pending
	default:
		t.Fatalf("invalid target state state %s", target)
	}

	tracker := initializeAndTransitionTo_HappyPath(t, prior)
	assert.Equal(t, prior, tracker.Value())
	oldState, success := tracker.Set(target)
	require.True(t, success, "unexpected failure of state transition %s to %s", prior, target)
	require.Equal(t, prior, oldState, "unexpected behaviour: prior state should be %s but got %s", prior, oldState)
	newState := tracker.Value()
	assert.Equal(t, target, newState, "unexpected behaviour: tracker should be in state %s but got %s", target, newState)
	return tracker
}

// TestState2Tracker_HappyPath tests the complete happy path through all states
func TestState2Tracker_HappyPath(t *testing.T) {
	for _, target := range []State2{State2Pending, State2Downloading, State2Indexing, State2WaitingPersist, State2Complete, State2Abandoned} {
		initializeAndTransitionTo_HappyPath(t, target)
	}
}

// TestState2Tracker_Set exhaustively checks all possible combinations of state transitions
// (valid and invalid) via the [State2Tracker.Set] method.
//
// Within these tests, we also verify that:
// * out state transition function is reflexive, i.e. self-transitions from the state to itself are allowed (no-op)
// * backward transitions are not allowed (e.g., Indexing -> Downloading).
// * skipping states is not allowed (e.g., Pending -> Indexing).
// * terminal states (Complete and Abandoned) cannot transition to any other state.
func TestState2Tracker_Set(t *testing.T) {
	//  verifyStateTransition is a helper to verify a single state transition via [State2Tracker.Set] method:
	// it supports both expected success and expected failure cases.
	verifyStateTransition := func(t *testing.T, tracker *State2Tracker, expectedOldState State2, to State2, expectedSuccess bool) {
		oldState, success := tracker.Set(to)
		newState := tracker.Value()
		assert.Equal(t, expectedOldState, oldState, "Expected the old state to be reported as %s but got %s", expectedOldState, oldState)

		if expectedSuccess {
			t.Run(fmt.Sprintf("verifying %s -> %s succeeds", expectedOldState, to), func(t *testing.T) {
				assert.True(t, success, "Expected state transition to be successful")
				assert.Equal(t, to, newState, "Expected the state to be updated to %s but got %s", to, newState)
			})
		} else {
			t.Run(fmt.Sprintf("verifying %s -> %s fails", expectedOldState, to), func(t *testing.T) {
				assert.False(t, success, "Expected state transition to fail")
				// since state transition failed, state should remain unchanged
				assert.Equal(t, expectedOldState, newState, "Expected the state to be updated to %s but got %s", expectedOldState, newState)
			})
		}
	}

	t.Run("transitions from Pending", func(t *testing.T) {
		for _, to := range allState2Values {
			tracker := initializeAndTransitionTo_HappyPath(t, State2Pending) // correctness verified in prior test `TestState2Tracker_HappyPath`
			expectedValidity := State2Pending.IsValidTransition(to)          // assumed to be correct, as tested above
			verifyStateTransition(t, tracker, State2Pending, to, expectedValidity)
		}
	})

	// We can only start from Pending, so we need to manually transition to other states
	// to test transitions from those states.

	t.Run("transitions from Downloading", func(t *testing.T) {
		for _, to := range allState2Values {
			tracker := initializeAndTransitionTo_HappyPath(t, State2Downloading) // correctness verified in prior test `TestState2Tracker_HappyPath`
			expectedValidity := State2Downloading.IsValidTransition(to)          // assumed to be correct, as tested above
			verifyStateTransition(t, tracker, State2Downloading, to, expectedValidity)
		}
	})

	t.Run("transitions from Indexing", func(t *testing.T) {
		for _, to := range allState2Values {
			tracker := initializeAndTransitionTo_HappyPath(t, State2Indexing) // correctness verified in prior test `TestState2Tracker_HappyPath`
			expectedValidity := State2Indexing.IsValidTransition(to)          // assumed to be correct, as tested above
			verifyStateTransition(t, tracker, State2Indexing, to, expectedValidity)
		}
	})

	t.Run("transitions from WaitingPersist", func(t *testing.T) {
		for _, to := range allState2Values {
			tracker := initializeAndTransitionTo_HappyPath(t, State2WaitingPersist) // correctness verified in prior test `TestState2Tracker_HappyPath`
			expectedValidity := State2WaitingPersist.IsValidTransition(to)          // assumed to be correct, as tested above
			verifyStateTransition(t, tracker, State2WaitingPersist, to, expectedValidity)
		}
	})

	t.Run("transitions from Complete (terminal)", func(t *testing.T) {
		for _, to := range allState2Values {
			tracker := initializeAndTransitionTo_HappyPath(t, State2Complete) // correctness verified in prior test `TestState2Tracker_HappyPath`
			expectedValidity := to == State2Complete                          // Complete is terminal, so all transitions except self-transition should fail
			verifyStateTransition(t, tracker, State2Complete, to, expectedValidity)
		}
	})

	t.Run("transitions from Abandoned (terminal)", func(t *testing.T) {
		for _, to := range allState2Values {
			tracker := initializeAndTransitionTo_HappyPath(t, State2Abandoned) // correctness verified in prior test `TestState2Tracker_HappyPath`
			expectedValidity := to == State2Abandoned                          // Abandoned is terminal, so all transitions except self-transition should fail
			verifyStateTransition(t, tracker, State2Abandoned, to, expectedValidity)
		}
	})
}

// TestState2Tracker_Set exhaustively checks all possible combinations of state transitions
// (valid and invalid) via the [State2Tracker.CompareAndSwap] method.
//
// Within these tests, we also verify that:
// * out state transition function is reflexive, i.e. self-transitions from the state to itself are allowed (no-op)
// * backward transitions are not allowed (e.g., Indexing -> Downloading).
// * skipping states is not allowed (e.g., Pending -> Indexing).
// * terminal states (Complete and Abandoned) cannot transition to any other state.
func TestState2Tracker_CompareAndSwap(t *testing.T) {
	type casRequest struct{ from, to State2 } // syntactic sugar for better readability of the tests below

	// verifyStateTransition is a helper to verify a single state transition via [State2Tracker.CompareAndSwap] method:
	// it supports both expected success and expected failure cases. Process:
	// 1. A new tracker is initialized and transitioned to `currentTrackerState` along the happy path. This generates the
	//    starting conditions for the CAS operation.
	// 2. The CAS operation is performed, requesting a state transition `casRequest.from` -> `casRequest.to`. Note that `casRequest.from`
	//    might be different from `currentTrackerState`. Thereby, we can test both valid and invalid requests for CAS operations.
	// 3. The criteria for success/failure of the CAS operation are:
	//     (i) the trackers `currentTrackerState` must be equal to the `casRequest.from` state requested by the CAS operation, and
	//    (ii) the transition `casRequest.from` -> `casRequest.to` must be valid according to [State2.IsValidTransition].
	//         In this test, we assume that [State2.IsValidTransition] is correct, which is verified in a prior test.
	// Notes:
	// - `currentTrackerState` should be set to *valid* states only, since the State tracker should prevent reaching invalid states.
	verifyStateTransition := func(t *testing.T, currentTrackerState State2, casRequest casRequest) {
		tracker := initializeAndTransitionTo_HappyPath(t, currentTrackerState) // only succeeds for valid states
		require.Equal(t, currentTrackerState, tracker.Value())                 // sanity check

		// perform CAS operation
		oldState, success := tracker.CompareAndSwap(casRequest.from, casRequest.to)
		newState := tracker.Value()
		assert.Equal(t, currentTrackerState, oldState, "Expected the old state to be reported as %s but got %s", casRequest.from, oldState)

		// check CAS result
		expectedSuccess := (currentTrackerState == casRequest.from) && casRequest.from.IsValidTransition(casRequest.to) // criteria (i) and (ii) determining expected success
		if expectedSuccess {
			t.Run(fmt.Sprintf("verifying that on state %s, CAS %s -> %s succeeds", currentTrackerState, casRequest.from, casRequest.to), func(t *testing.T) {
				assert.True(t, success, "Expected state transition to be successful")
				assert.Equal(t, casRequest.to, newState, "Expected the state to be updated to %s but got %s", casRequest.to, newState)
			})
		} else {
			t.Run(fmt.Sprintf("verifying that on state %s, CAS %s -> %s fails", currentTrackerState, casRequest.from, casRequest.to), func(t *testing.T) {
				assert.False(t, success, "Expected state transition to fail")
				// since state transition failed, state should remain unchanged
				assert.Equal(t, currentTrackerState, newState, "Expected the state to be updated to %s but got %s", casRequest.from, newState)
			})
		}
	}

	for _, currentTrackerState := range []State2{State2Pending, State2Downloading, State2Indexing, State2WaitingPersist, State2Complete, State2Abandoned} {
		t.Run(fmt.Sprintf("starting from tracker state %s, exhaustively checking all pairs of CompareAndSwap requests", currentTrackerState), func(t *testing.T) {
			for _, casFrom := range allState2Values {
				for _, casTo := range allState2Values {
					verifyStateTransition(t, currentTrackerState, casRequest{from: casFrom, to: casTo})
				}
			}
		})
	}
}
