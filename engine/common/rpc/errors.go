package rpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-multierror"
	"github.com/onflow/flow-go/storage"
)

func ConvertStorageError(err error) error {
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

// ConvertMultiError converts a multierror to a grpc status error.
// If all of the errors in the multierror have the same code, that code is used, otherwise
// defaultCode is used.
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
