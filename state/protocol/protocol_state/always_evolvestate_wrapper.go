package protocol_state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// AlwaysEvolveStateWrapper wraps a `protocol_state.OrthogonalStoreStateMachine`. The wrapper guarantees that
// the state machine's `EvolveState(…)` is always called before its `Build()` method. If it hasn't been executed
// before, the wrapper will call the state machine's `EvolveState(…)` method first in its `Build()` step.
//
// NOT CONCURRENCY SAFE
type AlwaysEvolveStateWrapper[P any] struct {
	OrthogonalStoreStateMachine[P]
	evolveStateCalled bool
}

var _ OrthogonalStoreStateMachine[any] = (*AlwaysEvolveStateWrapper[any])(nil)

// NewAlwaysEvolveStateWrapper adds a wrapper to the given `stateMachine`, which guarantees that
// `stateMachine.EvolveState(…)` is always called before `stateMachine.Build()`. When the external logic
// calls `AlwaysEvolveStateWrapper.Build()`, the wrapper will call `stateMachine.EvolveState( empty )`
// first, if and only if it hasn't been called before. The input `empty` is an empty list of service events.
func NewAlwaysEvolveStateWrapper[P any](stateMachine OrthogonalStoreStateMachine[P]) OrthogonalStoreStateMachine[P] {
	return &AlwaysEvolveStateWrapper[P]{
		OrthogonalStoreStateMachine: stateMachine,
		evolveStateCalled:           false,
	}
}

// Build calls the state machine's `EvolveState(…)` method first with an empty list
// of Service Events, if and only if `EvolveState` hasn't been called before.
func (a *AlwaysEvolveStateWrapper[P]) Build() (*protocol.DeferredBlockPersist, error) {
	if !a.evolveStateCalled {
		err := a.OrthogonalStoreStateMachine.EvolveState([]flow.ServiceEvent{})
		if err != nil {
			return protocol.NewDeferredBlockPersist(), fmt.Errorf("attempting to execute EvolveState method with empty list of Service Events failed: %w", err)
		}
	}
	return a.OrthogonalStoreStateMachine.Build()
}

// EvolveState passes the call to the wrapped state machine. Thereafter, the wrapper will abstain
// from calling `EvolveState` during the `Build` step, irrespective of `EvolveState`'s return here.
func (a *AlwaysEvolveStateWrapper[P]) EvolveState(sealedServiceEvents []flow.ServiceEvent) error {
	a.evolveStateCalled = true
	return a.OrthogonalStoreStateMachine.EvolveState(sealedServiceEvents)
}
