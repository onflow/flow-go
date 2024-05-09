package rpc

import (
	"context"
	"errors"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
)

// ConvertError converts a generic error into a grpc status error. The input may either
// be a status.Error already, or standard error type. Any error that matches on of the
// common status code mappings will be converted, all unmatched errors will be converted
// to the provided defaultCode.
func ConvertError(err error, msg string, defaultCode codes.Code) error {
	if err == nil {
		return nil
	}

	// Handle multierrors separately
	if multiErr, ok := err.(*multierror.Error); ok {
		return ConvertMultiError(multiErr, msg, defaultCode)
	}

	// Already converted
	if status.Code(err) != codes.Unknown {
		return err
	}

	if msg != "" {
		msg += ": "
	}

	var returnCode codes.Code
	switch {
	case errors.Is(err, context.Canceled):
		returnCode = codes.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		returnCode = codes.DeadlineExceeded
	default:
		returnCode = defaultCode
	}

	return status.Errorf(returnCode, "%s%v", msg, err)
}

// ConvertStorageError converts a generic error into a grpc status error, converting storage errors
// into codes.NotFound
func ConvertStorageError(err error) error {
	if err == nil {
		return nil
	}

	// Already converted
	if status.Code(err) == codes.NotFound {
		return err
	}

	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return status.Errorf(codes.Internal, "failed to find: %v", err)
}

// ConvertIndexError converts errors related to index and storage to appropriate gRPC status errors.
// If the error is nil, it returns nil. If the error is not recognized, it falls back to ConvertError
// with the provided default message and Internal gRPC code.
func ConvertIndexError(err error, height uint64, defaultMsg string) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, indexer.ErrIndexNotInitialized) {
		return status.Errorf(codes.FailedPrecondition, "data for block is not available: %v", err)
	}

	if errors.Is(err, storage.ErrHeightNotIndexed) {
		return status.Errorf(codes.OutOfRange, "data for block height %d is not available", height)
	}

	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "data not found: %v", err)
	}

	return ConvertError(err, defaultMsg, codes.Internal)
}

// ConvertMultiError converts a multierror to a grpc status error.
// If the errors have related status codes, the common code is returned, otherwise defaultCode is used.
func ConvertMultiError(err *multierror.Error, msg string, defaultCode codes.Code) error {
	allErrors := err.WrappedErrors()
	if len(allErrors) == 0 {
		return nil
	}

	if msg != "" {
		msg += ": "
	}

	// get a list of all of status codes
	allCodes := make(map[codes.Code]struct{})
	for _, err := range allErrors {
		allCodes[status.Code(err)] = struct{}{}
	}

	// if they all match, return that
	if len(allCodes) == 1 {
		code := status.Code(allErrors[0])
		return status.Errorf(code, "%s%v", msg, err)
	}

	// if they mostly match, ignore Unavailable and DeadlineExceeded since any other code is
	// more descriptive
	if len(allCodes) == 2 {
		if _, ok := allCodes[codes.Unavailable]; ok {
			delete(allCodes, codes.Unavailable)
			for code := range allCodes {
				return status.Errorf(code, "%s%v", msg, err)
			}
		}
		if _, ok := allCodes[codes.DeadlineExceeded]; ok {
			delete(allCodes, codes.DeadlineExceeded)
			for code := range allCodes {
				return status.Errorf(code, "%s%v", msg, err)
			}
		}
	}

	// otherwise, return the default code
	return status.Errorf(defaultCode, "%s%v", msg, err)
}
