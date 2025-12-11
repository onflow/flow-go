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
		expected error
	}{
		{
			name:     "NotFound with data not found prefix",
			expected: access.NewDataNotFoundError("collection", storage.ErrNotFound),
		},
		{
			name:     "Internal with internal error prefix",
			expected: access.NewInternalError(errors.New("database connection failed")),
		},
		{
			name:     "OutOfRange with out of range prefix",
			expected: access.NewOutOfRangeError(errors.New("block height 1000 not available")),
		},
		{
			name:     "FailedPrecondition with precondition failed prefix",
			expected: access.NewPreconditionFailedError(errors.New("index not initialized")),
		},
		{
			name:     "InvalidArgument with invalid argument prefix",
			expected: access.NewInvalidRequestError(errors.New("malformed transaction")),
		},
		{
			name:     "Canceled with request canceled prefix",
			expected: access.NewRequestCanceledError(errors.New("client disconnected")),
		},
		{
			name:     "Canceled without prefix (client side)",
			expected: access.NewRequestCanceledError(status.Error(codes.Canceled, "context canceled")),
		},
		{
			name:     "DeadlineExceeded with request timed out prefix",
			expected: access.NewRequestTimedOutError(errors.New("execution took too long")),
		},
		{
			name:     "DeadlineExceeded without prefix (client side)",
			expected: access.NewRequestTimedOutError(status.Error(codes.DeadlineExceeded, "context deadline exceeded")),
		},
		{
			name:     "Unavailable with service unavailable error prefix",
			expected: access.NewServiceUnavailable(errors.New("upstream service down")),
		},
		{
			name:     "Unavailable without prefix (client side)",
			expected: access.NewServiceUnavailable(status.Error(codes.Unavailable, "connection refused")),
		},
		{
			name:     "ResourceExhausted with service resource exhausted error prefix",
			expected: access.NewResourceExhausted(errors.New("execution computation limit reached")),
		},
		{
			name:     "ResourceExhausted without prefix (client side)",
			expected: access.NewResourceExhausted(status.Error(codes.ResourceExhausted, "execution computation limit reached")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := irrecoverable.NewMockSignalerContext(t, context.Background())

			grpcError := rpc.ErrorToStatus(tt.expected)

			actual := convertError(ctx, grpcError)
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
		name    string
		grpcErr error
	}{
		{
			name:    "generic error",
			grpcErr: fmt.Errorf("generic error"),
		},
		{
			name:    "unknown status code",
			grpcErr: status.Error(codes.PermissionDenied, "permission denied"),
		},
		{
			name:    "not found with incorrect prefix",
			grpcErr: status.Error(codes.NotFound, "wrong prefix: key not found"),
		},
		{
			name:    "internal error with incorrect prefix",
			grpcErr: status.Error(codes.Internal, "wrong prefix: database connection failed"),
		},
		{
			name:    "out of range with incorrect prefix",
			grpcErr: status.Error(codes.OutOfRange, "wrong prefix: block height 1000 not available"),
		},
		{
			name:    "precondition failed with incorrect prefix",
			grpcErr: status.Error(codes.FailedPrecondition, "wrong prefix: index not initialized"),
		},
		{
			name:    "invalid argument with incorrect prefix",
			grpcErr: status.Error(codes.InvalidArgument, "wrong prefix: malformed transaction"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up a mock signaler context that expects Throw to be called with the original error
			expectedErr := fmt.Errorf("failed to convert upstream error: %w", tt.grpcErr)
			mockSignalerCtx := irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr)
			ctx := irrecoverable.WithSignalerContext(context.Background(), mockSignalerCtx)

			// convertError should call Throw and return an irrecoverable exception
			result := convertError(ctx, tt.grpcErr)

			// The returned error should be an irrecoverable exception (not nil)
			require.Error(t, result)
			assert.Equal(t, result.Error(), irrecoverable.NewException(expectedErr).Error())
		})
	}
}

// TestSplitNotFoundError tests the splitNotFoundError function, which extracts
// the type name and the cleaned error message from an error string that starts
// with a defined access.DataNotFoundPrefix.
//
// Test cases:
// Happy cases (ok=true):
//
//  1. Single-word type:
//     Input:  "data not found for header: failed to load header"
//     Output: typeName="header", errorStr="failed to load header", ok=true
//
//  2. Multi-word type:
//     Input:  "data not found for execution result snapshot: missing root hash"
//     Output: typeName="execution result snapshot", errorStr="missing root hash", ok=true
//
//  3. Hyphenated type:
//     Input:  "data not found for block-header info: no header at height 10"
//     Output: typeName="block-header info", errorStr="no header at height 10", ok=true
//
//  4. Nested colons in error message:
//     Input:  "data not found for account data: failed: could not parse: invalid"
//     Output: typeName="account data", errorStr="failed: could not parse: invalid", ok=true
//
//  5. Whitespace trimming:
//     Input:  "data not found for   header   :   some error  "
//     Output: typeName="header value", errorStr="some error", ok=true
//
// Error cases (ok=false):
//
//  1. Missing prefix:
//     Input:  "some prefix"
//     Output: ok=false
//
//  2. Prefix but no colon:
//     Input:  "data not found for header missing colon"
//     Output: ok=false
//
//  3. Only prefix:
//     Input:  "data not found for "
//     Output: ok=false
func TestSplitNotFoundError(t *testing.T) {
	// Happy cases (ok=true)
	t.Run("single word type", func(t *testing.T) {
		input := "data not found for header: failed to load header"
		typ, errStr, ok := splitNotFoundError(input)

		require.True(t, ok)
		require.Equal(t, "header", typ)
		require.Equal(t, "failed to load header", errStr)
	})

	t.Run("multi-word type", func(t *testing.T) {
		input := "data not found for execution result snapshot: missing root hash"
		typ, errStr, ok := splitNotFoundError(input)

		require.True(t, ok)
		require.Equal(t, "execution result snapshot", typ)
		require.Equal(t, "missing root hash", errStr)
	})

	t.Run("hyphenated type", func(t *testing.T) {
		input := "data not found for block-header info: no header at height 10"
		typ, errStr, ok := splitNotFoundError(input)

		require.True(t, ok)
		require.Equal(t, "block-header info", typ)
		require.Equal(t, "no header at height 10", errStr)
	})

	t.Run("nested colons in error message", func(t *testing.T) {
		input := "data not found for account data: failed: could not parse: invalid"
		typ, errStr, ok := splitNotFoundError(input)

		require.True(t, ok)
		require.Equal(t, "account data", typ)
		require.Equal(t, "failed: could not parse: invalid", errStr)
	})

	t.Run("whitespace trimming", func(t *testing.T) {
		input := "data not found for   header   :   some error  "
		typ, errStr, ok := splitNotFoundError(input)

		require.True(t, ok)
		require.Equal(t, "header", typ)
		require.Equal(t, "some error", errStr)
	})

	// Error cases (ok=false)
	t.Run("missing prefix", func(t *testing.T) {
		input := "some prefix"
		_, _, ok := splitNotFoundError(input)

		require.False(t, ok)
	})

	t.Run("prefix but no colon", func(t *testing.T) {
		input := "data not found for header missing colon"
		_, _, ok := splitNotFoundError(input)

		require.False(t, ok)
	})

	t.Run("only prefix", func(t *testing.T) {
		input := "data not found for "
		_, _, ok := splitNotFoundError(input)

		require.False(t, ok)
	})
}
