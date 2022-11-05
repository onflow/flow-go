package state_stream

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

type API interface {
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*entities.BlockExecutionData, error)
}

type StateStreamBackend struct {
	headers       storage.Headers
	seals         storage.Seals
	results       storage.ExecutionResults
	execDataStore execution_data.ExecutionDataStore
}

func New(
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	execDataStore execution_data.ExecutionDataStore,
) *StateStreamBackend {
	return &StateStreamBackend{
		headers:       headers,
		seals:         seals,
		results:       results,
		execDataStore: execDataStore,
	}
}

func (s *StateStreamBackend) GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*entities.BlockExecutionData, error) {
	header, err := s.headers.ByBlockID(blockID)
	if err != nil {
		return nil, storage.ConvertStorageError(err)
	}

	seal, err := s.seals.FinalizedSealForBlock(header.ID())
	if err != nil {
		return nil, storage.ConvertStorageError(err)
	}

	result, err := s.results.ByID(seal.ResultID)
	if err != nil {
		return nil, storage.ConvertStorageError(err)
	}

	blockExecData, err := s.execDataStore.GetExecutionData(ctx, result.ExecutionDataID)
	if err != nil {
		return nil, err
	}

	message, err := convert.BlockExecutionDataToMessage(blockExecData)
	if err != nil {
		return nil, err
	}
	return message, nil
}
