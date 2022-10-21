package state_stream

import (
	"context"
	"errors"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

type API interface {
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*entities.BlockExecutionData, error)
}

type StateStreamBackend struct {
	headers        storage.Headers
	seals          storage.Seals
	results        storage.ExecutionResults
	execDownloader execution_data.Downloader
}

func New(
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	execDownloader execution_data.Downloader,
) *StateStreamBackend {
	return &StateStreamBackend{
		headers:        headers,
		seals:          seals,
		results:        results,
		execDownloader: execDownloader,
	}
}

func (s *StateStreamBackend) GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*entities.BlockExecutionData, error) {
	header, err := s.headers.ByBlockID(blockID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	seal, err := s.seals.FinalizedSealForBlock(header.ID())
	if err != nil {
		return nil, convertStorageError(err)
	}

	result, err := s.results.ByID(seal.ResultID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	blockExecData, err := s.execDownloader.Download(ctx, result.ExecutionDataID)
	if err != nil {
		return nil, err
	}

	message, err := convert.BlockExecutionDataToMessage(blockExecData)
	if err != nil {
		return nil, err
	}
	return message, nil
}

func convertStorageError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) == codes.NotFound {
		// Already converted
		return err
	}
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return status.Errorf(codes.Internal, "failed to find: %v", err)
}
