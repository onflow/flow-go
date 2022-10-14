package errors

import (
	"fmt"
	"testing"

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
