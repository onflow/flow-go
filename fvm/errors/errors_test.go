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
		e3 := NewInvalidProposalSignatureError(flow.EmptyAddress, 0, e2)
		e4 := fmt.Errorf("wrapped: %w", e3)

		txErr, vmErr := SplitErrorTypes(e4)
		require.Nil(t, vmErr)
		require.Equal(t, e3, txErr)

		require.False(t, IsFailure(e4))
	})

	t.Run("test fatal error detection", func(t *testing.T) {
		e1 := NewOperationNotSupportedError("some operations")
		e2 := NewEncodingFailuref(e1, "bad encoding")
		e3 := NewLedgerFailure(e2)
		e4 := fmt.Errorf("some other errors: %w", e3)
		e5 := NewInvalidProposalSignatureError(flow.EmptyAddress, 0, e4)
		e6 := fmt.Errorf("wrapped: %w", e5)

		txErr, vmErr := SplitErrorTypes(e6)
		require.Nil(t, txErr)
		require.Equal(t, e3, vmErr)

		require.True(t, IsFailure(e6))
	})

	t.Run("unknown error", func(t *testing.T) {
		e1 := fmt.Errorf("some unknown errors")
		txErr, vmErr := SplitErrorTypes(e1)
		require.Nil(t, txErr)
		require.NotNil(t, vmErr)

		require.True(t, IsFailure(e1))
	})
}
