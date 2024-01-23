package errors

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	cadenceErr "github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestErrorHandling(t *testing.T) {
	require.False(t, IsFailure(nil))

	t.Run("test nonfatal error detection", func(t *testing.T) {
		e1 := NewOperationNotSupportedError("some operations")
		e2 := fmt.Errorf("some other errors: %w", e1)
		e3 := NewInvalidProposalSignatureError(flow.ProposalKey{}, e2)
		e4 := fmt.Errorf("wrapped: %w", e3)

		expectedErr := WrapCodedError(
			e1.Code(), // The root cause's error code
			e4,        // All the error message detail.
			"error caused by")

		txErr, vmErr := SplitErrorTypes(e4)
		require.Nil(t, vmErr)
		require.Equal(t, expectedErr, txErr)

		require.False(t, IsFailure(e4))
		require.False(t, IsFailure(txErr))
	})

	t.Run("test fatal error detection", func(t *testing.T) {
		e1 := NewOperationNotSupportedError("some operations")
		e2 := NewEncodingFailuref(e1, "bad encoding")
		e3 := NewLedgerFailure(e2)
		e4 := fmt.Errorf("some other errors: %w", e3)
		e5 := NewInvalidProposalSignatureError(flow.ProposalKey{}, e4)
		e6 := fmt.Errorf("wrapped: %w", e5)

		expectedErr := WrapCodedError(
			e3.Code(), // The shallowest failure's error code
			e6,        // All the error message detail.
			"failure caused by")

		txErr, vmErr := SplitErrorTypes(e6)
		require.Nil(t, txErr)
		require.Equal(t, expectedErr, vmErr)

		require.True(t, IsFailure(e6))
		require.True(t, IsFailure(vmErr))
	})

	t.Run("unknown error", func(t *testing.T) {
		e1 := fmt.Errorf("some unknown errors")
		txErr, vmErr := SplitErrorTypes(e1)
		require.Nil(t, txErr)
		require.NotNil(t, vmErr)

		require.True(t, IsFailure(e1))
	})
}

func TestHandleRuntimeError(t *testing.T) {
	baseErr := fmt.Errorf("base error")
	tests := []struct {
		name string
		err  error
		code ErrorCode
	}{
		{
			name: "nil error",
			err:  nil,
			code: 0,
		},
		{
			name: "unknown error",
			err:  baseErr,
			code: FailureCodeUnknownFailure,
		},
		{
			name: "runtime error",
			err:  runtime.Error{Err: baseErr},
			code: ErrCodeCadenceRunTimeError,
		},
		{
			name: "coded error in Unwrappable error",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: NewScriptExecutionCancelledError(baseErr),
				},
			},
			code: ErrCodeScriptExecutionCancelledError,
		},
		{
			name: "coded error in ParentError error",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: sema.CheckerError{
						Errors: []error{
							fmt.Errorf("first error"),
							NewScriptExecutionTimedOutError(),
						},
					},
				},
			},
			code: ErrCodeScriptExecutionTimedOutError,
		},
		{
			name: "first coded error returned",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: sema.CheckerError{
						Errors: []error{
							fmt.Errorf("first error"),
							NewScriptExecutionTimedOutError(),
							NewScriptExecutionCancelledError(baseErr),
						},
					},
				},
			},
			code: ErrCodeScriptExecutionTimedOutError,
		},
		{
			name: "failure returned",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: sema.CheckerError{
						Errors: []error{
							fmt.Errorf("first error"),
							NewLedgerFailure(baseErr),
						},
					},
				},
			},
			code: FailureCodeLedgerFailure,
		},
		{
			name: "error returned if before failure",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: sema.CheckerError{
						Errors: []error{
							fmt.Errorf("first error"),
							NewScriptExecutionTimedOutError(),
							NewLedgerFailure(baseErr),
						},
					},
				},
			},
			code: FailureCodeLedgerFailure,
		},
		{
			name: "embedded coded errors return deepest error",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: sema.CheckerError{
						Errors: []error{
							fmt.Errorf("first error"),
							NewScriptExecutionCancelledError(
								NewScriptExecutionTimedOutError(),
							),
						},
					},
				},
			},
			code: ErrCodeScriptExecutionTimedOutError,
		},
		{
			name: "failure with embedded error returns failure",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: sema.CheckerError{
						Errors: []error{
							fmt.Errorf("first error"),
							NewLedgerFailure(
								NewScriptExecutionTimedOutError(),
							),
						},
					},
				},
			},
			code: FailureCodeLedgerFailure,
		},
		{
			name: "coded error with embedded failure returns failure",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: sema.CheckerError{
						Errors: []error{
							fmt.Errorf("first error"),
							NewScriptExecutionCancelledError(
								NewLedgerFailure(baseErr),
							),
						},
					},
				},
			},
			code: FailureCodeLedgerFailure,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := HandleRuntimeError(tc.err)
			if tc.code == 0 {
				assert.NoError(t, actual)
				return
			}

			coded, ok := actual.(CodedError)
			require.True(t, ok, "error is not a CodedError")

			if tc.code == FailureCodeUnknownFailure {
				assert.Equalf(t, tc.code, coded.Code(), "error code mismatch: expected %d, got %d", tc.code, coded.Code())
				return
			}

			// split the error to ensure that the wrapped error is available
			actualCoded, failureCoded := SplitErrorTypes(coded)

			if tc.code.IsFailure() {
				assert.NoError(t, actualCoded)
				assert.Equalf(t, tc.code, failureCoded.Code(), "error code mismatch: expected %d, got %d", tc.code, failureCoded.Code())
			} else {
				assert.NoError(t, failureCoded)
				assert.Equalf(t, tc.code, actualCoded.Code(), "error code mismatch: expected %d, got %d", tc.code, actualCoded.Code())
			}
		})
	}
}
