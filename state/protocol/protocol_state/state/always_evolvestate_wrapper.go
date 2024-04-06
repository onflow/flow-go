package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	ps "github.com/onflow/flow-go/state/protocol/protocol_state"
)

var _ ps.OrthogonalStoreStateMachine[any] = (*AlwaysEvolveStateWrapper[any])(nil)

func NewAlwaysEvolveStateWrapper[P any](stateMachine ps.OrthogonalStoreStateMachine[P]) ps.OrthogonalStoreStateMachine[P] {
	return &AlwaysEvolveStateWrapper[P]{
		OrthogonalStoreStateMachine: stateMachine,
		evolveStateCalled:           false,
	}
}

type AlwaysEvolveStateWrapper[P any] struct {
	ps.OrthogonalStoreStateMachine[P]
	evolveStateCalled bool
}

func (a AlwaysEvolveStateWrapper[P]) Build() (protocol.DeferredBlockPersistOps, error) {
	if !a.evolveStateCalled {
		err := a.OrthogonalStoreStateMachine.EvolveState([]flow.ServiceEvent{})
		if err != nil {
			return ps.NewDeferredBlockPersist(), fmt.Errorf("attempting to execute EvolveState method with empty list of Service Events failed: %w", err)
		}
	}
	return a.OrthogonalStoreStateMachine.Build()
}

func (a AlwaysEvolveStateWrapper[P]) EvolveState(sealedServiceEvents []flow.ServiceEvent) error {
	a.evolveStateCalled = true
	return a.OrthogonalStoreStateMachine.EvolveState(sealedServiceEvents)
}
