package optimistic_sync

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allValidState2Values = []State2{
	State2Pending,        // 0
	State2Downloading,    // 1
	State2Indexing,       // 2
	State2WaitingPersist, // 3
	State2Complete,       // 4
	State2Abandoned,      // 5
}

// allState2Values includes all valid State2 values plus some invalid ones
// permitted by the underlying uint32 type.
var allState2Values = append(allValidState2Values,
	// invalid (but permitted by the underlying uint32 type):
	State2(State2Abandoned+1),
	State2(999),
)

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

// TestState2_IsValidTransition tests the [State2.IsValidTransition] method.
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
// * Our state transition function is reflexive, i.e. self-transitions from the state to itself are allowed (no-op) for valid states
// * Backward transitions are not allowed (e.g., Indexing -> Downloading).
// * Skipping states is not allowed (e.g., Pending -> Indexing).
// * Terminal states (Complete and Abandoned) cannot transition to any other state.
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

	t.Run("states transitions involving invalid states should always be considered invalid", func(t *testing.T) {
		for _, invalid := range []State2{State2Abandoned + 1, 999} {
			for _, s := range allState2Values {
				assert.False(t, s.IsValidTransition(invalid), "%s -> %s is not a valid transition", s, invalid)
				assert.False(t, invalid.IsValidTransition(s), "%s -> %s is not a valid transition", invalid, s)
			}
		}
	})
}

// TestState2_CanReach tests [State2.CanReach] method.
// For a state machine, "reachability" is defined as the reflexive transitive closure of the state
// transition function, whose correctness we have verified in the previous test [State2.IsValidTransition].
//
// Within this test, we also verify that:
// * Our state transition function is reflexive, i.e. self-transitions from the state to itself are allowed (no-op) for valid states
// * Backward transitions are not allowed (e.g., Indexing -> Downloading).
// * Terminal states (Complete and Abandoned) cannot transition to any other state.
// * Any pairs including invalid states are not reachable (including self-reachability).
func TestState2_CanReach(t *testing.T) {
	// We fix a `destination` state and ask: From which `start` states is this `destination` state reachable? For each destination
	// state, we store all possible start state we have found so far in the map `r: destination -> Set of known start states`.
	r := constructReachability()

	t.Run("exhaustively iterate over all pairs of states and check `CanReach` function", func(t *testing.T) {
		for _, start := range allState2Values {
			for _, dest := range allState2Values {
				if !start.IsValid() || !dest.IsValid() { // if either a or b are invalid states, reachability must be false
					assert.False(t, start.CanReach(dest), "starting from state '%s', state '%s' should not be reachable, as one or both states are invalid", start, dest)
				} else { // both `start` and `dest` are valid states
					_, expectedReachable := r[dest][start] // is `b` in the set of known start states from which `a` is reachable?
					assert.Equal(t, expectedReachable, start.CanReach(dest), "starting from state '%s', reachability of state '%s' is incorrect", start, dest)
				}
			}
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
		t.Fatalf("invalid target state %s", target)
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
	for _, target := range allValidState2Values {
		initializeAndTransitionTo_HappyPath(t, target)
	}
}

// TestState2Tracker_Set exhaustively checks all possible combinations of state transitions
// (valid and invalid) via the [State2Tracker.Set] method.
//
// Within these tests, we also verify that:
//   - Our state transition function is reflexive, i.e. self-transitions from the state to itself are allowed (no-op) for valid states.
//   - Backward transitions are not allowed (e.g., Indexing -> Downloading).
//   - Skipping states is not allowed (e.g., Pending -> Indexing).
//   - Terminal states (Complete and Abandoned) cannot transition to any other state.
//   - Transitioning to invalid states is not allowed. (we can't test transitions from invalid states, since the
//     State2Tracker can only be initialized in valid states and only allows valid state transitions).
func TestState2Tracker_Set(t *testing.T) {
	// verifyStateTransition is a helper to verify a single state transition via [State2Tracker.Set] method:
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
				assert.Equal(t, expectedOldState, newState, "Expected the state to remain at %s but got %s", expectedOldState, newState)
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

// TestState2Tracker_Evolve exhaustively checks all possible combinations of `start` and `destination` states
// (valid and invalid) and attempts to evolve the state via the [State2Tracker.Evolve] method.
//
// Within these tests, we also verify that:
//   - Our state transition function is reflexive, i.e. self-transitions from the state to itself are allowed (no-op) for valid states.
//   - Backward transitions are not allowed (e.g., Indexing -> Downloading).
//   - Skipping states is not allowed (e.g., Pending -> Indexing).
//   - Terminal states (Complete and Abandoned) cannot transition to any other state.
//   - Transitioning to invalid states is not allowed. (we can't test transitions from invalid states, since the
//     State2Tracker can only be initialized in valid states and only allows valid state transitions).
func TestState2Tracker_Evolve(t *testing.T) {
	// verifyStateEvolution is a helper to verify an atomic state evolution executed via [State2Tracker.Evolve] method.
	// It supports both expected success and expected failure cases. Process:
	// 1. A new tracker is initialized and transitioned to `currentTrackerState` along the happy path (utilizes functionality, whose
	//    correctness we have confirmed in prior tests). This generates the starting conditions for the `Evolve` operation we are testing.
	// 2. The `Evolve` operation is performed, requesting a state evolution to the `target` state.
	// 3. We compare the reported old state, success return as well as the resulting state against the
	//    reference implementation of "reachability" defined by the [State2.CanReach] method (whose correctness we have confirmed in
	//    prior tests)
	// Notes:
	// - `currentTrackerState` should be set to *valid* states only, since the State tracker should prevent reaching invalid states.
	verifyStateEvolution := func(t *testing.T, currentTrackerState Start, requestedEvolutionTarget Destination) {
		tracker := initializeAndTransitionTo_HappyPath(t, currentTrackerState) // only succeeds for valid states
		require.Equal(t, currentTrackerState, tracker.Value())                 // sanity check

		// evolve state via method [State2Tracker.Evolve]
		oldState, success := tracker.Evolve(requestedEvolutionTarget)
		newState := tracker.Value()
		assert.Equal(t, currentTrackerState, oldState, "Expected the old state to be reported as %s but got %s", currentTrackerState, oldState)
		expectedSuccess := currentTrackerState.CanReach(requestedEvolutionTarget)

		if expectedSuccess {
			t.Run(fmt.Sprintf("verifying evolution from '%s' to '%s' succeeds", currentTrackerState, requestedEvolutionTarget), func(t *testing.T) {
				assert.True(t, success, "Expected state evolution to be successful")
				assert.Equal(t, requestedEvolutionTarget, newState, "Expected the state to be evolved to '%s' but got '%s'", requestedEvolutionTarget, newState)
			})
		} else {
			t.Run(fmt.Sprintf("verifying evolution from '%s' to '%s' fails", currentTrackerState, requestedEvolutionTarget), func(t *testing.T) {
				assert.False(t, success, "Expected state transition to fail")
				// since state evolution failed, state should remain unchanged
				assert.Equal(t, currentTrackerState, newState, "Expected the state to remain at '%s' but got '%s'", currentTrackerState, newState)
			})
		}
	}

	for _, currentTrackerState := range allValidState2Values {
		t.Run(fmt.Sprintf("starting from tracker state '%s', exhaustively checking all target states", currentTrackerState), func(t *testing.T) {
			for _, requestedEvolutionTarget := range allState2Values {
				verifyStateEvolution(t, currentTrackerState, requestedEvolutionTarget)
			}
		})
	}
}

// TestState2Tracker_Set exhaustively checks all possible combinations of state transitions
// (valid and invalid) via the [State2Tracker.CompareAndSwap] method.
//
// Within these tests, we also verify that:
// * Our state transition function is reflexive, i.e. self-transitions from the state to itself are allowed (no-op) for valid states
// * Backward transitions are not allowed (e.g., Indexing -> Downloading).
// * Skipping states is not allowed (e.g., Pending -> Indexing).
// * Terminal states (Complete and Abandoned) cannot transition to any other state.
// * Transitioning to invalid states is not allowed.
func TestState2Tracker_CompareAndSwap(t *testing.T) {
	type casRequest struct{ from, to State2 } // syntactic sugar for better readability of the tests below

	// verifyStateTransition is a helper to verify a atomic state transition via [State2Tracker.CompareAndSwap] method.
	// It supports both expected success and expected failure cases. Process:
	// 1. A new tracker is initialized and transitioned to `currentTrackerState` along the happy path (utilizes functionality,
	//    whose correctness we have confirmed in prior tests). This generates the starting conditions for the CAS operation.
	// 2. The CAS operation is performed, requesting a state transition `casRequest.from` -> `casRequest.to`. Note that `casRequest.from`
	//    might be different from `currentTrackerState`. Thereby, we can test both valid and invalid requests for CAS operations.
	// 3. The criteria for success/failure of the CAS operation are:
	//     (i) the trackers `currentTrackerState` must be equal to the `casRequest.from` state requested by the CAS operation, and
	//    (ii) the transition `casRequest.from` -> `casRequest.to` must be valid according to [State2.IsValidTransition].
	//         In this test, we assume that [State2.IsValidTransition] is correct, which is verified in a prior test.
	// Notes:
	// - `currentTrackerState` should be set to *valid* states only, since the State tracker should prevent reaching invalid states.
	verifyStateTransition := func(t *testing.T, currentTrackerState State2, casRequest casRequest) {
		tracker := initializeAndTransitionTo_HappyPath(t, currentTrackerState) // only succeeds for valid starting states
		require.Equal(t, currentTrackerState, tracker.Value())                 // sanity check

		// perform CAS operation
		oldState, success := tracker.CompareAndSwap(casRequest.from, casRequest.to)
		newState := tracker.Value()
		assert.Equal(t, currentTrackerState, oldState, "Expected the old state to be reported as %s but got %s", casRequest.from, oldState)

		// check CAS result
		expectedSuccess := (currentTrackerState == casRequest.from) && casRequest.from.IsValidTransition(casRequest.to) // criteria (i) and (ii) determining expected success
		if expectedSuccess {
			t.Run(fmt.Sprintf("verifying that on state '%s', CAS '%s' -> '%s' succeeds", currentTrackerState, casRequest.from, casRequest.to), func(t *testing.T) {
				assert.True(t, success, "Expected state transition to be successful")
				assert.Equal(t, casRequest.to, newState, "Expected the state to be updated to '%s' but got '%s'", casRequest.to, newState)
			})
		} else {
			t.Run(fmt.Sprintf("verifying that on state '%s', CAS '%s' -> '%s' fails", currentTrackerState, casRequest.from, casRequest.to), func(t *testing.T) {
				assert.False(t, success, "Expected state transition to fail")
				// since state transition failed, state should remain unchanged
				assert.Equal(t, currentTrackerState, newState, "Expected the state to remain at '%s' but got '%s'", casRequest.from, newState)
			})
		}
	}

	for _, currentTrackerState := range allValidState2Values {
		t.Run(fmt.Sprintf("starting from tracker state '%s', exhaustively checking all pairs of CompareAndSwap requests", currentTrackerState), func(t *testing.T) {
			for _, casFrom := range allState2Values {
				for _, casTo := range allState2Values {
					verifyStateTransition(t, currentTrackerState, casRequest{from: casFrom, to: casTo})
				}
			}
		})
	}
}

/* ********************************* Test Helpers ********************************* */

type Destination = State2 // syntactic sugar for better readability
type Start = State2       // syntactic sugar for better readability

type ReachabilityReferenceImplementation map[Destination]map[Start]struct{}

// constructReachability evaluates expected reachability between pairs of states.
// While reachability in the production implementation is hard-coded for efficiency, this implementation here is for
// testing purposes only following an orthogonal: In a nutshell, we utilize that reachability is the reflexive transitive
// closure of the state transition function, whose correctness we have verified in the test [State2.IsValidTransition].
func constructReachability() ReachabilityReferenceImplementation {
	// We fix a `destination` state and ask: From which `start` states is this `destination` state reachable? For
	// each destination state, we store a tentative notion of all possible start state we have found so far in the
	// map `r: destination -> Set of known start states`.

	// Step 1: We begin with the start states that can reach the destination in one step (i.e. valid transitions).
	r := make(ReachabilityReferenceImplementation) // mapping destination -> Set of known start states from which `destination` is reachable
	for _, start := range allState2Values {
		for _, destination := range allState2Values {
			if start.IsValidTransition(destination) {
				starts := r[destination] // might return zero value, i.e. nil slice (works with all operations below)
				if starts == nil {
					starts = make(map[Start]struct{})
				}
				starts[start] = struct{}{}
				r[destination] = starts
			}
		}
	}

	// Step 2: we iteratively expand the set of known start states by adding all states that can reach any of the
	// already known start states in one step. This process is repeated until no new start states were found.
	for {
		foundNewStartingStates := false
		// We know that we have each possible destination state already in the map `r` from Step 1, because each state can at least
		// reach itself (reflexivity). Furthermore, for each destination state, the state itself is in the set of known start state.
		// Hence, we neither have to worry about missing destination states in `r`, nor about the set of known start states being nil.
		for _, starts := range r {
			for start := range starts { // iterates over all known start states, from which `destination` can be reached
				// Check _all_ possible states `s`, we check if the transition `s` -> `start` is valid. If
				// yes, then `s` can also reach `destination` (via start), so we add `s` to the set `r[destination]`.
				for _, s := range allState2Values {
					_, isKnownStartingState := starts[s]
					if isKnownStartingState || !s.IsValidTransition(start) { // if s is already a known starting state or s -> start is not a valid transition, skip
						continue
					}
					starts[s] = struct{}{}
					foundNewStartingStates = true
				}
			}
		}
		if !foundNewStartingStates {
			// no new start states were found: we have constructed the transitive closure of the state transition function (definition of reachability)
			return r
		}
	}
}
