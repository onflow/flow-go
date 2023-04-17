package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type storageState struct {
	baseStorage StorageSnapshot

	// The read set only include reads from the baseStorage
	readSet map[flow.RegisterID]struct{}

	writeSet map[flow.RegisterID]flow.RegisterValue
}

func newStorageState(base StorageSnapshot) *storageState {
	return &storageState{
		baseStorage: base,
		readSet:     map[flow.RegisterID]struct{}{},
		writeSet:    map[flow.RegisterID]flow.RegisterValue{},
	}
}

func (state *storageState) NewChild() *storageState {
	return newStorageState(NewPeekerStorageSnapshot(state))
}

func (state *storageState) Finalize() *ExecutionSnapshot {
	return &ExecutionSnapshot{
		ReadSet:  state.readSet,
		WriteSet: state.writeSet,
	}
}

func (state *storageState) Merge(snapshot *ExecutionSnapshot) error {
	for id := range snapshot.ReadSet {
		_, ok := state.writeSet[id]
		if ok {
			continue
		}
		state.readSet[id] = struct{}{}
	}

	for id, value := range snapshot.WriteSet {
		state.writeSet[id] = value
	}

	return nil
}

func (state *storageState) Set(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	state.writeSet[id] = value
	return nil
}

func (state *storageState) get(
	id flow.RegisterID,
) (
	bool, // read from base storage
	flow.RegisterValue,
	error,
) {
	value, ok := state.writeSet[id]
	if ok {
		return false, value, nil
	}

	if state.baseStorage == nil {
		return true, nil, nil
	}

	value, err := state.baseStorage.Get(id)
	if err != nil {
		return true, nil, fmt.Errorf("get register failed: %w", err)
	}

	return true, value, nil
}

func (state *storageState) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	readFromBaseStorage, value, err := state.get(id)
	if err != nil {
		return nil, err
	}

	if readFromBaseStorage {
		state.readSet[id] = struct{}{}
	}

	return value, nil
}

func (state *storageState) Peek(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	_, value, err := state.get(id)
	return value, err
}

func (state *storageState) DropChanges() error {
	state.writeSet = map[flow.RegisterID]flow.RegisterValue{}
	return nil
}
