package sync

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type StateSynchronizer interface {
	PersistDelta(blockID flow.Identifier, set flow.RegisterDelta) error
	DeltaRange(
		startID, endID flow.Identifier,
		onDelta func(blockID flow.Identifier, delta flow.RegisterDelta) error,
	) error
}

func NewStateSynchronizer(
	state protocol.State,
	//registerDeltas storage.RegisterDeltas,
) StateSynchronizer {
	return &stateSync{
		state:          state,
		//registerDeltas: registerDeltas,
	}
}

type stateSync struct {
	state          protocol.State
	registerDeltas storage.RegisterDeltas
}

func (ss *stateSync) PersistDelta(blockID flow.Identifier, set flow.RegisterDelta) error {
	err := ss.registerDeltas.Store(blockID, &set)
	if err != nil {
		return fmt.Errorf("failed to persist block delta: %w", err)
	}

	return nil
}

func (ss *stateSync) DeltaRange(
	startID, endID flow.Identifier,
	onDelta func(blockID flow.Identifier, delta flow.RegisterDelta) error,
) error {
	blockID := startID

	// TODO: handle edge cases
	// - case 1: we have complete range (happy path)
	// - case 2: we are missing start of range
	// - case 3: we are missing end of range
	for blockID != flow.ZeroID {
		header, err := ss.state.AtBlockID(blockID).Head()
		if err != nil {
			return fmt.Errorf("failed to load block: %w", err)
		}

		delta, err := ss.getDelta(blockID)
		if err != nil {
			return err
		}

		err = onDelta(blockID, delta)
		if err != nil {
			return fmt.Errorf("failed to handle block delta: %w", err)
		}

		blockID = header.ParentID

		if header.ID() == endID {
			break
		}
	}

	return nil
}

func (ss *stateSync) getDelta(blockID flow.Identifier) (flow.RegisterDelta, error) {
	set, err := ss.registerDeltas.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block delta: %w", err)
	}

	return *set, nil
}
