package execution

import (
	"context"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

var _ state.ReadOnlyExecutionState = &ScriptExecutionState{}

type ScriptExecutionState struct{}

func (s ScriptExecutionState) NewStorageSnapshot(commitment flow.StateCommitment) snapshot.StorageSnapshot {
	return &storageSnapshot{}
}

func (s ScriptExecutionState) StateCommitmentByBlockID(
	ctx context.Context,
	identifier flow.Identifier,
) (flow.StateCommitment, error) {
	return flow.DummyStateCommitment, nil
}

func (s ScriptExecutionState) HasState(commitment flow.StateCommitment) bool {
	return false
}

func (s ScriptExecutionState) ChunkDataPackByChunkID(identifier flow.Identifier) (*flow.ChunkDataPack, error) {
	//TODO implement me
	panic("implement me")
}

func (s ScriptExecutionState) GetExecutionResultID(
	ctx context.Context,
	identifier flow.Identifier,
) (flow.Identifier, error) {
	//TODO implement me
	panic("implement me")
}

func (s ScriptExecutionState) GetHighestExecutedBlockID(ctx context.Context) (uint64, flow.Identifier, error) {
	//TODO implement me
	panic("implement me")
}

type storageSnapshot struct {
}

func (s *storageSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	return nil, nil
}
