package access

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

// ConvertIndexError converts the provided error and returns the appropriate access sentinel error.
func ConvertIndexError(dataType string, err error, height uint64) error {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return NewDataNotFoundError(dataType,
			fmt.Errorf("could not find transaction result for block: %w", err))
	case errors.Is(err, indexer.ErrIndexNotInitialized):
		return NewPreconditionFailedError(
			fmt.Errorf("%s index is not available: %w", dataType, err))
	case errors.Is(err, storage.ErrHeightNotIndexed):
		return NewOutOfRangeError(
			fmt.Errorf("%s for block %d are not available", dataType, height))
	default:
		return fmt.Errorf("failed to find transaction result: %w", err)
	}
}

// ConvertGrpcError converts the provided gRPC status.Error and access sentinel error.
func ConvertGrpcError(dataType string, err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return NewDataNotFoundError(dataType, err)
	case codes.Unavailable:
		return NewServiceUnavailable(err)
	case codes.Canceled:
		return NewRequestCanceledError(err)
	case codes.DeadlineExceeded:
		return NewRequestTimedOutError(err)
	case codes.InvalidArgument:
		return NewInvalidRequestError(err)
	case codes.OutOfRange:
		return NewOutOfRangeError(err)
	case codes.FailedPrecondition:
		return NewPreconditionFailedError(err)
	default:
		return NewInternalError(err)
	}
}
