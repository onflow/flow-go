package errors

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
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

		expectedErr := WrapCodedFailure(
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
		name        string
		err         error
		errorCode   ErrorCode
		failureCode FailureCode
	}{
		{
			name: "nil error",
			err:  nil,
		},
		{
			name:        "unknown error",
			err:         baseErr,
			failureCode: FailureCodeUnknownFailure,
		},
		{
			name:      "runtime error",
			err:       runtime.Error{Err: baseErr},
			errorCode: ErrCodeCadenceRunTimeError,
		},
		{
			name: "coded error in Unwrappable error",
			err: runtime.Error{
				Err: cadenceErr.ExternalError{
					Recovered: NewScriptExecutionCancelledError(baseErr),
				},
			},
			errorCode: ErrCodeScriptExecutionCancelledError,
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
			errorCode: ErrCodeScriptExecutionTimedOutError,
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
			errorCode: ErrCodeScriptExecutionTimedOutError,
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
			failureCode: FailureCodeLedgerFailure,
		},
		{
			name: "error before failure returns failure",
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
			failureCode: FailureCodeLedgerFailure,
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
			errorCode: ErrCodeScriptExecutionTimedOutError,
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
			failureCode: FailureCodeLedgerFailure,
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
			failureCode: FailureCodeLedgerFailure,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := HandleRuntimeError(tc.err)
			if tc.err == nil {
				assert.NoError(t, actual)
				return
			}

			var actualCoded CodedError
			var failureCoded CodedFailure
			switch coded := actual.(type) {
			case CodedError:
				actualCoded, failureCoded = SplitErrorTypes(coded)

			case CodedFailure:
				actualCoded, failureCoded = SplitErrorTypes(coded)

			default:
				t.Fatalf("unexpected error type: %T", actual)
			}

			if tc.failureCode != 0 {
				assert.NoError(t, actualCoded)
				assert.Equalf(t, tc.failureCode, failureCoded.Code(), "error code mismatch: expected %d, got %d", tc.failureCode, failureCoded.Code())
			} else {
				assert.NoError(t, failureCoded)
				assert.Equalf(t, tc.errorCode, actualCoded.Code(), "error code mismatch: expected %d, got %d", tc.errorCode, actualCoded.Code())
			}
		})
	}
}

func TestFind(t *testing.T) {
	targetCode := ErrCodeScriptExecutionCancelledError
	baseErr := fmt.Errorf("base error")

	tests := []struct {
		name  string
		err   error
		found bool
	}{
		{
			name:  "nil error",
			err:   nil,
			found: false,
		},
		{
			name:  "plain error",
			err:   baseErr,
			found: false,
		},
		{
			name:  "wrapped plain error",
			err:   fmt.Errorf("wrapped: %w", baseErr),
			found: false,
		},
		{
			name:  "coded failure",
			err:   NewLedgerFailure(baseErr),
			found: false,
		},
		{
			name:  "incorrect coded error",
			err:   NewScriptExecutionTimedOutError(),
			found: false,
		},
		{
			name:  "found",
			err:   NewScriptExecutionCancelledError(baseErr),
			found: true,
		},
		{
			name:  "found with embedded errors",
			err:   NewScriptExecutionCancelledError(NewLedgerFailure(NewScriptExecutionTimedOutError())),
			found: true,
		},
		{
			name:  "found embedded in error",
			err:   NewDerivedDataCacheImplementationFailure(NewScriptExecutionCancelledError(baseErr)),
			found: true,
		},
		{
			name:  "found embedded in failure",
			err:   NewLedgerFailure(NewScriptExecutionCancelledError(baseErr)),
			found: true,
		},
		{
			name: "found embedded with multierror",
			err: &multierror.Error{
				Errors: []error{
					baseErr,
					NewScriptExecutionTimedOutError(),
					NewLedgerFailure(NewScriptExecutionCancelledError(baseErr)),
				},
			},
			found: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := Find(tc.err, targetCode)
			if !tc.found {
				assert.NoError(t, actual)
				return
			}

			assert.Equalf(t, targetCode, actual.Code(), "error code mismatch: expected %d, got %d", targetCode, actual.Code())
		})
	}
}

func TestFindFailure(t *testing.T) {
	targetCode := FailureCodeLedgerFailure
	baseErr := fmt.Errorf("base error")
	tests := []struct {
		name  string
		err   error
		found bool
	}{
		{
			name:  "nil error",
			err:   nil,
			found: false,
		},
		{
			name:  "plain error",
			err:   baseErr,
			found: false,
		},
		{
			name:  "wrapped plain error",
			err:   fmt.Errorf("wrapped: %w", baseErr),
			found: false,
		},
		{
			name:  "coded error",
			err:   NewScriptExecutionTimedOutError(),
			found: false,
		},
		{
			name:  "incorrect coded failure",
			err:   NewStateMergeFailure(baseErr),
			found: false,
		},
		{
			name:  "found",
			err:   NewLedgerFailure(baseErr),
			found: true,
		},
		{
			name:  "found with embedded errors",
			err:   NewLedgerFailure(NewScriptExecutionCancelledError(NewScriptExecutionTimedOutError())),
			found: true,
		},
		{
			name:  "found embedded in error",
			err:   NewDerivedDataCacheImplementationFailure(NewLedgerFailure(baseErr)),
			found: true,
		},
		{
			name:  "found embedded in failure",
			err:   NewStateMergeFailure(NewLedgerFailure(baseErr)),
			found: true,
		},
		{
			name: "found embedded with multierror",
			err: &multierror.Error{
				Errors: []error{
					baseErr,
					NewScriptExecutionTimedOutError(),
					NewScriptExecutionCancelledError(NewLedgerFailure(baseErr)),
				},
			},
			found: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := FindFailure(tc.err, targetCode)
			if !tc.found {
				assert.NoError(t, actual)
				return
			}

			assert.Equalf(t, targetCode, actual.Code(), "error code mismatch: expected %d, got %d", targetCode, actual.Code())
		})
	}
}
