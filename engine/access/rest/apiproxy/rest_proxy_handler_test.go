package apiproxy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

func TestConvertError_Success(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		expected error
	}{
		{
			name:     "NotFound with data not found prefix",
			typeName: "collection",
			expected: access.NewDataNotFoundError("collection", storage.ErrNotFound),
		},
		{
			name:     "Internal with internal error prefix",
			typeName: "execution result",
			expected: access.NewInternalError(errors.New("database connection failed")),
		},
		{
			name:     "OutOfRange with out of range prefix",
			typeName: "block",
			expected: access.NewOutOfRangeError(errors.New("block height 1000 not available")),
		},
		{
			name:     "FailedPrecondition with precondition failed prefix",
			typeName: "events",
			expected: access.NewPreconditionFailedError(errors.New("index not initialized")),
		},
		{
			name:     "InvalidArgument with invalid argument prefix",
			typeName: "transaction",
			expected: access.NewInvalidRequestError(errors.New("malformed transaction")),
		},
		{
			name:     "Canceled with request canceled prefix",
			typeName: "script",
			expected: access.NewRequestCanceledError(errors.New("client disconnected")),
		},
		{
			name:     "Canceled without prefix (client side)",
			typeName: "account",
			expected: access.NewRequestCanceledError(status.Error(codes.Canceled, "context canceled")),
		},
		{
			name:     "DeadlineExceeded with request timed out prefix",
			typeName: "script",
			expected: access.NewRequestTimedOutError(errors.New("execution took too long")),
		},
		{
			name:     "DeadlineExceeded without prefix (client side)",
			typeName: "account",
			expected: access.NewRequestTimedOutError(status.Error(codes.DeadlineExceeded, "context deadline exceeded")),
		},
		{
			name:     "Unavailable with service unavailable error prefix",
			typeName: "execution result",
			expected: access.NewServiceUnavailable(errors.New("upstream service down")),
		},
		{
			name:     "Unavailable without prefix (client side)",
			typeName: "collection",
			expected: access.NewServiceUnavailable(status.Error(codes.Unavailable, "connection refused")),
		},
		{
			name:     "ResourceExhausted with service resource exhausted error prefix",
			typeName: "",
			expected: access.NewResourceExhausted(errors.New("execution computation limit reached")),
		},
		{
			name:     "ResourceExhausted without prefix (client side)",
			typeName: "",
			expected: access.NewResourceExhausted(status.Error(codes.ResourceExhausted, "execution computation limit reached")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := irrecoverable.NewMockSignalerContext(t, context.Background())

			grpcError := rpc.ErrorToStatus(tt.expected)

			actual := convertError(ctx, grpcError, tt.typeName)
			require.Error(t, actual, "convertError should return an access sentinel error")

			assert.Equal(t, tt.expected.Error(), actual.Error())

			switch tt.expected.(type) {
			case access.DataNotFoundError:
				assert.True(t, access.IsDataNotFoundError(actual))
			case access.InternalError:
				assert.True(t, access.IsInternalError(actual))
			case access.OutOfRangeError:
				assert.True(t, access.IsOutOfRangeError(actual))
			case access.PreconditionFailedError:
				assert.True(t, access.IsPreconditionFailedError(actual))
			case access.InvalidRequestError:
				assert.True(t, access.IsInvalidRequestError(actual))
			case access.RequestCanceledError:
				assert.True(t, access.IsRequestCanceledError(actual))
			case access.RequestTimedOutError:
				assert.True(t, access.IsRequestTimedOutError(actual))
			case access.ServiceUnavailable:
				assert.True(t, access.IsServiceUnavailable(actual))
			case access.ResourceExhausted:
				assert.True(t, access.IsResourceExhausted(actual))
			default:
				t.Fatalf("unexpected error type: %T", tt.expected)
			}
		})
	}
}

func TestConvertError_Irrecoverable(t *testing.T) {
	// This test verifies that convertError triggers irrecoverable error handling when it cannot convert the error.
	// We test both unknown status codes and incorrectly formatted error messages for known status codes.
	// Note: codes.Canceled, codes.DeadlineExceeded, and codes.Unavailable have fallback logic that wraps
	// the original error directly, so they don't trigger irrecoverable errors.

	tests := []struct {
		name     string
		grpcErr  error
		typeName string
	}{
		{
			name:     "generic error",
			grpcErr:  fmt.Errorf("generic error"),
			typeName: "account",
		},
		{
			name:     "unknown status code",
			grpcErr:  status.Error(codes.PermissionDenied, "permission denied"),
			typeName: "account",
		},
		{
			name:     "not found with incorrect prefix",
			grpcErr:  status.Error(codes.NotFound, "wrong prefix: key not found"),
			typeName: "collection",
		},
		{
			name:     "internal error with incorrect prefix",
			grpcErr:  status.Error(codes.Internal, "wrong prefix: database connection failed"),
			typeName: "execution result",
		},
		{
			name:     "out of range with incorrect prefix",
			grpcErr:  status.Error(codes.OutOfRange, "wrong prefix: block height 1000 not available"),
			typeName: "block",
		},
		{
			name:     "precondition failed with incorrect prefix",
			grpcErr:  status.Error(codes.FailedPrecondition, "wrong prefix: index not initialized"),
			typeName: "events",
		},
		{
			name:     "invalid argument with incorrect prefix",
			grpcErr:  status.Error(codes.InvalidArgument, "wrong prefix: malformed transaction"),
			typeName: "transaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up a mock signaler context that expects Throw to be called with the original error
			expectedErr := fmt.Errorf("failed to convert upstream error: %w", tt.grpcErr)
			mockSignalerCtx := irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr)
			ctx := irrecoverable.WithSignalerContext(context.Background(), mockSignalerCtx)

			// convertError should call Throw and return an irrecoverable exception
			result := convertError(ctx, tt.grpcErr, tt.typeName)

			// The returned error should be an irrecoverable exception (not nil)
			require.Error(t, result)
			assert.Equal(t, result.Error(), irrecoverable.NewException(expectedErr).Error())
		})
	}
}
