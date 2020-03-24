package sync

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

type StateSynchronizer interface {
	//PersistDelta(blockID flow.Identifier, set flow.RegisterDelta) error
	DeltaRange(
		startID, endID flow.Identifier,
		onDelta func(blockID flow.Identifier, delta *messages.ExecutionStateDelta) error,
	) error
}

func NewStateSynchronizer(
	state state.ReadOnlyExecutionState,
//registerDeltas storage.RegisterDeltas,
) StateSynchronizer {
	return &stateSync{
		execState: state,
		//registerDeltas: registerDeltas,
	}
}

type stateSync struct {
	execState state.ReadOnlyExecutionState
	//registerDeltas storage.RegisterDeltas
}

//func (ss *stateSync) PersistDelta(blockID flow.Identifier, set flow.RegisterDelta) error {
//	err := ss.registerDeltas.Store(blockID, &set)
//	if err != nil {
//		return fmt.Errorf("failed to persist block delta: %w", err)
//	}
//
//	return nil
//}

func (ss *stateSync) DeltaRange(
	startID, endID flow.Identifier,
	onDelta func(blockID flow.Identifier, delta *messages.ExecutionStateDelta) error,
) error {
	blockID := startID

	// TODO: handle edge cases
	// - case 1: we have complete range (happy path)
	// - case 2: we are missing start of range
	// - case 3: we are missing end of range
	for blockID != flow.ZeroID {

		delta, err := ss.execState.RetrieveStateDelta(blockID)

		if err != nil {
			return fmt.Errorf("failed to retrieve state delta: %w", err)
		}

		//delta, err := ss.getDelta(header)
		//if err != nil {
		//	return err
		//}

		err = onDelta(blockID, delta)
		if err != nil {
			return fmt.Errorf("failed to handle block delta: %w", err)
		}

		if delta.Block.ID() == endID {
			break
		}

		blockID = delta.Block.ParentID
	}

	return nil
}

//func (ss *stateSync) getDelta(header *flow.Header) (*messages.ExecutionStateDelta, error) {
//
//	set, err := ss.registerDeltas.ByBlockID(blockID)
//	if err != nil {
//		return nil, fmt.Errorf("failed to retrieve block delta: %w", err)
//	}
//
//	return *set, nil
//}
